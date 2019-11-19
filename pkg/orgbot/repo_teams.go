package orgbot

import (
	"context"
	"fmt"
	"strings"

	set "github.com/deckarep/golang-set"
	"github.com/rs/zerolog"
)

// RepoTeamsChangeSet describes a set of changes to repo team permissions.
type RepoTeamsChangeSet struct {
	AddTeams     []*TeamPermission // Teams with associated permissions to be added
	RemoveTeams  []*TeamPermission // Teams with associated permissions to be removed
	ExcludeRepos []string          // Names of repos to exclude from the change
	OnlyRepos    []string          // Names of repos to limit the update to
}

// ApplyOrgResult describes the complete set of operations taken by the UpdateRepoTeams function.
type UpdateRepoTeamsResult struct {
	TeamPermissionsRemoved int `json:"teamPermissionsRemoved,omitempty" yaml:"teamPermissionsRemoved,omitempty"`
	TeamPermissionsAdded   int `json:"teamPermissionsAdded,omitempty" yaml:"teamPermissionsAdded,omitempty"`
}

// UpdateRepoTeams updates all repos in the org according to the specified change set.
func UpdateRepoTeams(ctx context.Context, plat Platform, orgName string, changeSet *RepoTeamsChangeSet) (*UpdateRepoTeamsResult, error) {
	// Retrieve all teams in the org
	teams, err := plat.GitHubService().ListTeams(ctx, orgName)
	if err != nil {
		return nil, err
	}

	// Organise the teams into a map keyed by team name
	teamsByName := map[string]*GitHubTeam{}
	for _, t := range teams {
		teamsByName[t.Name] = t
	}

	// Ensure the change-set is valid
	if err := validateRepoTeamsChangeSet(changeSet, teamsByName); err != nil {
		return nil, err
	}

	// teamPermissionExists returns whether the specified TeamPermission exists.
	teamPermissionExists := func(haveTeamPermissions []*TeamPermission, wantTeamPermission *TeamPermission) bool {
		for _, haveTeamPermission := range haveTeamPermissions {
			if *haveTeamPermission == *wantTeamPermission {
				return true
			}
		}
		return false
	}

	// repoExcluded returns whether the specified repo should be excluded from the change.
	repoExcluded := func(r *Repo) bool {
		for _, rn := range changeSet.ExcludeRepos {
			if strings.ToLower(r.Name) == strings.ToLower(rn) {
				return true
			}
		}
		return false
	}

	// Result set that we'll accumulate below
	res := UpdateRepoTeamsResult{}

	// walkFn is the function that we execute on each repository in the update list.
	walkFn := func(r *Repo) error {
		if repoExcluded(r) {
			return nil // Skip
		}

		// Calculate the set of teams currently assigned to the repo
		haveTeams := set.NewSet()
		for _, tp := range r.Teams {
			haveTeams.Add(tp.TeamName)
		}

		// Iterate over the teams that need to be removed, deleting each one from the repo
		for _, tp := range changeSet.RemoveTeams {
			if teamPermissionExists(r.Teams, tp) {
				if err := plat.GitHubService().DeleteTeamRepoPermission(ctx, orgName, r.Name, teamsByName[tp.TeamName].ID); err != nil {
					return err
				}

				zerolog.Ctx(ctx).Debug().Msgf("Removed team '%s' from repository '%s' which had permission '%s'", tp.TeamName, r.Name, tp.Permission)
				res.TeamPermissionsRemoved++
			}
		}

		// Iterate over each of the teams that need to be added and add them to the repo with the specified permissions
		for _, tp := range changeSet.AddTeams {
			if !teamPermissionExists(r.Teams, tp) {
				if err := plat.GitHubService().AddTeamRepoPermission(ctx, orgName, r.Name, teamsByName[tp.TeamName].ID, tp.Permission); err != nil {
					return err
				}

				zerolog.Ctx(ctx).Debug().Msgf("Added team '%s' to repository '%s' with permission '%s'", tp.TeamName, r.Name, tp.Permission)
				res.TeamPermissionsAdded++
			}
		}

		return nil
	}

	// Create a walkRepos function that, if OnlyRepos has been set, walks only over the specified repos. If
	// OnlyRepos has not been set, then we default to walking over all repos in the org.
	var walkRepos func(context.Context, string, WalkReposFunc) error
	if len(changeSet.OnlyRepos) > 0 {
		walkRepos = func(ctx context.Context, orgName string, walkFn WalkReposFunc) error {
			for _, repoName := range changeSet.OnlyRepos {
				repo, err := plat.GitHubService().RepoByName(ctx, orgName, repoName)
				if err != nil {
					return err
				}

				if err := walkFn(repo); err != nil {
					return err
				}
			}
			return nil
		}
	} else {
		walkRepos = plat.GitHubService().WalkRepos
	}

	// Walk over the repos, and update each if it is not excluded and an update is warranted
	if err := walkRepos(ctx, orgName, walkFn); err != nil {
		return nil, err
	}

	return &res, nil
}

// validateRepoTeamsChangeSet checks that the specified change-set contains valid teams.
func validateRepoTeamsChangeSet(changeSet *RepoTeamsChangeSet, teamsByName map[string]*GitHubTeam) error {
	for _, tp := range append(changeSet.AddTeams, changeSet.RemoveTeams...) {
		if _, ok := teamsByName[tp.TeamName]; !ok {
			return fmt.Errorf("team '%s' does not exist", tp.TeamName)
		}
	}

	return nil
}
