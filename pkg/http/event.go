package http

import (
	"context"
	"encoding/json"

	hub "github.com/google/go-github/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"

	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

type eventHandler struct {
	plat orgbot.Platform
}

func newEventHandler(plat orgbot.Platform) githubapp.EventHandler {
	return &eventHandler{plat: plat}
}

// Handles implements githubapp.EventHandler
func (h *eventHandler) Handles() []string {
	return []string{teamEvent}
}

// Handle implements githubapp.EventHandler
func (h *eventHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event hub.TeamEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "failed to parse event payload")
	}

	id := githubapp.GetInstallationIDFromEvent(&event)
	c := message{
		InstallationID: id,
		EventType:      eventType,
		DeliveryID:     deliveryID,
		Payload:        string(payload),
	}

	return sendMessage(h.plat, c)
}
