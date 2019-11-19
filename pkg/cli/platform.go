package cli

import (
	"github.com/rs/zerolog"

	"github.com/SEEK-Jobs/orgbot/pkg/cmd"
	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"

	"context"
)

var (
	// lazyPlatform provides a means of overriding the concrete implementation of
	// Platform used in tests. It's lazy because creation of a real Platform has side-effects.
	lazyPlatform = func() (orgbot.Platform, error) {
		plat, _, err := cmd.NewPlatform()
		return plat, err
	}
)

// newPlatform returns an instance of orgbot.Platform that honours the specified flag.
func newPlatform(ctx context.Context, readOnly bool) (orgbot.Platform, error) {
	if readOnly {
		return newReadOnlyPlatform(ctx)
	}
	return newReadWritePlatform(ctx)
}

// newReadWritePlatform returns a read-write instance of orgbot.Platform.
func newReadWritePlatform(ctx context.Context) (orgbot.Platform, error) {
	zerolog.Ctx(ctx).Info().Msg("Running in READ-WRITE mode")
	return lazyPlatform()
}

// newReadOnlyPlatform returns a read-only instance of orgbot.Platform.
func newReadOnlyPlatform(ctx context.Context) (orgbot.Platform, error) {
	zerolog.Ctx(ctx).Info().Msg("Running in READ-ONLY mode")
	plat, err := lazyPlatform()
	if err != nil {
		return nil, err
	}

	return cmd.NewReadOnlyPlatform(plat), nil
}
