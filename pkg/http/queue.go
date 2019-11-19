package http

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	hub "github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/SEEK-Jobs/orgbot/pkg/cmd"
	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

type queueService struct {
	config    *orgbot.Config
	sqsClient sqsiface.SQSAPI
}

type message struct {
	InstallationID int64
	EventType      string
	DeliveryID     string
	// String is better- base64 encoded if byte[]
	Payload string
}

type receiptHandle string

// newQueueService returns a configured queueService implementation.
func newQueueService(c *orgbot.Config) (*queueService, error) {
	sess, err := cmd.NewAWSSession()
	if err != nil {
		return nil, err
	}

	return &queueService{
		config:    c,
		sqsClient: sqs.New(sess),
	}, nil

}

// submit puts a given message on the queue
func (s *queueService) submit(m *message) error {
	buf, err := json.Marshal(m)
	if err != nil {
		return err
	}

	input := sqs.SendMessageInput{
		MessageBody:            aws.String(string(buf)),
		MessageDeduplicationId: aws.String(m.DeliveryID),
		MessageGroupId:         aws.String(s.config.Name),
		QueueUrl:               aws.String(s.config.QueueURL),
	}

	_, err = s.sqsClient.SendMessage(&input)
	return err
}

// receive fetches messages from the queue
func (s *queueService) receive() (*message, receiptHandle, error) {
	receiveInput := &sqs.ReceiveMessageInput{
		MaxNumberOfMessages: aws.Int64(1),
		QueueUrl:            aws.String(s.config.QueueURL),
	}

	messageOutput, err := s.sqsClient.ReceiveMessage(receiveInput)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed to receive message from sqs")
	}

	if len(messageOutput.Messages) == 0 {
		return nil, "", nil
	}

	if len(messageOutput.Messages) > 1 {
		return nil, "", fmt.Errorf("received too many messages from sqs (expected 1, received %d)", len(messageOutput.Messages))
	}

	var m message
	receivedMessage := messageOutput.Messages[0]
	if err := json.Unmarshal([]byte(*receivedMessage.Body), &m); err != nil {
		return nil, "", errors.Wrap(err, "failed to unmarshal sqs message")
	}

	return &m, receiptHandle(*receivedMessage.ReceiptHandle), nil
}

// delete removes a message from the queue, signalling that processing it is successfully completed
func (s *queueService) delete(handle receiptHandle) error {
	if handle == "" {
		return nil
	}
	deleteInput := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(s.config.QueueURL),
		ReceiptHandle: aws.String(string(handle)),
	}

	_, err := s.sqsClient.DeleteMessage(deleteInput)
	if err != nil {
		return errors.Wrap(err, "failed to delete message from sqs")
	}

	return nil
}

// ListenForEvents polls an SQS queue for Github event payloads
func ListenForEvents(ctx context.Context, p orgbot.Platform, errChan chan error) {
	log.Info().Msgf("Starting queue service")

	queueService, err := newQueueService(p.Config())
	if err != nil {
		zerolog.Ctx(ctx).Error().Err(err)
		errChan <- err
		return
	}

	for {
		m, receiptHandle, err := queueService.receive()
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err)
			errChan <- err
			return
		}

		if m == nil {
			continue
		}

		var event hub.TeamEvent
		if err := json.Unmarshal([]byte(m.Payload), &event); err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msgf("Failed to unmarshal message")
			continue
		}

		if event.Repo != nil && *event.Repo.Fork {
			log.Ctx(ctx).Warn().Msgf("Skipping repo %s", *event.Repo.FullName)
			err := queueService.delete(receiptHandle)
			if err != nil {
				zerolog.Ctx(ctx).Error().Err(err).Msgf("failed to delete message")
			}
			continue
		}

		if event.Action != nil {
			switch *event.Action {
			case "added_to_repository", "removed_from_repository":
				log.Ctx(ctx).Info().Msgf("Received team update event for team %s on repo %s", *event.Repo.Name, *event.Team.Name)
				_, err = orgbot.UpdateRepoAdminTopics(ctx, p, *event.Repo.Owner.Login, *event.Repo.Name)

			case "edited":
				// Permissions changing is covered here
				log.Ctx(ctx).Info().Msgf("Received team update event for team %s", *event.Team.Name)
				if event.Repo != nil {
					// If the repo isn't nil, the permissions for that team on that repo have changed
					_, err = orgbot.UpdateRepoAdminTopics(ctx, p, *event.Repo.Owner.Login, *event.Repo.Name)
				} else {
					_, err = orgbot.UpdateTeamAdminTopics(ctx, p, *event.Org.Login, orgbot.GitHubTeamID(*event.Team.ID))
				}
			case "created", "deleted":
			default:
				err = fmt.Errorf("don't recognise command %s", *event.Action)
			}
		}

		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err)
		}
		err = queueService.delete(receiptHandle)
		if err != nil {
			zerolog.Ctx(ctx).Error().Err(err).Msgf("failed to delete message")
		}
	}
}

// sendMessage pushes a message to the queue
func sendMessage(p orgbot.Platform, m message) error {
	queueService, err := newQueueService(p.Config())
	if err != nil {
		return err
	}

	if err := queueService.submit(&m); err != nil {
		return errors.Wrap(err, "failed to submit command")
	}

	return nil
}
