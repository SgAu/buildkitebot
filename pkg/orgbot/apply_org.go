package orgbot

import (
	"context"
	"fmt"

	set "github.com/deckarep/golang-set"
	"github.com/rs/zerolog"
)

// ApplyOrgResult describes the complete set of operations taken by the ApplyOrg function.
type ApplyOrgResult struct {
	TeamsCreated       int `json:"teamsCreated" yaml:"teamsCreated"`
	TeamsUpdated       int `json:"teamsUpdated" yaml:"teamsUpdated"`
	TeamsDeleted       int `json:"teamsDeleted" yaml:"teamsDeleted"`
	MembershipsAdded   int `json:"membershipsAdded" yaml:"membershipsAdded"`
	MembershipsDeleted int `json:"membershipsDeleted" yaml:"membershipsDeleted"`
}

// HasChanges returns whether the apply operation resulted in any changes.
func (r *ApplyOrgResult) HasChanges() bool {
	return *r != ApplyOrgResult{}
}

// keptGitHubTeam extends GitHubTeam to also specify whether the team was freshly created.
type keptGitHubTeam struct {
	GitHubTeam
	created bool
}

// ApplyOrg applies the specified organisational structure against the GitHub organisation
// making the necessary changes to teams and memberships as required.
func ApplyOrg(ctx context.Context, plat Platform, org *Org) (*ApplyOrgResult, error) {
	// First, verify that no rules have been broken
	if err := plat.RuleEngine().Run(ctx, org); err != nil {
		return nil, err
	}

	// Make the GitHubService gather stats so that we print/return results
	statsGitHubService := NewStatsGitHubService(plat.GitHubService())

	// What teams currently exist?
	haveTeams, err := statsGitHubService.ListTeams(ctx, org.Name)
	if err != nil {
		return nil, err
	}

	// Configure the GitHub teams based on the org specification
	keptTeams, err := configureTeams(ctx, statsGitHubService, org, haveTeams)
	if err != nil {
		return nil, err
	}

	// Promote all admins to maintainer roles within the org
	if err = promoteAdmins(ctx, statsGitHubService, org); err != nil {
		return nil, err
	}

	// Configure the memberships for the teams based on the org specification
	if err = configureTeamMemberships(ctx, statsGitHubService, org, keptTeams); err != nil {
		return nil, err
	}

	stats := statsGitHubService.Stats()
	zerolog.Ctx(ctx).Info().Msgf("Created %d teams, updated %d teams, deleted %d teams, added %d memberships, and deleted %d memberships",
		stats.TeamsCreated, stats.TeamsUpdated, stats.TeamsDeleted, stats.TeamMembershipsAdded, stats.TeamMembershipsDeleted)

	return &ApplyOrgResult{
		TeamsCreated:       stats.TeamsCreated,
		TeamsUpdated:       stats.TeamsUpdated,
		TeamsDeleted:       stats.TeamsDeleted,
		MembershipsAdded:   stats.TeamMembershipsAdded,
		MembershipsDeleted: stats.TeamMembershipsDeleted,
	}, nil
}

// promoteAdmins updates the specified Org so that all admin users are moved to team maintainer
// roles rather than member roles. Org admins can't NOT be team maintainers as far as GitHub is
// concerned. Even though we don't allow team maintainers to be specified via YAML, we need to
// represent admin users as maintainers when we talk to GitHub otherwise it looks like we're
// demoting them every single time we do an update.
func promoteAdmins(ctx context.Context, gitHubService GitHubService, org *Org) error {
	admins, err := gitHubService.ListAdmins(ctx, org.Name)
	if err != nil {
		return err
	}

	// isAdmin returns whether the user with the specified email is an org admin.
	isAdmin := func(email string) bool {
		for _, a := range admins {
			if a.Email == email {
				return true
			}
		}
		return false
	}

	// shuffleAdmins moves all admins from the specified member list to the specified maintainer list.
	shuffleAdmins := func(maintainerEmails *[]string, memberEmails *[]string) {
		var updatedMemberEmails []string
		for _, email := range *memberEmails {
			if isAdmin(email) {
				*maintainerEmails = append(*maintainerEmails, email)
			} else {
				updatedMemberEmails = append(updatedMemberEmails, email)
			}
		}

		*memberEmails = updatedMemberEmails
	}

	// process recursively processes the specified desired teams, progressing down the
	// family tree, moving admins from member roles to maintainer roles.
	var process func([]*Team)
	process = func(teams []*Team) {
		for _, t := range teams {
			shuffleAdmins(&t.Maintainers, &t.Members)
			process(t.Children)
		}
	}

	process(org.Teams)
	return nil
}

