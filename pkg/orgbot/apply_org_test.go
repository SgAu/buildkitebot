package orgbot

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
)

// gitHubTeamWithMembers represents a team in GitHub and its members.
type gitHubTeamWithMembers struct {
	team        *GitHubTeam
	maintainers []*GitHubUser
	members     []*GitHubUser
}

var (
	// testOrg is a desired org structure used in the tests.
	testOrg = &Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{
				Name:        "parent",
				Description: "Parent team",
				Maintainers: []string{"david@seek.com.au"},
				Members:     []string{"alice@seek.com.au", "bob@seek.com.au"},
				Children: []*Team{
					{
						Name:        "child1",
						Description: "Child team 1",
						Maintainers: []string{"gavin@seekasia.com"},
						Members:     []string{"lester@seekasia.com", "bob@seek.com.au"},
					},
					{
						Name:        "child2",
						Description: "Child team 2",
						Maintainers: []string{"alice@seek.com.au"},
						Members:     []string{"david@seek.com.au"},
					},
				},
			},
		},
	}

	parentGitHubTeam = asGitHubTeamWithMembers(testOrg.Teams[0], 100, 0)
	child1GitHubTeam = asGitHubTeamWithMembers(testOrg.Teams[0].Children[0], 101, 100)
	child2GitHubTeam = asGitHubTeamWithMembers(testOrg.Teams[0].Children[1], 102, 100)
)

