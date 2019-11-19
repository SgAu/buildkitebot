package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SEEK-Jobs/orgbot/pkg/cli"

	"github.com/rs/zerolog"
)

// log is the configured zerolog Logger instance that gets injected into the Context.
var log zerolog.Logger

func init() {
	// Default to info level logging unless --debug is provided
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	logWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	logWriter.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
	}
	logWriter.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}
	logWriter.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}
	logWriter.FormatFieldValue = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}

	log = zerolog.New(logWriter).With().Timestamp().Logger()
}

// This main runs Orgbot as a command line tool.
func main() {
	ctx := log.WithContext(context.Background())
	if err := cli.NewRootCommand(ctx).Execute(); err != nil {
		log.Fatal().Err(err).Msg("Error occurred")
	}
}
