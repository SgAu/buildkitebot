package http

import (
	"net/http"

	"github.com/palantir/go-githubapp/githubapp"

	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

const (
	pushEvent        = "push"
	pullRequestEvent = "pull_request"
	teamEvent        = "team"
)

// NewHookHandler returns the handler used for the event hooks
func NewHookHandler(plat orgbot.Platform) http.Handler {
	return githubapp.NewEventDispatcher(
		[]githubapp.EventHandler{newEventHandler(plat)},
		plat.Config().GitHubWebhookSecret)
}