func TestApplyOrgRuleViolations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatform(ctrl)
	ctx := context.Background()
	wantErr := &CompositeRuleError{OrgName: testOrg.Name}

	// Return an error when the rule engine is run
	plat.MockRuleEngine.
		EXPECT().
		Run(ctx, testOrg).
		Return(wantErr)

	_, err := ApplyOrg(context.Background(), plat, testOrg)

	// Verify that the same error is returned by ApplyOrg
	if diff := cmp.Diff(wantErr, err); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestApplyOrgCreateFromScratch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatformPassingRules(ctrl)
	ctx := context.Background()

	// Return zero existing teams
	plat.MockGitHubService.
		EXPECT().
		ListTeams(ctx, testOrg.Name).
		Return(nil, nil)

	// Return no admins
	expectListAdmins(ctx, plat.MockGitHubService, testOrg.Name)

	// Expect user SSO lookups
	plat.MockGitHubService.ExpectUserByEmailAnyTimes(ctx, testOrg.Name)

	// Expect all teams to be created and members added
	expectCreateTeams(ctx, plat.MockGitHubService, testOrg.Name, parentGitHubTeam, child1GitHubTeam, child2GitHubTeam)

	// Run the SUT
	res, err := ApplyOrg(context.Background(), plat, testOrg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify results
	wantRes := &ApplyOrgResult{
		TeamsCreated:     3,
		MembershipsAdded: 8,
	}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestApplyOrgNoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatformPassingRules(ctrl)
	ctx := context.Background()

	// Return the same teams that are represented within testOrg
	plat.MockGitHubService.
		EXPECT().
		ListTeams(ctx, testOrg.Name).
		Return([]*GitHubTeam{parentGitHubTeam.team, child1GitHubTeam.team, child2GitHubTeam.team}, nil)

	// Return no admins
	expectListAdmins(ctx, plat.MockGitHubService, testOrg.Name)

	// Expect user SSO lookups
	plat.MockGitHubService.ExpectUserByEmailAnyTimes(ctx, testOrg.Name)

	// Expect each existing team to have its members listed
	expectListTeamMembers(ctx, plat.MockGitHubService, testOrg.Name, parentGitHubTeam, child1GitHubTeam, child2GitHubTeam)

	// Run the SUT
	res, err := ApplyOrg(context.Background(), plat, testOrg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify results
	wantRes := &ApplyOrgResult{}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestApplyOrgAddTeams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatformPassingRules(ctrl)
	ctx := context.Background()

	// Return the parent and child1
	plat.MockGitHubService.
		EXPECT().
		ListTeams(ctx, testOrg.Name).
		Return([]*GitHubTeam{parentGitHubTeam.team, child1GitHubTeam.team}, nil)

	// Return no admins
	expectListAdmins(ctx, plat.MockGitHubService, testOrg.Name)

	// Expect user SSO lookups
	plat.MockGitHubService.ExpectUserByEmailAnyTimes(ctx, testOrg.Name)

	// Expect each existing team to have its members listed
	expectListTeamMembers(ctx, plat.MockGitHubService, testOrg.Name, parentGitHubTeam, child1GitHubTeam)

	// Expect child2 to be created
	expectCreateTeams(ctx, plat.MockGitHubService, testOrg.Name, child2GitHubTeam)

	// Run the SUT
	res, err := ApplyOrg(context.Background(), plat, testOrg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify results
	wantRes := &ApplyOrgResult{
		TeamsCreated:     1,
		MembershipsAdded: 2,
	}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestApplyOrgDeleteTeams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatformPassingRules(ctrl)
	ctx := context.Background()

	child3Team := &GitHubTeam{
		ID:          103,
		ParentID:    100,
		Name:        "child3",
		Description: "Child team 3",
	}

	// Return the parent, child1, child2, and a new child3
	plat.MockGitHubService.
		EXPECT().
		ListTeams(ctx, testOrg.Name).
		Return([]*GitHubTeam{parentGitHubTeam.team, child1GitHubTeam.team, child2GitHubTeam.team, child3Team}, nil)

	// Return no admins
	expectListAdmins(ctx, plat.MockGitHubService, testOrg.Name)

	// Expect user SSO lookups
	plat.MockGitHubService.ExpectUserByEmailAnyTimes(ctx, testOrg.Name)

	// Expect each that doesn't get deleted to have its members listed
	expectListTeamMembers(ctx, plat.MockGitHubService, testOrg.Name, parentGitHubTeam, child1GitHubTeam, child2GitHubTeam)

	// Expect child3 to be deleted
	plat.MockGitHubService.
		EXPECT().
		DeleteTeam(ctx, GitHubTeamID(103)).
		Return(nil)

	// Run the SUT
	res, err := ApplyOrg(context.Background(), plat, testOrg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify results
	wantRes := &ApplyOrgResult{
		TeamsDeleted: 1,
	}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestApplyOrgUpdateTeams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatformPassingRules(ctrl)
	ctx := context.Background()

	// updatedOrg is a desired org structure used in the tests.
	updatedOrg := &Org{
		Name: "SEEK-Jobs",
		Teams: []*Team{
			{
				Name:        "parent-no-longer", // Rename parent to parent-no-longer
				Previously:  []string{"parent"},
				Description: "Parent no longer", // Update parent's description
				Maintainers: []string{"david@seek.com.au"},
				// We're making Danny an admin so he'll be promoted to maintainer
				Members:  []string{"danny@seek.com.au", "alice@seek.com.au", "bob@seek.com.au"},
				Children: nil, // Update parent to have no children
			},
			{
				Name:        "new-parent",       // Rename child1 to new-parent
				Previously:  []string{"child1"}, // Make child1 a top-level team
				Description: "New parent",       // Update child1's description
				// Add Alice as a maintainer
				Maintainers: []string{"gavin@seekasia.com", "alice@seek.com.au"},
				Members:     []string{"lester@seekasia.com", "bob@seek.com.au"},
				Children: []*Team{
					{
						Name:        "only-child", // Rename child2 to only-child
						Previously:  []string{"child2"},
						Description: "Only child", // Update child2's description
						Maintainers: []string{"alice@seek.com.au"},
						// Remove David and add Bob as a members
						Members: []string{"bob@seek.com.au"},
					},
				},
			},
		},
	}

	// Return the same teams that are represented within testOrg
	plat.MockGitHubService.
		EXPECT().
		ListTeams(ctx, updatedOrg.Name).
		Return([]*GitHubTeam{parentGitHubTeam.team, child1GitHubTeam.team, child2GitHubTeam.team}, nil)

	// Return Alice as an admin
	alice := GitHubUser{Login: "danny", Email: "danny@seek.com.au"}
	expectListAdmins(ctx, plat.MockGitHubService, testOrg.Name, &alice)

	// Expect user SSO lookups
	plat.MockGitHubService.ExpectUserByEmailAnyTimes(ctx, testOrg.Name)

	// Expect each existing team to have its members listed
	expectListTeamMembers(ctx, plat.MockGitHubService, updatedOrg.Name, parentGitHubTeam, child1GitHubTeam, child2GitHubTeam)

	// Updated GitHubTeam structures
	updatedParent := asGitHubTeam(updatedOrg.Teams[0], 100, 0)
	updatedChild1 := asGitHubTeam(updatedOrg.Teams[1], 101, 0)
	updatedChild2 := asGitHubTeam(updatedOrg.Teams[1].Children[0], 102, 101)

	// Expect updates of each team
	plat.MockGitHubService.
		EXPECT().
		UpdateTeam(ctx, updatedParent).
		Return(updatedParent, nil)
	plat.MockGitHubService.
		EXPECT().
		UpdateTeam(ctx, updatedChild1).
		Return(updatedChild1, nil)
	plat.MockGitHubService.
		EXPECT().
		UpdateTeam(ctx, updatedChild2).
		Return(updatedChild2, nil)

		// Expect updates of team memberships
	plat.MockGitHubService.
		EXPECT().
		AddTeamMembership(ctx, "SEEK-Jobs", updatedParent.ID, "danny@seek.com.au", RoleMaintainer).
		Return(nil)
	plat.MockGitHubService.
		EXPECT().
		AddTeamMembership(ctx, "SEEK-Jobs", updatedChild1.ID, "alice@seek.com.au", RoleMaintainer).
		Return(nil)
	plat.MockGitHubService.
		EXPECT().
		AddTeamMembership(ctx, "SEEK-Jobs", updatedChild2.ID, "bob@seek.com.au", RoleMember).
		Return(nil)
	plat.MockGitHubService.
		EXPECT().
		DeleteTeamMembership(ctx, "SEEK-Jobs", updatedChild2.ID, "david@seek.com.au", RoleMember).
		Return(nil)

	// Run the SUT
	res, err := ApplyOrg(context.Background(), plat, updatedOrg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify results
	wantRes := &ApplyOrgResult{
		TeamsUpdated:       3,
		MembershipsAdded:   3,
		MembershipsDeleted: 1,
	}

	if diff := cmp.Diff(wantRes, res); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func expectListTeamMembers(ctx context.Context, gitHubService *MockGitHubService, orgName string, teams ...*gitHubTeamWithMembers) {
	for _, t := range teams {
		gitHubService.
			EXPECT().
			ListTeamMembers(ctx, orgName, t.team.ID, RoleMaintainer).
			Return(t.maintainers, nil)

		gitHubService.
			EXPECT().
			ListTeamMembers(ctx, orgName, t.team.ID, RoleMember).
			Return(t.members, nil)
	}
}

func expectCreateTeams(ctx context.Context, gitHubService *MockGitHubService, orgName string, teams ...*gitHubTeamWithMembers) {
	for _, t := range teams {
		newT := *t.team
		newT.ID = 0
		gitHubService.
			EXPECT().
			CreateTeam(ctx, orgName, &newT).
			Return(t.team, nil)

		for _, login := range gitHubUserEmails(t.maintainers) {
			gitHubService.
				EXPECT().
				AddTeamMembership(ctx, orgName, t.team.ID, login, RoleMaintainer)
		}

		for _, login := range gitHubUserEmails(t.members) {
			gitHubService.
				EXPECT().
				AddTeamMembership(ctx, orgName, t.team.ID, login, RoleMember)
		}
	}
}

func expectListAdmins(ctx context.Context, gitHubService *MockGitHubService, orgName string, admins ...*GitHubUser) {
	gitHubService.
		EXPECT().
		ListAdmins(ctx, orgName).
		Return(admins, nil)
}

func asGitHubTeamWithMembers(t *Team, id, parentID GitHubTeamID) *gitHubTeamWithMembers {
	var maintainers []*GitHubUser
	for _, email := range t.Maintainers {
		maintainers = append(maintainers, &GitHubUser{Login: emailToLogin(email), Email: email})
	}

	var members []*GitHubUser
	for _, email := range t.Members {
		members = append(members, &GitHubUser{Login: emailToLogin(email), Email: email})
	}

	return &gitHubTeamWithMembers{
		team:        asGitHubTeam(t, id, parentID),
		maintainers: maintainers,
		members:     members,
	}
}

func asGitHubTeam(t *Team, id, parentID GitHubTeamID) *GitHubTeam {
	return &GitHubTeam{
		ID:          id,
		ParentID:    parentID,
		Name:        t.Name,
		Description: t.Description,
	}
}
