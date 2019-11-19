package orgbot

import (
	"context"
	"sort"
	"strings"

	set "github.com/deckarep/golang-set"
	"github.com/rs/zerolog"
)

// adminTopicPrefix is the prefix prepended to normalised team names and then applied
// as repository topics to provide visibility of the admin teams.
const adminTopicPrefix = "admin-"
const maxTopicLength = 35

type UpdateAdminTopicsResult struct {
	ReposUpdated int `json:"reposUpdated,omitempty" yaml:"reposUpdated,omitempty"`
}

// UpdateAdminTopics updates all repositories in the specified organisation to include topics
// that specify the administrator teams of the repository.
func UpdateAdminTopics(ctx context.Context, plat Platform, orgName string) (*UpdateAdminTopicsResult, error) {
	// Collect the repositories that need to be updated
	var repos []*Repo
	if err := plat.GitHubService().WalkRepos(ctx, orgName, func(r *Repo) error {
		if rectifyRepoAdminTopics(r) {
			repos = append(repos, r)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Update each of the repositories to include the admin topics
	for _, repo := range repos {
		zerolog.Ctx(ctx).Info().Msgf("Updating repo %s/%s to have topics %s", orgName, repo.Name, strings.Join(repo.Topics, ", "))
		err := plat.GitHubService().UpdateRepoTopics(ctx, orgName, repo.Name, repo.Topics)
		if err != nil {
			return nil, err
		}
	}

	return &UpdateAdminTopicsResult{
		ReposUpdated: len(repos),
	}, nil
}

// rectifyRepoAdminTopics updates the specified repo to include topics that specify the
// administrator teams. It returns true if changes were made to the repo, otherwise false.
func rectifyRepoAdminTopics(r *Repo) bool {
	haveTopics := newStringSet(r.Topics)

	// Create a set of topics that we want applied, starting with all of the existing
	// topics minus any admin topics. We'll add the admin topics to the set below.
	wantTopics := set.NewSet()
	for _, topic := range r.Topics {
		if !strings.HasPrefix(topic, adminTopicPrefix) {
			wantTopics.Add(topic)
		}
	}

	// Add the admin topics to the desired set
	for _, team := range r.Teams {
		if team.Permission == RepoPermissionAdmin {
			adminTopic := adminTopicPrefix + normaliseName(team.TeamName)
			if len(adminTopic) >= maxTopicLength {
				adminTopic = adminTopic[:maxTopicLength-1]
			}
			wantTopics.Add(adminTopic)
		}
	}

	// If the sets are equal then we don't need to do anything
	if haveTopics.Equal(wantTopics) {
		return false
	}

	r.Topics = stringSetToSlice(wantTopics)

	// Sort the topics so that they can be predictably tested
	sort.Strings(r.Topics)

	return true
}

// UpdateRepoAdminTopics updates the admin topics for the given repo
func UpdateRepoAdminTopics(ctx context.Context, plat Platform, orgName, repoName string) (*UpdateAdminTopicsResult, error) {
	repo, err := plat.GitHubService().RepoByName(ctx, orgName, repoName)
	if err != nil {
		return nil, err
	}

	// Update repo topics
	if rectifyRepoAdminTopics(repo) {
		// Update repo
		err = plat.GitHubService().UpdateRepoTopics(ctx, orgName, repoName, repo.Topics)
		if err != nil {
			return nil, err
		}
	}

	return &UpdateAdminTopicsResult{
		ReposUpdated: 1,
	}, nil
}

// UpdateTeamAdminTopics updates all the topics of all repos the given team has admin permission for
func UpdateTeamAdminTopics(ctx context.Context, plat Platform, orgName string, teamID GitHubTeamID) (*UpdateAdminTopicsResult, error) {
	updatedRepos := 0
	err := plat.GitHubService().WalkReposByTeam(ctx, orgName, teamID, func(r *Repo) error {
		if rectifyRepoAdminTopics(r) {
			// Update repo
			err := plat.GitHubService().UpdateRepoTopics(ctx, orgName, r.Name, r.Topics)
			if err != nil {
				return err
			}
			updatedRepos++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &UpdateAdminTopicsResult{
		ReposUpdated: updatedRepos,
	}, nil
}
