package cli

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

// These are component level tests that test that Cobra commands are wired up correctly. The
// main benefit of these tests is to ensure that certain protections are working correctly,
// such as --dry-run and deletion protection via the RuleEngine. These tests don't need to
// verify that the GitHubService is called with the correct values (e.g. when creating teams)
// as that is already tested within the orgbot package; it is enough to verify that the calls
// are or are not made.

func TestApplyOrgCommandRuleViolations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := orgbot.NewTestPlatform(ctrl)
	lazyPlatform = func() (orgbot.Platform, error) {
		return plat, nil
	}

	wantErr := &orgbot.CompositeRuleError{OrgName: "SEEK-Jobs"}

	// Return an error when the rule engine is run
	plat.MockRuleEngine.
		EXPECT().
		Run(gomock.Any(), gomock.Any()).
		Return(wantErr)

	// Build the command
	args := []string{"org", "apply", "--file=test_data/valid_org.yaml", "--format=quiet"}
	rootCmd := NewRootCommand(context.Background())
	rootCmd.SetArgs(args)

	// Run the SUT
	err := rootCmd.Execute()

	// Verify that the same error is returned
	if diff := cmp.Diff(wantErr, errors.Cause(err)); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestApplyOrgCommandCreateFromScratch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := orgbot.NewTestPlatform(ctrl)
	lazyPlatform = func() (orgbot.Platform, error) {
		return plat, nil
	}

	// Pass rules
	plat.MockRuleEngine.
		EXPECT().
		Run(gomock.Any(), gomock.Any()).
		Return(nil)

	// Return no existing teams
	plat.MockGitHubService.
		EXPECT().
		ListTeams(gomock.Any(), "SEEK-Jobs").
		Return(nil, nil)

	// Return no admins
	plat.MockGitHubService.
		EXPECT().
		ListAdmins(gomock.Any(), "SEEK-Jobs").
		Return(nil, nil)

		// Return users by email address
	plat.MockGitHubService.ExpectUserByEmailAnyTimes(gomock.Any(), "SEEK-Jobs")

	// Expect teams to be created
	plat.MockGitHubService.
		EXPECT().
		CreateTeam(gomock.Any(), "SEEK-Jobs", gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName string, t *orgbot.GitHubTeam) (*orgbot.GitHubTeam, error) {
			return t, nil
		}).
		Times(3) // We expect 3 teams to be created

	// Expect team memberships to be added
	plat.MockGitHubService.
		EXPECT().
		AddTeamMembership(gomock.Any(), "SEEK-Jobs", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(6) // We expect 6 membership additions

	// Build the command
	args := []string{"org", "apply", "--file=test_data/valid_org.yaml", "--format=quiet"}
	rootCmd := NewRootCommand(context.Background())
	rootCmd.SetArgs(args)

	// Run the SUT
	err := rootCmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyOrgCommandDryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := orgbot.NewTestPlatform(ctrl)
	lazyPlatform = func() (orgbot.Platform, error) {
		return plat, nil
	}

	// Pass rules
	plat.MockRuleEngine.
		EXPECT().
		Run(gomock.Any(), gomock.Any()).
		Return(nil)

	// Return no existing teams
	plat.MockGitHubService.
		EXPECT().
		ListTeams(gomock.Any(), "SEEK-Jobs").
		Return(nil, nil)

	// Return no admins
	plat.MockGitHubService.
		EXPECT().
		ListAdmins(gomock.Any(), "SEEK-Jobs").
		Return(nil, nil)

		// Return users by email address
	plat.MockGitHubService.ExpectUserByEmailAnyTimes(gomock.Any(), "SEEK-Jobs")

	// Build the command
	args := []string{"org", "apply", "--file=test_data/valid_org.yaml", "--dry-run", "--format=quiet"}
	rootCmd := NewRootCommand(context.Background())
	rootCmd.SetArgs(args)

	// Run the SUT
	err := rootCmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
}

var (
	haveRepos = []*orgbot.Repo{
		{
			Name: "repo1",
			Teams: []*orgbot.TeamPermission{
				{TeamName: "Foo", Permission: orgbot.RepoPermissionRead},
			},
		},
		{
			Name: "repo2",
		},
	}
)

func TestUpdateRepoTeamsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := orgbot.NewTestPlatform(ctrl)
	lazyPlatform = func() (orgbot.Platform, error) {
		return plat, nil
	}

	// Return teams Foo and Bar
	plat.MockGitHubService.
		EXPECT().
		ListTeams(gomock.Any(), "SEEK-Jobs").
		Return([]*orgbot.GitHubTeam{
			{ID: 100, ParentID: 0, Name: "Foo"},
			{ID: 101, ParentID: 0, Name: "Bar"},
		}, nil)

	// Return repo1 and repo2
	plat.MockGitHubService.
		EXPECT().
		WalkRepos(gomock.Any(), "SEEK-Jobs", gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName string, walkFn orgbot.WalkReposFunc) error {
			for _, r := range haveRepos {
				if err := walkFn(r); err != nil {
					return err
				}
			}
			return nil
		})

	// Expect team Foo to be removed from repo1
	plat.MockGitHubService.
		EXPECT().
		DeleteTeamRepoPermission(gomock.Any(), "SEEK-Jobs", "repo1", orgbot.GitHubTeamID(100)).
		Return(nil)

	// Expect team Bar to be added to repo1 with read permissions
	plat.MockGitHubService.
		EXPECT().
		AddTeamRepoPermission(gomock.Any(), "SEEK-Jobs", "repo1", orgbot.GitHubTeamID(101), orgbot.RepoPermissionRead).
		Return(nil)

	// Build the command
	args := []string{"repos", "update-teams",
		"--org-name=SEEK-Jobs",
		"--remove-read-teams=Foo",
		"--add-read-teams=Bar",
		"--exclude=repo2",
		"--format=quiet"}
	rootCmd := NewRootCommand(context.Background())
	rootCmd.SetArgs(args)

	// Run the SUT
	err := rootCmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRepoTeamsCommandDryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := orgbot.NewTestPlatform(ctrl)
	lazyPlatform = func() (orgbot.Platform, error) {
		return plat, nil
	}

	// Return teams Foo and Bar
	plat.MockGitHubService.
		EXPECT().
		ListTeams(gomock.Any(), "SEEK-Jobs").
		Return([]*orgbot.GitHubTeam{
			{ID: 100, ParentID: 0, Name: "Foo"},
			{ID: 101, ParentID: 0, Name: "Bar"},
		}, nil)

	// Return single repository repo1
	plat.MockGitHubService.
		EXPECT().
		WalkRepos(gomock.Any(), "SEEK-Jobs", gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName string, walkFn orgbot.WalkReposFunc) error {
			for _, r := range haveRepos {
				if err := walkFn(r); err != nil {
					return err
				}
			}
			return nil
		})

	// Build the command
	args := []string{"repos", "update-teams",
		"--org-name=SEEK-Jobs",
		"--remove-read-teams=Foo",
		"--add-read-teams=Bar",
		"--dry-run",
		"--format=quiet"}
	rootCmd := NewRootCommand(context.Background())
	rootCmd.SetArgs(args)

	// Run the SUT
	err := rootCmd.Execute()
	if err != nil {
		t.Fatal(err)
	}
}
