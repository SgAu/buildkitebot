package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/SEEK-Jobs/orgbot/pkg/build"
	"github.com/SEEK-Jobs/orgbot/pkg/cmd"
)

var (
	debug   = false
	printer cmd.ResultPrinter
)

// NewRootCommand returns the root cobra.Command for orgctl.
func NewRootCommand(ctx context.Context) *cobra.Command {
	var format string
	rootCmd := &cobra.Command{
		Use:     "orgctl",
		Version: build.Version,
		Short:   "Command line tool for updating and querying org structure",
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			// Don't print usage if the command or child command produces an error as it's
			// confusing. We should only print usage if the CLI syntax is bad.
			c.SilenceUsage = true

			// Enable debug logging if requested
			if debug {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}

			// Initialise the ResultPrinter
			if err := initResultPrinter(format); err != nil {
				return err
			}

			return nil
		},
	}

	// Flags that apply to root command and all sub-commands
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVar(&format, "format", "yaml", "Output format (must be one of 'yaml', 'json', or 'quiet')")

	// Add sub-commands
	rootCmd.AddCommand(
		newOrgCommand(ctx),
		newReposCommand(ctx),
		newVersionCommand())

	// We'll take care of logging errors
	rootCmd.SilenceErrors = true

	return rootCmd
}

// initResultPrinter initialises the global ResultPrinter based on the specified format
func initResultPrinter(format string) error {
	switch strings.ToLower(format) {
	case "yaml":
		printer = cmd.NewYAMLResultPrinter(os.Stdout)
	case "json":
		printer = cmd.NewJSONResultPrinter(os.Stdout)
	case "quiet":
		printer = cmd.NewNoOpResultPrinter()
	default:
		return fmt.Errorf("unknown output format '%s'", format)
	}
	return nil
}