// configureTeams configures the teams within the GitHub organisation to match the desired state. This function
// returns the slice of all teams in the organisation after all create/update/deletes have occurred.
func configureTeams(ctx context.Context, gitHubService GitHubService, org *Org, haveTeams []*GitHubTeam) ([]*keptGitHubTeam, error) {
	// Collect all teams we're going to keep
	var keptTeams []*keptGitHubTeam

	// process recursively processes the specified desired teams, progressing down the
	// family tree, creating and updating as necessary
	var process func(*GitHubTeam, []*Team) error
	process = func(parent *GitHubTeam, wantTeams []*Team) error {
		for _, want := range wantTeams {
			var t *GitHubTeam
			var created bool
			var err error

			// Update the team if it exists, create it if it doesn't
			if have := findGitHubTeamFromDesired(haveTeams, want); have != nil {
				t, err = updateTeam(ctx, gitHubService, parent, have, want)
			} else {
				created = true
				t, err = createTeam(ctx, gitHubService, org.Name, parent, want)
			}

			if err != nil {
				return err
			}

			keptTeams = append(keptTeams, &keptGitHubTeam{GitHubTeam: *t, created: created})

			if err = process(t, want.Children); err != nil {
				return err
			}
		}

		return nil
	}

	// Process the desired teams, updating and creating as necessary, and collecting
	// the set of team IDs we will retain in keptIDs
	if err := process(nil, org.Teams); err != nil {
		return nil, err
	}

	// Build a set of the IDs of the teams we're keeping
	keptIDs := set.NewSet()
	for _, kept := range keptTeams {
		keptIDs.Add(kept.ID)
	}

	// Delete the teams that were not retained
	for _, t := range haveTeams {
		if !keptIDs.Contains(t.ID) {
			zerolog.Ctx(ctx).Info().Msgf("Deleting GitHub team '%s'", t.Name)
			if err := gitHubService.DeleteTeam(ctx, t.ID); err != nil {
				return nil, err
			}
		}
	}

	return keptTeams, nil
}

// configureTeamMemberships configures team memberships within the organisation to match the desired state.
func configureTeamMemberships(ctx context.Context, gitHubService GitHubService, org *Org, haveTeams []*keptGitHubTeam) error {
	// Transform the wanted teams into a map for easy access
	wantTeamsByName := teamsByName(org.Teams)

	for _, have := range haveTeams {
		want, ok := wantTeamsByName[have.Name]
		if !ok {
			return fmt.Errorf("expected team '%s' to be in desired state", have.Name)
		}

		// Get the current maintainers and members. If the team was freshly created then there is no need to query
		// GitHub as we know that there will be no maintainers or members.
		var haveMaintainers []*GitHubUser
		var haveMembers []*GitHubUser
		if !have.created {
			var err error
			if haveMaintainers, err = gitHubService.ListTeamMembers(ctx, org.Name, have.ID, RoleMaintainer); err != nil {
				return err
			}

			if haveMembers, err = gitHubService.ListTeamMembers(ctx, org.Name, have.ID, RoleMember); err != nil {
				return err
			}
		}

		// Add/remove team maintainers if they have changed
		haveMaintainerEmails := gitHubUserEmails(haveMaintainers)
		if err := updateTeamMemberships(ctx, gitHubService, org.Name, &have.GitHubTeam, haveMaintainerEmails, want.Maintainers, RoleMaintainer); err != nil {
			return err
		}

		// Add/remove team members if they have changed
		haveMemberEmails := gitHubUserEmails(haveMembers)
		if err := updateTeamMemberships(ctx, gitHubService, org.Name, &have.GitHubTeam, haveMemberEmails, want.Members, RoleMember); err != nil {
			return err
		}
	}

	return nil
}

