package orgbot

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
)

func TestUpdateRepoTeams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatform(ctrl)
	ctx := context.Background()

	haveRepos := testRepos()

	plat.MockGitHubService.
		EXPECT().
		ListTeams(ctx, "SEEK-Jobs").
		Return([]*GitHubTeam{
			{ID: 100, ParentID: 0, Name: "Foo"},
			{ID: 101, ParentID: 100, Name: "Bar"},
			{ID: 102, ParentID: 100, Name: "Baz"},
			{ID: 103, ParentID: 100, Name: "Qux"},
		}, nil)

	// Expect all repos in the org to be walked over
	plat.MockGitHubService.
		EXPECT().
		WalkRepos(ctx, "SEEK-Jobs", gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName string, walkFn WalkReposFunc) error {
			for _, r := range haveRepos {
				if err := walkFn(r); err != nil {
					return err
				}
			}
			return nil
		})

	// Expect team Baz to be removed from repo1 (not from repo3 as the permissions are different)
	plat.MockGitHubService.
		EXPECT().
		DeleteTeamRepoPermission(ctx, "SEEK-Jobs", "repo1", GitHubTeamID(102)).
		Return(nil)

	// Expect team Qux to be added to repo1 (repo2 is excluded and Qux already has read permissions on repo3)
	plat.MockGitHubService.
		EXPECT().
		AddTeamRepoPermission(ctx, "SEEK-Jobs", "repo1", GitHubTeamID(103), RepoPermissionRead).
		Return(nil)

	// Create a changeSet that instructs to remove Baz from all repos and add Qux with read permissions
	changeSet := &RepoTeamsChangeSet{
		RemoveTeams: []*TeamPermission{
			{TeamName: "Baz", Permission: RepoPermissionRead},
		},
		AddTeams: []*TeamPermission{
			{TeamName: "Qux", Permission: RepoPermissionRead},
		},
		ExcludeRepos: []string{
			"repo2",
		},
	}

	// Run the SUT
	res, err := UpdateRepoTeams(ctx, plat, "SEEK-Jobs", changeSet)
	if err != nil {
		t.Fatal(err)
	}

	wantRes := &UpdateRepoTeamsResult{
		TeamPermissionsRemoved: 1,
		TeamPermissionsAdded:   1,
	}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestUpdateRepoTeamsOnlyRepos(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatform(ctrl)
	ctx := context.Background()

	plat.MockGitHubService.
		EXPECT().
		ListTeams(ctx, "SEEK-Jobs").
		Return([]*GitHubTeam{
			{ID: 100, ParentID: 0, Name: "Foo"},
			{ID: 101, ParentID: 100, Name: "Bar"},
			{ID: 102, ParentID: 100, Name: "Baz"},
			{ID: 103, ParentID: 100, Name: "Qux"},
		}, nil)

	repo1 := Repo{
		Name:   "repo1",
		Topics: []string{"blue", "red", "green"}, // No admin teams as topics
		Teams: []*TeamPermission{
			{TeamName: "Foo", Permission: RepoPermissionAdmin},
			{TeamName: "Bar", Permission: RepoPermissionAdmin},
			{TeamName: "Baz", Permission: RepoPermissionRead},
		},
	}
	repo2 := Repo{Name: "repo2"}

	// Expect repositories in the OnlyRepos list to be retrieved individually
	plat.MockGitHubService.
		EXPECT().
		RepoByName(ctx, "SEEK-Jobs", "repo1").
		Return(&repo1, nil)
	plat.MockGitHubService.
		EXPECT().
		RepoByName(ctx, "SEEK-Jobs", "repo2").
		Return(&repo2, nil)

	// Expect team Baz to be removed from repo1
	plat.MockGitHubService.
		EXPECT().
		DeleteTeamRepoPermission(ctx, "SEEK-Jobs", "repo1", GitHubTeamID(102)).
		Return(nil)

	// Expect team Qux to be added to repo1
	plat.MockGitHubService.
		EXPECT().
		AddTeamRepoPermission(ctx, "SEEK-Jobs", "repo1", GitHubTeamID(103), RepoPermissionRead).
		Return(nil)

	// Create a changeSet that instructs to remove Baz from all repos and add Qux with read permissions
	changeSet := &RepoTeamsChangeSet{
		RemoveTeams: []*TeamPermission{
			{TeamName: "Baz", Permission: RepoPermissionRead},
		},
		AddTeams: []*TeamPermission{
			{TeamName: "Qux", Permission: RepoPermissionRead},
		},
		ExcludeRepos: []string{
			"repo2",
		},
		OnlyRepos: []string{
			"repo1", "repo2",
		},
	}

	// Run the SUT
	res, err := UpdateRepoTeams(ctx, plat, "SEEK-Jobs", changeSet)
	if err != nil {
		t.Fatal(err)
	}

	wantRes := &UpdateRepoTeamsResult{
		TeamPermissionsRemoved: 1,
		TeamPermissionsAdded:   1,
	}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}
