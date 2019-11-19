package orgbot

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
)

func testRepos() []*Repo {
	return []*Repo{
		{
			Name:   "repo1",
			Topics: []string{"blue", "red", "green"}, // No admin teams as topics
			Teams: []*TeamPermission{
				{TeamName: "Foo", Permission: RepoPermissionAdmin},
				{TeamName: "Bar", Permission: RepoPermissionAdmin},
				{TeamName: "Baz", Permission: RepoPermissionRead},
			},
		},
		{
			Name:   "repo2",
			Topics: []string{"admin-foo", "admin-old", "one", "two", "three"}, // One admin team and one former admin team as topics
			Teams: []*TeamPermission{
				{TeamName: "Foo", Permission: RepoPermissionAdmin},
				{TeamName: "Bar", Permission: RepoPermissionAdmin},
			},
		},
		{
			Name:   "repo3",
			Topics: []string{"admin-foo", "admin-baz", "ichi", "ni", "san", "shi"}, // All admin teams as topics
			Teams: []*TeamPermission{
				{TeamName: "Foo", Permission: RepoPermissionAdmin},
				{TeamName: "Baz", Permission: RepoPermissionAdmin},
				{TeamName: "Qux", Permission: RepoPermissionRead},
			},
		},
	}
}

func TestUpdateAdminTopics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatform(ctrl)
	ctx := context.Background()

	haveRepos := testRepos()

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

	// Expect the repos without correct admin topics to be updated
	plat.MockGitHubService.
		EXPECT().
		UpdateRepoTopics(ctx, "SEEK-Jobs", "repo1", []string{"admin-bar", "admin-foo", "blue", "green", "red"}).
		Return(nil)
	plat.MockGitHubService.
		EXPECT().
		UpdateRepoTopics(ctx, "SEEK-Jobs", "repo2", []string{"admin-bar", "admin-foo", "one", "three", "two"}).
		Return(nil)

	// Run the SUT
	res, err := UpdateAdminTopics(ctx, plat, "SEEK-Jobs")
	if err != nil {
		t.Fatal(err)
	}

	wantRes := &UpdateAdminTopicsResult{ReposUpdated: 2}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestUpdateRepoAdminTopics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatform(ctrl)
	ctx := context.Background()

	plat.MockGitHubService.
		EXPECT().
		RepoByName(ctx, "SEEK-Jobs", "repo1").
		Return(&Repo{
			Name:   "repo1",
			Topics: []string{"blue", "red", "green"},
			Teams: []*TeamPermission{
				{TeamName: "Foo", Permission: RepoPermissionAdmin},
				{TeamName: "Bar", Permission: RepoPermissionAdmin},
				{TeamName: "Baz", Permission: RepoPermissionRead},
			},
		}, nil)

	// Expect the repos without correct admin topics to be updated
	plat.MockGitHubService.
		EXPECT().
		UpdateRepoTopics(ctx, "SEEK-Jobs", "repo1", []string{"admin-bar", "admin-foo", "blue", "green", "red"}).
		Return(nil)

	res, err := UpdateRepoAdminTopics(ctx, plat, "SEEK-Jobs", "repo1")
	if err != nil {
		t.Fatal(err)
	}

	wantRes := &UpdateAdminTopicsResult{ReposUpdated: 1}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestUpdateTeamAdminTopics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatform(ctrl)
	ctx := context.Background()

	haveRepos := testRepos()

	plat.MockGitHubService.
		EXPECT().
		WalkReposByTeam(ctx, "SEEK-Jobs", GitHubTeamID(123), gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName string, teamID GitHubTeamID, walkFn WalkReposFunc) error {
			for _, r := range haveRepos {
				if err := walkFn(r); err != nil {
					return err
				}
			}
			return nil
		})

	// Expect the repos without correct admin topics to be updated
	plat.MockGitHubService.
		EXPECT().
		UpdateRepoTopics(ctx, "SEEK-Jobs", "repo1", []string{"admin-bar", "admin-foo", "blue", "green", "red"}).
		Return(nil)
	plat.MockGitHubService.
		EXPECT().
		UpdateRepoTopics(ctx, "SEEK-Jobs", "repo2", []string{"admin-bar", "admin-foo", "one", "three", "two"}).
		Return(nil)

	res, err := UpdateTeamAdminTopics(ctx, plat, "SEEK-Jobs", GitHubTeamID(123))
	if err != nil {
		t.Fatal(err)
	}

	wantRes := &UpdateAdminTopicsResult{ReposUpdated: 2}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}