// updateTeam updates the existing GitHub team to match the expected state and have the specified parent.
func updateTeam(ctx context.Context, gitHubService GitHubService, parent *GitHubTeam, have *GitHubTeam, want *Team) (*GitHubTeam, error) {
	// Was the team modified?
	modified := false

	var wantParentID GitHubTeamID
	if parent != nil {
		wantParentID = parent.ID
	}

	// Was the parent changed?
	if have.ParentID != wantParentID {
		if parent != nil {
			zerolog.Ctx(ctx).Info().Msgf("Updating parent of GitHub team '%s' to be '%s'", have.Name, parent.Name)
		} else {
			zerolog.Ctx(ctx).Info().Msgf("Updating GitHub team '%s' to be a top level team", have.Name)
		}
		modified = true
	}

	// Was the description changed?
	if have.Description != want.Description {
		zerolog.Ctx(ctx).Info().Msgf("Update description for GitHub team '%s' to '%s'", have.Name, want.Description)
		modified = true
	}

	// Was the team name changed?
	if have.Name != want.Name {
		zerolog.Ctx(ctx).Info().Msgf("Renaming GitHub team from '%s' to '%s'", have.Name, want.Name)
		modified = true
	}

	// Update the team if it was modified
	updated := have
	if modified {
		t := *have
		t.Name = want.Name
		t.Description = want.Description
		t.ParentID = wantParentID
		tt, err := gitHubService.UpdateTeam(ctx, &t)
		if err != nil {
			return nil, err
		}
		updated = tt
	}

	return updated, nil
}

// createTeam creates the specified desired team with the specified parent.
func createTeam(ctx context.Context, gitHubService GitHubService, orgName string, parent *GitHubTeam, want *Team) (*GitHubTeam, error) {
	var wantParentID GitHubTeamID
	if parent != nil {
		wantParentID = parent.ID
	}

	if parent != nil {
		zerolog.Ctx(ctx).Info().Msgf("Creating GitHub team '%s' as child of '%s'", want.Name, parent.Name)
	} else {
		zerolog.Ctx(ctx).Info().Msgf("Creating GitHub team '%s' as top-level team", want.Name)
	}

	newT := GitHubTeam{
		ParentID:    wantParentID,
		Name:        want.Name,
		Description: want.Description,
	}

	// Create the team
	created, err := gitHubService.CreateTeam(ctx, orgName, &newT)
	if err != nil {
		return nil, err
	}

	return created, nil
}

// updateTeamMemberships updates the specified GitHub team to have the specified users with the specified role.
func updateTeamMemberships(ctx context.Context, gitHubService GitHubService, orgName string, t *GitHubTeam, haveEmails []string, wantEmails []string, role GitHubTeamRole) error {
	haveEmailSet := newStringSet(haveEmails)
	wantEmailSet := newStringSet(wantEmails)

	// Add additional members to the team
	added := wantEmailSet.Difference(haveEmailSet)
	for _, m := range added.ToSlice() {
		u := m.(string)
		zerolog.Ctx(ctx).Info().Msgf("Adding user '%s' to GitHub team '%s' with role '%s'", u, t.Name, role)
		if err := gitHubService.AddTeamMembership(ctx, orgName, t.ID, u, role); err != nil {
			return err
		}
	}

	// Delete removed members from the team
	deleted := haveEmailSet.Difference(wantEmailSet)
	for _, m := range deleted.ToSlice() {
		u := m.(string)
		zerolog.Ctx(ctx).Info().Msgf("Removing user '%s' from GitHub team '%s' with role '%s'", u, t.Name, role)
		if err := gitHubService.DeleteTeamMembership(ctx, orgName, t.ID, u, role); err != nil {
			return err
		}
	}

	return nil
}

// teamsByName transforms the specified slice of teams into a map of team names to Teams.
func teamsByName(teams []*Team) map[string]*Team {
	m := map[string]*Team{}
	var traverse func(wantTeams []*Team)
	traverse = func(wantTeams []*Team) {
		for _, want := range wantTeams {
			m[want.Name] = want
			traverse(want.Children)
		}
	}

	traverse(teams)

	return m
}

// gitHubUserEmails returns a slice of email addresses corresponding to the specified GitHubUsers.
func gitHubUserEmails(users []*GitHubUser) []string {
	var emails []string
	for _, u := range users {
		emails = append(emails, u.Email)
	}

	return emails
}
