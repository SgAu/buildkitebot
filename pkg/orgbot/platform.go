package orgbot

import (
	"time"
)

// Platform provides the domain interface for interacting with the application configuration
// as well as all backend services.
type Platform interface {
	Config() *Config
	GitHubService() GitHubService
	RuleEngine() RuleEngine
	Codec() Codec
}

// Codec provides a means of decoding/encoding domain types from/to strings.
type Codec interface {
	Decode([]byte, interface{}) error
	Encode(interface{}) ([]byte, error)
}

// Config provides the application configuration.
type Config struct {
	Name              string        // Name of this application
	Version           string        // Version of this application
	MetricsInterval   time.Duration // Interval at which metrics are reported to CloudWatch
	GitHubAuditBucket string        // Name of the bucket where GitHub audit data is stored
	QueueURL          string        // URL of the SQS queue used for asynchronous processing

	GitHubAppConfig
}

// GitHubAppConfig provides GitHub App specific configuration.
type GitHubAppConfig struct {
	GitHubAppID          int    `json:"gitHubAppID"`
	GitHubInstallationID int64  `json:"gitHubInstallationID"`
	GitHubPrivateKey     string `json:"gitHubPrivateKey"`
	GitHubWebhookSecret  string `json:"gitHubWebhookSecret"`
}
