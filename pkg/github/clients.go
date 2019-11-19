package github

import (
	"context"
	"errors"

	"github.com/google/go-github/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/shurcooL/githubv4"
)

var (
	// missingInstallationID is the error returned when no installation ID was set on the context.
	missingInstallationID = errors.New("no GitHub App installation ID set on context")
)

// Key used for setting the GitHub App installation ID in the context.
type ctxKey struct{}

// InstallationID returns the GitHub App installation ID associated with the context
// if one exists. If one does not exist false is returned as the second return parameter.
func InstallationID(ctx context.Context) (int64, bool) {
	if id, ok := ctx.Value(ctxKey{}).(int64); ok {
		return id, true
	}
	return 0, false
}

// WithInstallationID returns a new context with the specified GitHub App installation ID set.
func WithInstallationID(ctx context.Context, id int64) context.Context {
	if id2, ok := ctx.Value(ctxKey{}).(int64); ok {
		if id2 == id {
			return ctx // Don't store same ID
		}
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// ClientFactory provides a means of creating GitHub clients based on different auth strategies.
type ClientFactory struct {
	token          string
	installationID int64
	githubapp.ClientCreator
}

// NewTokenClientFactory returns a ClientFactory that creates user token-based GitHub clients.
func NewTokenClientFactory(clientCreator githubapp.ClientCreator, token string) *ClientFactory {
	return &ClientFactory{token: token, ClientCreator: clientCreator}
}

// NewSingleInstallationClientFactory returns a ClientFactory that creates installation-based
// GitHub clients where the installation ID is known ahead of time and does not vary.
func NewSingleInstallationClientFactory(clientCreator githubapp.ClientCreator, installationID int64) *ClientFactory {
	return &ClientFactory{ClientCreator: clientCreator, installationID: installationID}
}

// NewMultiInstallationClientFactory returns a ClientFactory that creates installation-based
// GitHub clients where the installation ID is expected to be set on the context for each request.
func NewMultiInstallationClientFactory(clientCreator githubapp.ClientCreator) *ClientFactory {
	return &ClientFactory{ClientCreator: clientCreator}
}

// V3Client returns a V3 GitHub client.
func (f *ClientFactory) V3Client(ctx context.Context) (*github.Client, error) {
	if f.token != "" {
		return f.NewTokenClient(f.token)
	}

	if f.installationID != 0 {
		return f.NewInstallationClient(f.installationID)
	}

	if id, ok := InstallationID(ctx); ok {
		return f.NewInstallationClient(id)
	}
	return nil, missingInstallationID
}

// V4Client returns a V4 GitHub client.
func (f *ClientFactory) V4Client(ctx context.Context) (*githubv4.Client, error) {
	if f.token != "" {
		return f.NewTokenV4Client(f.token)
	}

	if f.installationID != 0 {
		return f.NewInstallationV4Client(f.installationID)
	}

	if id, ok := InstallationID(ctx); ok {
		return f.NewInstallationV4Client(id)
	}

	return nil, missingInstallationID
}
