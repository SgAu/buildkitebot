package github

import (
	"context"
	"fmt"
	"net/url"
	"reflect"

	"github.com/google/go-github/github"
	"github.com/google/go-querystring/query"
)

// listTeamMembers is a copy of ListTeamMembers in github.com/google/go-github/github/teams_members.go
// and modified to NOT apply the `Accept: application/vnd.github.hellcat-preview+json` header. When this
// header is present the client receives all nested members of teams which is not useful to us.
func (s *service) listTeamMembers(ctx context.Context, teamID int64, opt *github.TeamListTeamMembersOptions) ([]*github.User, *github.Response, error) {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return nil, nil, err
	}

	u, err := addOptions(fmt.Sprintf("teams/%v/members", teamID), opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := v3.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var members []*github.User
	resp, err := v3.Do(ctx, req, &members)
	if err != nil {
		return nil, resp, err
	}

	return members, resp, nil
}

// listTeamRepos is a copy of ListTeamRepos in github.com/google/go-github/github/repos.go
// and modified to NOT apply the `Accept: application/vnd.github.hellcat-preview+json` header. When this
// header is present the client receives all repositories that both the specified team and its ancestors
// have access to which is not useful to us.
func (s *service) listTeamRepos(ctx context.Context, teamID int64, opt *github.ListOptions) ([]*github.Repository, *github.Response, error) {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return nil, nil, err
	}

	u, err := addOptions(fmt.Sprintf("teams/%v/repos", teamID), opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := v3.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var repos []*github.Repository
	resp, err := v3.Do(ctx, req, &repos)
	if err != nil {
		return nil, resp, err
	}

	return repos, resp, nil
}

// editTeam is a modified copy of EditTeam in github.com/google/go-github/github/teams.go. The
// original version of the function specifies an 'omitempty' tag on the ParentTeamID property
// of NewTeam which results in the PATCH operation not actually updating the parent. This version
// users an alias struct for NewTeam that doesn't specify the 'omitempty' tag on ParentTeamID.
func (s *service) editTeam(ctx context.Context, teamID int64, team github.NewTeam) (*github.Team, *github.Response, error) {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return nil, nil, err
	}

	type alias struct {
		Name         string  `json:"name"`
		Description  *string `json:"description,omitempty"`
		ParentTeamID *int64  `json:"parent_team_id"`
		Privacy      *string `json:"privacy,omitempty"`
	}
	a := alias{
		Name:         team.Name,
		Description:  team.Description,
		ParentTeamID: team.ParentTeamID,
		Privacy:      team.Privacy,
	}

	u := fmt.Sprintf("teams/%v", teamID)
	req, err := v3.NewRequest("PATCH", u, a)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.hellcat-preview+json")

	t := new(github.Team)
	resp, err := v3.Do(ctx, req, t)
	if err != nil {
		return nil, resp, err
	}

	return t, resp, nil
}

// addOptions is a copy of addOptions in github.com/google/go-github/github/github.go
// which we need to support listTeamMembers above but which is private.
func addOptions(s string, opt interface{}) (string, error) {
	v := reflect.ValueOf(opt)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opt)
	if err != nil {
		return s, err
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}
