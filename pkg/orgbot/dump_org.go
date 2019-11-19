package orgbot

import (
	"context"
	"sort"
)

const (
	// demoteMaintainers indicates whether team maintainers should
	// be demoted to be members when dumping the org configuration.
	demoteMaintainers = true
)

// DumpOrg returns the current structure of the specified GitHub organisation.
func DumpOrg(ctx context.Context, plat Platform, orgName string) (*Org, error) {
	output := Org{Name: orgName}

	idMap := map[GitHubTeamID]*Team{}             // Teams by ID
	children := map[GitHubTeamID][]GitHubTeamID{} // Team children by ID
	var tops []GitHubTeamID                       // Top-level teams

	// Retrieve all existing teams
	teams, err := plat.GitHubService().ListTeams(ctx, orgName)
	if err != nil {
		return nil, err
	}

	for _, t := range teams {
		if t.ParentID == 0 { // Top level team
			tops = append(tops, t.ID)
		} else {
			children[t.ParentID] = append(children[t.ParentID], t.ID)
		}

		// Convert the GitHub team to the generalised Team and retrieve it's members
		team, err := buildOrgTeam(ctx, plat, orgName, t)
		if err != nil {
			return nil, err
		}

		idMap[t.ID] = team
	}

	var makeTeam func(id GitHubTeamID) *Team
	makeTeam = func(id GitHubTeamID) *Team {
		t := idMap[id]
		for _, cid := range children[id] {
			child := makeTeam(cid)
			t.Children = append(t.Children, child)
		}

		SortTeam(t)

		return t
	}

	for _, id := range tops {
		output.Teams = append(output.Teams, makeTeam(id))
	}

	return &output, nil
}

// buildOrgTeam converts the specified GitHub team representation to a Team appending
// all of the direct members of the team.
func buildOrgTeam(ctx context.Context, plat Platform, orgName string, t *GitHubTeam) (*Team, error) {
	team := Team{
		Name:        t.Name,
		Description: t.Description,
	}

	// Assign team maintainers
	maintainers, err := plat.GitHubService().ListTeamMembers(ctx, orgName, t.ID, RoleMaintainer)
	if err != nil {
		return nil, err
	}
	team.Maintainers = gitHubUserEmails(maintainers)

	// Assign team members
	members, err := plat.GitHubService().ListTeamMembers(ctx, orgName, t.ID, RoleMember)
	if err != nil {
		return nil, err
	}
	team.Members = gitHubUserEmails(members)

	// Demotion of maintainers is useful for the transition from manual team administration
	// to automated administration via Orgbot. It might also come in handy if rogue admins
	// inadvertently make people maintainers and we need to demote them.
	if demoteMaintainers {
		team.Members = append(team.Members, team.Maintainers...)
		team.Maintainers = nil
	}

	// Sort members for consistency
	sort.Strings(team.Maintainers)
	sort.Strings(team.Members)

	return &team, nil
}
