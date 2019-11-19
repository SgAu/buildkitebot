package orgbot

import (
	"context"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
)

func TestUnknownUsersRule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gitHubService := NewMockGitHubService(ctrl)
	ctx := context.Background()

	gitHubService.
		EXPECT().
		UserByEmail(ctx, "SEEK-Jobs", gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName, email string) (*GitHubUser, error) {
			if strings.HasSuffix(email, "@unknown.com") {
				return nil, &GitHubUserNotFoundError{OrgName: "SEEK-Jobs", Email: email}
			}
			return &GitHubUser{Login: "foobar", Email: email}, nil
		}).
		AnyTimes()

	org := Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{
				Name: "Foo",
				Children: []*Team{
					{
						Name:        "Bar",
						Maintainers: []string{"maintainer-a@unknown.com"},
						Members:     []string{"member-a@bar.com", "member-b@unknown.com"},
					},
				},
			},
		},
	}

	rule := unknownUsersRule{gitHubService: gitHubService}
	err := rule.Run(context.Background(), &org)
	wantError := &UnknownUsersError{
		Violations: map[string][]string{
			"Bar": {"maintainer-a@unknown.com", "member-b@unknown.com"},
		},
	}
	if diff := cmp.Diff(wantError, err); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestTeamNamesUniqueRule(t *testing.T) {
	org := Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{Name: "Foo"},
			{Name: "Bar"},
			{
				Name: "Foo", // Duplicate sibling
				Children: []*Team{
					{Name: "Bar"}, // Duplicate child
				},
			},
		},
	}

	rule := teamNamesUniqueRule{}
	err := rule.Run(context.Background(), &org)
	wantError := &TeamNamesUniqueError{
		Violations: []string{"Foo", "Bar"},
	}
	if diff := cmp.Diff(wantError, err); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestUsersUniqueWithinTeamRule(t *testing.T) {
	org := Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{
				Name:        "Foo",
				Maintainers: []string{"maintainer-a@foo.com", "maintainer-b@foo.com"},             // Unique
				Members:     []string{"member-a@foo.com", "member-b@foo.com", "member-a@foo.com"}, // Non-unique (member-a@foo.com repeated)
			},
			{
				Name: "Bar",
				Children: []*Team{
					{
						Name:        "Baz",
						Maintainers: []string{"member-a@baz.com"},
						Members:     []string{"member-a@baz.com", "member-b@baz.com"}, // member-a@baz.com is both a maintainer and member
					},
				},
			},
		},
	}

	rule := usersUniqueWithinTeamRule{}
	err := rule.Run(context.Background(), &org)
	wantError := &UsersUniqueWithinTeamError{
		Violations: map[string][]string{
			"Foo": {"member-a@foo.com"},
			"Baz": {"member-a@baz.com"},
		},
	}
	if diff := cmp.Diff(wantError, err); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestActiveTeamsDeletionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gitHubService := NewMockGitHubService(ctrl)
	ctx := context.Background()

	// Return the existing GitHub teams
	gitHubService.
		EXPECT().
		ListTeams(ctx, "SEEK-Jobs").
		Return([]*GitHubTeam{
			{
				ID:       100,
				ParentID: 0,
				Name:     "Foo",
			},
			{
				ID:       101,
				ParentID: 100,
				Name:     "Bar",
			},
			{
				ID:       102,
				ParentID: 100,
				Name:     "Baz",
			},
			{
				ID:       103,
				ParentID: 100,
				Name:     "Qux",
			},
		}, nil)

	// walk is the function we'll use to drive the behavior of WalkReposByTeam
	walk := func(ctx context.Context, orgName string, teamID GitHubTeamID, walkFn WalkReposFunc) error {
		if teamID == 103 {
			return nil // Return no repos for team Qux
		}
		// Return two repos for Bar and Baz
		_ = walkFn(&Repo{Name: "repo1"})
		_ = walkFn(&Repo{Name: "repo2"})
		return nil
	}

	// Return repositories for the existing teams
	gitHubService.EXPECT().WalkReposByTeam(ctx, "SEEK-Jobs", GitHubTeamID(101), gomock.Any()).DoAndReturn(walk)
	gitHubService.EXPECT().WalkReposByTeam(ctx, "SEEK-Jobs", GitHubTeamID(102), gomock.Any()).DoAndReturn(walk)
	gitHubService.EXPECT().WalkReposByTeam(ctx, "SEEK-Jobs", GitHubTeamID(103), gomock.Any()).DoAndReturn(walk)

	org := Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{
				Name: "Foo",
			},
		},
	}

	rule := activeTeamDeletionsRule{gitHubService: gitHubService}
	err := rule.Run(context.Background(), &org)
	wantError := &ActiveTeamDeletionsError{
		Violations: map[string][]string{
			"Bar": {"repo1", "repo2"},
			"Baz": {"repo1", "repo2"},
		},
	}
	if diff := cmp.Diff(wantError, err); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestCrossOrgMembershipRule(t *testing.T) {
	tests := []struct {
		name      string
		org       *Org
		wantError error
	}{
		{
			name: "Valid org",
			org: &Org{
				Name: "SEEK-Jobs",
				Teams: []*Team{
					{
						Name:            "Foo",
						Members:         []string{"foo-a@foo.com", "foo-b@foo.com"},
						RestrictMembers: []string{".*@foo.com", ".*@bar.com"},
						Children: []*Team{
							{
								Name:        "Bar",
								Maintainers: []string{"bar-a@bar.com", "bar-b@bar.com"},
								Members:     []string{"foo-a@foo.com"},
							},
						},
					},
				},
			},
			wantError: nil,
		},
		{
			name: "Multiple invalid memberships",
			org: &Org{
				Name: "SEEK-Jobs",
				Teams: []*Team{
					{
						Name:            "Foo",
						Members:         []string{"foo-a@foo.com", "foo-b@foo.com"},
						RestrictMembers: []string{".*@foo.com"},
						Children: []*Team{
							{
								Name:        "Bar",
								Maintainers: []string{"bar-a@bar.com", "bar-b@bar.com"},
								Members:     []string{"foo-a@foo.com"},
							},
						},
					},
				},
			},
			wantError: &CrossOrgMembershipsError{
				Violations: map[string][]string{
					"Bar": {"bar-a@bar.com", "bar-b@bar.com"},
				},
			},
		},
	}

	for _, test := range tests {
		rule := crossOrgMembershipsRule{}
		err := rule.Run(context.Background(), test.org)
		if diff := cmp.Diff(test.wantError, err); diff != "" {
			t.Errorf("Test case '%s': (-want +got)\n%s", test.name, diff)
		}
	}
}

