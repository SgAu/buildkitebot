package aws

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
)

type SecretsManager struct {
	client secretsmanageriface.SecretsManagerAPI
}

func NewSecretsManager(sess *session.Session) *SecretsManager {
	return &SecretsManager{
		client: secretsmanager.New(sess),
	}
}

func (s *SecretsManager) SecretValue(id string) (string, error) {
	req := secretsmanager.GetSecretValueInput{SecretId: &id}
	res, err := s.client.GetSecretValue(&req)
	if err != nil {
		return "", err
	}

	return *res.SecretString, nil
}
