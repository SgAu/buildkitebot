package orgbot

import (
	"context"
	"regexp"

	"github.com/golang/mock/gomock"

	"github.com/SEEK-Jobs/orgbot/pkg/yaml"
)

// TestPlatform provides a test implementation of Platform that contains mocks.
type TestPlatform struct {
	config *Config
	codec  Codec

	MockGitHubService *MockGitHubService
	MockRuleEngine    *MockRuleEngine
}

// NewTestPlatform returns a new TestPlatform instance.
func NewTestPlatform(ctrl *gomock.Controller) *TestPlatform {
	config := &Config{
		Name:              "orgbot",
		Version:           "1.0.0",
		GitHubAuditBucket: "bucket",
	}

	return &TestPlatform{
		config:            config,
		codec:             yaml.NewCodec(),
		MockGitHubService: NewMockGitHubService(ctrl),
		MockRuleEngine:    NewMockRuleEngine(ctrl),
	}
}

// NewTestPlatformPassingRules returns a new TestPlatform instance with a MockRuleEngine
// that has been configured to pass.
func NewTestPlatformPassingRules(ctrl *gomock.Controller) *TestPlatform {
	plat := NewTestPlatform(ctrl)

	// Rules pass
	plat.MockRuleEngine.
		EXPECT().
		Run(gomock.Any(), gomock.Any()).
		Return(nil)

	return plat
}

// Config implements Platform.
func (plat *TestPlatform) Config() *Config {
	return plat.config
}

// GitHubService implements Platform.
func (plat *TestPlatform) GitHubService() GitHubService {
	return plat.MockGitHubService
}

// RuleEngine implements Platform.
func (plat *TestPlatform) RuleEngine() RuleEngine {
	return plat.MockRuleEngine
}

// Codec implements Platform.
func (plat *TestPlatform) Codec() Codec {
	return plat.codec
}

// ExpectUserByEmailAnyTimes configures an expectation on the MockGitHubService for UserByEmail
// to be called any times and for the function to return a phony GitHub user for each call.
func (m *MockGitHubService) ExpectUserByEmailAnyTimes(ctx interface{}, orgName interface{}) {
	m.
		EXPECT().
		UserByEmail(ctx, orgName, gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName, email string) (*GitHubUser, error) {
			login := emailToLogin(email)
			return &GitHubUser{Login: login, Email: email}, nil
		}).
		AnyTimes()
}

// emailToLogin returns a phony GitHub login for the specified email address.
func emailToLogin(email string) string {
	emailSuffixRegex := regexp.MustCompile(`@.*`)
	return emailSuffixRegex.ReplaceAllString(email, "")
}
