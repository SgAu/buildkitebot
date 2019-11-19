package main

import (
	"context"
	"fmt"
	h "net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"

	"github.com/justinas/alice"

	"github.com/SEEK-Jobs/orgbot/pkg/cmd"
	"github.com/SEEK-Jobs/orgbot/pkg/http"
)

// This main runs Orgbot as a GitHub App.
func main() {
	log := zerolog.New(os.Stderr).With().Timestamp().Logger()

	httpPort, err := cmd.LookupHttpPort()
	if err != nil {
		log.Fatal().Err(err).Msg("Could not determine HTTP port")
	}

	plat, metricsFn, err := cmd.NewPlatform()
	if err != nil {
		log.Fatal().Err(err).Msg("Could not create Platform")
	}

	m := buildMiddleware(log)

	h.Handle("/health", m.Then(http.NewHealthHandler()))
	h.Handle("/smoke", m.Then(http.NewSmokeHandler(plat)))
	h.Handle("/hook", m.Then(http.NewHookHandler(plat)))

	log.Info().Msgf("Reporting metrics every %v", plat.Config().MetricsInterval)
	go metricsFn()

	ctx := log.WithContext(context.Background())

	errChan := make(chan error)
	defer close(errChan)

	go http.ListenForEvents(ctx, plat, errChan)

	go func() {
		for {
			e := <-errChan
			if e != nil {
				log.Fatal().Err(e).Msg("Received error while processing events")
			}
		}
	}()

	log.Info().Msgf("Listening on port %d", httpPort)
	if err = h.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil); err != nil {
		log.Fatal().Err(err).Msg("Server crashed")
	}
}

func buildMiddleware(log zerolog.Logger) alice.Chain {
	m := alice.New()

	// Install the log handler
	m = m.Append(hlog.NewHandler(log))

	// Install handlers that populate log events with useful fields
	m = m.Append(hlog.AccessHandler(func(r *h.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Msg("")
	}))
	m = m.Append(hlog.RemoteAddrHandler("ip"))
	m = m.Append(hlog.UserAgentHandler("user_agent"))
	m = m.Append(hlog.RefererHandler("referer"))
	m = m.Append(hlog.RequestIDHandler("req_id", "Request-Id"))

	return m
}