func TestRuleEngineValidOrg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gitHubService := NewMockGitHubService(ctrl)
	ruleEngine := NewRuleEngine(gitHubService)
	ctx := context.Background()

	// Expect unknownUsersRule to query users by email
	gitHubService.
		EXPECT().
		UserByEmail(ctx, "SEEK-Jobs", gomock.Any()).
		Return(&GitHubUser{Login: "foobar", Email: "foo@bar.com"}, nil).
		AnyTimes()

	// Expect activeTeamDeletionsRule to list existing teams
	gitHubService.
		EXPECT().
		ListTeams(ctx, "SEEK-Jobs").
		Return(nil, nil)

	org := Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{
				Name:            "AP&A",
				Maintainers:     []string{"bob@seek.com.au", "alice@seek.com.au"},
				RestrictMembers: []string{".*@seek.com.au", ".*@seekasia.com"},
				Children: []*Team{
					{
						Name:        "Discover-apna",
						Maintainers: []string{"david@seek.com.au"},
						// Gavin and Lester are from seekasia.com.au which is allowed here
						Members: []string{"gavin@seekasia.com", "lester@seekasia.com"},
					},
				},
			},
			{
				Name:            "SEEK ANZ",
				Maintainers:     []string{"bob@seek.com.au", "alice@seek.com.au"},
				Members:         []string{"david@seek.com.au"},
				RestrictMembers: []string{".*@seek.com.au"},
			},
			{
				Name:    "00archive",
				Members: []string{}, // Team not allowed to have any members
			},
		},
	}

	err := ruleEngine.Run(ctx, &org)
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestRuleEngineCompositeRuleError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gitHubService := NewMockGitHubService(ctrl)
	ruleEngine := NewRuleEngine(gitHubService)
	ctx := context.Background()

	// Return GitHubUserNotFoundError for mary@seek.com.au and success for anything else
	gitHubService.
		EXPECT().
		UserByEmail(ctx, "SEEK-Jobs", gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName, email string) (*GitHubUser, error) {
			if email == "mary@seek.com.au" {
				return nil, &GitHubUserNotFoundError{OrgName: "SEEK-Jobs", Email: email}
			}
			return &GitHubUser{Login: "foobar", Email: email}, nil
		}).
		AnyTimes()

	// Return a single existing team that is not represented in the desired org
	gitHubService.
		EXPECT().
		ListTeams(ctx, "SEEK-Jobs").
		Return([]*GitHubTeam{
			{
				ID:       100,
				ParentID: 0,
				Name:     "Deleted Team",
			},
		}, nil)

	// Return a repository for the single existing team
	gitHubService.
		EXPECT().
		WalkReposByTeam(ctx, "SEEK-Jobs", GitHubTeamID(100), gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName string, teamID GitHubTeamID, walkFn WalkReposFunc) error {
			return walkFn(&Repo{Name: "repo"})
		})

	org := Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{
				Name:            "AP&A",
				Maintainers:     []string{"bob@seek.com.au", "alice@seek.com.au"},
				RestrictMembers: []string{".*@seek.com.au", ".*@seekasia.com"},
				Children: []*Team{
					{
						Name:        "Discover-apna",
						Maintainers: []string{"david@seek.com.au"},
						// Gavin and Lester are from seekasia.com.au which is allowed here
						Members: []string{"gavin@seekasia.com", "lester@seekasia.com"},
					},
				},
			},
			{
				Name:        "SEEK ANZ",
				Maintainers: []string{"bob@seek.com.au", "alice@seek.com.au"},
				// Gavin and Lester are from seekasia.com.au which is NOT allowed here
				Members:         []string{"gavin@seekasia.com", "lester@seekasia.com"},
				RestrictMembers: []string{".*@seek.com.au"},
				Children: []*Team{
					{
						Name: "00archive", // Duplicate team name
					},
				},
			},
			{
				Name:            "Jora",
				Maintainers:     []string{"david@seek.com.au", "bob@seek.com.au", "mary@seek.com.au"},    // Mary is an unknown user
				Members:         []string{"alice@seek.com.au", "david@seek.com.au", "alice@seek.com.au"}, // Non-unique users in team
				RestrictMembers: []string{".*@seek.com.au"},
			},
			{
				Name:    "00archive",
				Members: []string{"david@seek.com.au"}, // Team not allowed to have any members
			},
		},
	}

	err := ruleEngine.Run(ctx, &org)

	wantError := &CompositeRuleError{
		OrgName: "SEEK-Jobs",
		Errors: []RuleError{
			&UnknownUsersError{
				Violations: map[string][]string{
					"Jora": {"mary@seek.com.au"},
				},
			},
			&TeamNamesUniqueError{
				Violations: []string{"00archive"},
			},
			&UsersUniqueWithinTeamError{
				Violations: map[string][]string{
					"Jora": {"david@seek.com.au", "alice@seek.com.au"},
				},
			},
			&ActiveTeamDeletionsError{
				Violations: map[string][]string{
					"Deleted Team": {"repo"},
				},
			},
			&CrossOrgMembershipsError{
				Violations: map[string][]string{
					"SEEK ANZ":  {"gavin@seekasia.com", "lester@seekasia.com"},
					"00archive": {"david@seek.com.au"},
				},
			},
		},
	}
	if diff := cmp.Diff(wantError, err); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestTeamNamesLengthRule(t *testing.T) {
	org := Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{Name: "a"},
			{Name: "team-name"},
		},
	}

	rule := teamNameLengthRule{maxLength: 2}
	err := rule.Run(context.Background(), &org)
	wantError := &TeamNameLengthError{
		Violations:    []string{"team-name"},
		MaxNameLength: 2,
	}
	if diff := cmp.Diff(wantError, err); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}
