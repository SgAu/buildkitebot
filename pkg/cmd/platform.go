package cmd

import (
	"fmt"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/palantir/go-githubapp/githubapp"

	"github.com/SEEK-Jobs/orgbot/pkg/github"
	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
	"github.com/SEEK-Jobs/orgbot/pkg/yaml"
)

const (
	// gitHubV3URL is the base URL of the GitHub REST API
	gitHubV3URL = "https://api.github.com"
	// gitHubV4URL is the base URL of the GitHub GraphQL API
	gitHubV4URL = "https://api.github.com/graphql"
)

// platform provides the implementation of orgbot.Platform.
type platform struct {
	config        *orgbot.Config
	gitHubService orgbot.GitHubService
	ruleEngine    orgbot.RuleEngine
	codec         orgbot.Codec
}

// NewPlatform returns a new read-write orgbot.Platform.
func NewPlatform() (orgbot.Platform, MetricsReporterFunc, error) {
	sess, err := NewAWSSession()
	if err != nil {
		return nil, nil, err
	}

	config, err := loadConfig(sess)
	if err != nil {
		return nil, nil, err
	}

	clientFactory, err := newGitHubClientFactory(config)
	if err != nil {
		return nil, nil, err
	}

	gitHubService, err := github.NewService(config, clientFactory, sess)
	if err != nil {
		return nil, nil, err
	}

	ruleEngine := orgbot.NewRuleEngine(orgbot.NewReadOnlyGitHubService(gitHubService))

	return &platform{
		config:        config,
		gitHubService: gitHubService,
		ruleEngine:    ruleEngine,
		codec:         yaml.NewCodec(),
	}, metricsReporter(config, sess), nil
}

// NewReadOnlyPlatform returns a new read-only orgbot.Platform.
func NewReadOnlyPlatform(delegate orgbot.Platform) orgbot.Platform {
	readOnlyGitHubService := orgbot.NewReadOnlyGitHubService(delegate.GitHubService())

	return &platform{
		config:        delegate.Config(),
		gitHubService: readOnlyGitHubService,
		ruleEngine:    delegate.RuleEngine(),
		codec:         delegate.Codec(),
	}
}

// Config implements orgbot.Platform.Config.
func (plat *platform) Config() *orgbot.Config {
	return plat.config
}

// GitHubService implements orgbot.Platform.GitHubService.
func (plat *platform) GitHubService() orgbot.GitHubService {
	return plat.gitHubService
}

// RuleEngine implements orgbot.Platform.GitHubService.
func (plat *platform) RuleEngine() orgbot.RuleEngine {
	return plat.ruleEngine
}

// Codec implements orgbot.Platform.GitHubService.
func (plat *platform) Codec() orgbot.Codec {
	return plat.codec
}

// newGitHubClientFactory returns a configured github.ClientFactory.
func newGitHubClientFactory(c *orgbot.Config) (*github.ClientFactory, error) {
	delegate := githubapp.NewClientCreator(gitHubV3URL, gitHubV4URL, c.GitHubAppID, []byte(c.GitHubPrivateKey),
		githubapp.WithClientUserAgent(fmt.Sprintf("%s/%s", c.Name, c.Version)),
		githubapp.WithClientMiddleware(
			githubapp.ClientMetrics(metricsRegistry),
		))

	clientCreator, err := githubapp.NewCachingClientCreator(delegate, githubapp.DefaultCachingClientCapacity)
	if err != nil {
		return nil, err
	}

	return github.NewSingleInstallationClientFactory(clientCreator, c.GitHubInstallationID), nil
}

// NewAWSSession creates a new AWS session.Session.
func NewAWSSession() (*session.Session, error) {
	region, err := LookupRegion()
	if err != nil {
		return nil, err
	}

	return session.NewSession(&awssdk.Config{Region: &region})
}
