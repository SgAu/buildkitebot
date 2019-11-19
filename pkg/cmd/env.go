package cmd

import (
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

const (
	regionEnvKey          = "REGION"
	httpPortEnvKey        = "PORT"
	configSecretEnvKey    = "CONFIG_SECRET_ID"
	gitHubAuditEnvKey     = "GITHUB_AUDIT_BUCKET"
	queueURLEnvKey        = "QUEUE_URL"
	metricsIntervalEnvKey = "METRICS_INTERVAL"

	// Defaults config values
	defaultRegion            = "ap-southeast-2"
	defaultHttpPort          = "8000"
	defaultConfigSecretId    = "orgbot/config"
	defaultGitHubAuditBucket = "sec-github-audit"
	defaultQueueURL          = "https://sqs.ap-southeast-2.amazonaws.com/547523876443/orgbot.fifo"
	defaultMetricsInterval   = "30s"
)

func LookupRegion() (string, error) {
	return configValue(regionEnvKey, defaultRegion), nil
}

func LookupHttpPort() (int, error) {
	port := configValue(httpPortEnvKey, defaultHttpPort)
	httpPort, err := strconv.Atoi(port)
	if err != nil {
		return 0, errors.Errorf("bad port number: %s", port)
	}

	return httpPort, nil
}

func LookupConfigSecretID() (string, error) {
	return configValue(configSecretEnvKey, defaultConfigSecretId), nil
}

func LookupGitHubAuditBucket() (string, error) {
	return configValue(gitHubAuditEnvKey, defaultGitHubAuditBucket), nil
}

func LookupMetricsInterval() (time.Duration, error) {
	v := configValue(metricsIntervalEnvKey, defaultMetricsInterval)
	return time.ParseDuration(v)
}

func LookupQueueURL() (string, error) {
	return configValue(queueURLEnvKey, defaultQueueURL), nil
}

func configValue(envKey, defaultValue string) string {
	if v, ok := os.LookupEnv(envKey); ok {
		return v
	}
	return defaultValue
}
