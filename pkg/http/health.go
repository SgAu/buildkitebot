package http

import (
	"fmt"
	"net/http"
)

func NewHealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "OK")
	})
}
