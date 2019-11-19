package cli

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

// newOrgCommand returns the "orgctl org" sub-command which nests other sub-commands
// for interacting with org/team resources.
func newOrgCommand(ctx context.Context) *cobra.Command {
	orgCmd := &cobra.Command{
		Use:   "org",
		Short: "Organisation related commands",
	}

	orgCmd.AddCommand(
		newApplyOrgCommand(ctx),
		newDumpOrgCommand(ctx),
		newMergeOrgCommand(ctx))

	return orgCmd
}

// newApplyOrgCommand returns the "orgctl org apply" sub-command which applies org/team configuration.
func newApplyOrgCommand(ctx context.Context) *cobra.Command {
	var dir string
	var file string
	var dryRun bool
	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Applies organisation and team configuration",
		RunE: func(c *cobra.Command, args []string) error {
			plat, err := newPlatform(ctx, dryRun)
			if err != nil {
				return err
			}

			org, err := readOrg(plat.Codec(), file, dir)
			if err != nil {
				return err
			}

			return applyOrg(ctx, plat, org)
		},
	}

	applyCmd.Flags().StringVar(&dir, "dir", "", "The org directory to apply")
	applyCmd.Flags().StringVar(&file, "file", "", "The org config file to apply")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate write operations")

	return applyCmd
}

// newApplyOrgCommand returns the "orgctl org dump" sub-command which dumps org/team configuration.
func newDumpOrgCommand(ctx context.Context) *cobra.Command {
	var orgName string
	var dir string
	dumpCmd := &cobra.Command{
		Use:   "dump",
		Short: "Dumps organisation and team configuration",
		RunE: func(c *cobra.Command, args []string) error {
			plat, err := newReadOnlyPlatform(ctx)
			if err != nil {
				return err
			}

			return dumpOrg(ctx, plat, orgName, dir)
		},
	}

	dumpCmd.Flags().StringVar(&orgName, "org-name", "", "Name of the GitHub organisation to dump")
	dumpCmd.Flags().StringVar(&dir, "dir", "", "Directory to write org hierarchy to")
	_ = dumpCmd.MarkFlagRequired("org-name")

	return dumpCmd
}

// newMergeOrgCommand returns the "orgctl org merge" sub-command which merges org/team configuration.
func newMergeOrgCommand(ctx context.Context) *cobra.Command {
	var dir string
	mergeCmd := &cobra.Command{
		Use:   "merge",
		Short: "Merges organisation and team configuration",
		RunE: func(c *cobra.Command, args []string) error {
			plat, err := newReadOnlyPlatform(ctx)
			if err != nil {
				return err
			}

			org, err := readOrgDir(plat.Codec(), dir)
			if err != nil {
				return err
			}

			return printer.Print(*org)
		},
	}

	mergeCmd.Flags().StringVar(&dir, "dir", "", "The org directory to merge")

	return mergeCmd
}

// readOrg is a helper function for reading org configuration from either file or directory.
func readOrg(codec orgbot.Codec, file string, dir string) (*orgbot.Org, error) {
	if dir != "" && file != "" {
		return nil, fmt.Errorf("either --dir or --file must be specified but not both")
	}

	if file != "" {
		return readOrgFile(codec, file)
	} else if dir != "" {
		return readOrgDir(codec, dir)
	}

	return nil, fmt.Errorf("either --dir or --file must be specified")
}

// readOrgFile reads org configuration from file.
func readOrgFile(codec orgbot.Codec, file string) (*orgbot.Org, error) {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read %s", file)
	}

	var org orgbot.Org
	if err = codec.Decode(buf, &org); err != nil {
		return nil, errors.Wrapf(err, "could not unmarshal %s", file)
	}

	return &org, nil
}

// readOrgDir reads org configuration in its unmerged state from the specified directory.
func readOrgDir(codec orgbot.Codec, dir string) (*orgbot.Org, error) {
	org, err := orgbot.MergeOrg(codec, dir)
	if err != nil {
		return nil, errors.Wrapf(err, "could not merge dir %s", dir)
	}

	return org, nil
}

// applyOrg applies the specified org configuration against GitHub.
func applyOrg(ctx context.Context, plat orgbot.Platform, org *orgbot.Org) error {
	res, err := orgbot.ApplyOrg(ctx, plat, org)
	if err != nil {
		return errors.Wrapf(err, "error applying org")
	}

	return printer.Print(*res)
}

// dumpOrg dumps the specified org to the specified directory if it is non-empty or otherwise to stdout.
func dumpOrg(ctx context.Context, plat orgbot.Platform, orgName string, dir string) error {
	org, err := orgbot.DumpOrg(ctx, plat, orgName)
	if err != nil {
		return errors.Wrap(err, "error dumping org")
	}

	// If the --dir flag was specified write the unmerged output to the specified directory
	if dir != "" {
		return orgbot.UnmergeOrg(plat.Codec(), org, dir)
	}

	// Otherwise write to stdout
	return printer.Print(*org)
}
