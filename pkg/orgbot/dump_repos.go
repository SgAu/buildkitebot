package orgbot

import (
	"context"
)

// DumpRepos returns the current configuration of repositories within the specified GitHub organisation.
func DumpRepos(ctx context.Context, plat Platform, orgName string) ([]*Repo, error) {
	var repos []*Repo
	if err := plat.GitHubService().WalkRepos(ctx, orgName, func(r *Repo) error {
		repos = append(repos, r)
		return nil
	}); err != nil {
		return nil, err
	}

	return repos, nil
}
