package http

import (
	"fmt"
	"net/http"

	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

func NewSmokeHandler(plat orgbot.Platform) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Do something more meaningful
		_, _ = fmt.Fprintf(w, "TODO")
	})
}
