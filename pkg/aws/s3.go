package aws

import (
	"bytes"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

type S3 struct {
	client s3iface.S3API
}

func NewS3(sess *session.Session) *S3 {
	return &S3{
		client: s3.New(sess),
	}
}

func (s *S3) GetObjectAsString(bucket, key string) (string, error) {
	req := s3.GetObjectInput{Bucket: &bucket, Key: &key}
	res, err := s.client.GetObject(&req)
	if err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	if _, err := buf.ReadFrom(res.Body); err != nil {
		return "", err
	}

	return buf.String(), nil
}
