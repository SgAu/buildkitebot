package cli

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

// newReposCommand returns the "orgctl repos" sub-command which nests other sub-commands
// for interacting with repositories.
func newReposCommand(ctx context.Context) *cobra.Command {
	dumpCmd := &cobra.Command{
		Use:   "repos",
		Short: "Repository related commands",
	}

	dumpCmd.AddCommand(
		newDumpReposCommand(ctx),
		newUpdateAdminTopicsCommand(ctx),
		newUpdateTeamsCommand(ctx))

	return dumpCmd
}

// newDumpReposCommand returns the "orgctl repos dump" sub-command which dumps repository configuration.
func newDumpReposCommand(ctx context.Context) *cobra.Command {
	var orgName string
	orgCmd := &cobra.Command{
		Use:   "dump",
		Short: "Dumps repository configuration",
		RunE: func(c *cobra.Command, args []string) error {
			plat, err := newReadOnlyPlatform(ctx)
			if err != nil {
				return err
			}

			repos, err := orgbot.DumpRepos(ctx, plat, orgName)
			if err != nil {
				return err
			}

			return printer.Print(repos)
		},
	}

	orgCmd.Flags().StringVar(&orgName, "org-name", "", "Name of the GitHub organisation that owns the repositories")
	_ = orgCmd.MarkFlagRequired("org-name")

	return orgCmd
}

// newUpdateAdminTopicsCommand returns the "orgctl repos update-admin-topics" sub-command which adds
// topics to each repository in the org that indicate the administrators of the repository.
func newUpdateAdminTopicsCommand(ctx context.Context) *cobra.Command {
	var orgName string
	var dryRun bool
	orgCmd := &cobra.Command{
		Use:   "update-admin-topics",
		Short: "Adds topics to repositories to indicate the repository administrators",
		RunE: func(c *cobra.Command, args []string) error {
			plat, err := newPlatform(ctx, dryRun)
			if err != nil {
				return err
			}

			res, err := orgbot.UpdateAdminTopics(ctx, plat, orgName)
			if err != nil {
				return err
			}

			return printer.Print(*res)
		},
	}

	orgCmd.Flags().StringVar(&orgName, "org-name", "", "Name of the GitHub organisation that owns the repositories")
	orgCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate write operations")
	_ = orgCmd.MarkFlagRequired("org-name")

	return orgCmd
}

// newUpdateTeamsCommand returns the "orgctl repos update-teams" sub-command which updates the teams
// assigned to all repositories in the org.
func newUpdateTeamsCommand(ctx context.Context) *cobra.Command {
	var orgName string
	var addReadTeams, removeReadTeams, excludeRepos, onlyRepos []string
	var dryRun bool
	orgCmd := &cobra.Command{
		Use:   "update-teams",
		Short: "Updates teams for all repositories in the organisation",
		RunE: func(c *cobra.Command, args []string) error {
			if len(addReadTeams) == 0 && len(removeReadTeams) == 0 {
				return errors.New("at least one of --add-read-teams or --remove-read-teams must be specified")
			}

			plat, err := newPlatform(ctx, dryRun)
			if err != nil {
				return err
			}

			changeSet := orgbot.RepoTeamsChangeSet{
				RemoveTeams:  teamPermissions(removeReadTeams, orgbot.RepoPermissionRead),
				AddTeams:     teamPermissions(addReadTeams, orgbot.RepoPermissionRead),
				ExcludeRepos: excludeRepos,
				OnlyRepos:    onlyRepos,
			}

			res, err := orgbot.UpdateRepoTeams(ctx, plat, orgName, &changeSet)
			if err != nil {
				return err
			}

			return printer.Print(*res)
		},
	}

	orgCmd.Flags().StringVar(&orgName, "org-name", "", "Name of the GitHub organisation that owns the repositories")
	orgCmd.Flags().StringSliceVar(&addReadTeams, "add-read-teams", nil, "Team to add with read permission")
	orgCmd.Flags().StringSliceVar(&removeReadTeams, "remove-read-teams", nil, "Team with read permission to remove")
	orgCmd.Flags().StringSliceVar(&excludeRepos, "exclude", nil, "Repos that should be excluded from the update")
	orgCmd.Flags().StringSliceVar(&onlyRepos, "only", nil, "Repos that the update should be limited to")
	orgCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate write operations")
	_ = orgCmd.MarkFlagRequired("org-name")

	return orgCmd
}

// teamPermissions is a helper function that returns a slice of TeamPermissions for the
// specified team names with the specified RepoPermission.
func teamPermissions(teamNames []string, permission orgbot.RepoPermission) []*orgbot.TeamPermission {
	var teamPermissions []*orgbot.TeamPermission
	for _, tn := range teamNames {
		teamPermissions = append(teamPermissions, &orgbot.TeamPermission{TeamName: tn, Permission: permission})
	}

	return teamPermissions
}
