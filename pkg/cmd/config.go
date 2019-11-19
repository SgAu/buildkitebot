package cmd

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/pkg/errors"

	"github.com/SEEK-Jobs/orgbot/pkg/aws"
	"github.com/SEEK-Jobs/orgbot/pkg/build"
	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

// loadConfig builds and returns the orgbot.Config.
func loadConfig(sess *session.Session) (*orgbot.Config, error) {
	gitHubAuditBucket, err := LookupGitHubAuditBucket()
	if err != nil {
		return nil, err
	}

	metricsInterval, err := LookupMetricsInterval()
	if err != nil {
		return nil, err
	}

	appConfig, err := loadGitHubAppConfig(sess)
	if err != nil {
		return nil, err
	}

	queueURL, err := LookupQueueURL()
	if err != nil {
		return nil, err
	}

	return &orgbot.Config{
		Name:              build.Name,
		Version:           build.Version,
		MetricsInterval:   metricsInterval,
		GitHubAuditBucket: gitHubAuditBucket,
		GitHubAppConfig:   *appConfig,
		QueueURL:          queueURL,
	}, nil
}

// loadGitHubAppConfig returns the GitHub App config data from AWS SecretsManager.
func loadGitHubAppConfig(sess *session.Session) (*orgbot.GitHubAppConfig, error) {
	secretID, err := LookupConfigSecretID()
	if err != nil {
		return nil, err
	}

	secretsManager := aws.NewSecretsManager(sess)
	v, err := secretsManager.SecretValue(secretID)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve secret with ID '%s'", secretID)
	}

	var c orgbot.GitHubAppConfig
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal JSON secret")
	}

	return &c, nil
}
