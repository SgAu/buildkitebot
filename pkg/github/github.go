package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/shurcooL/githubv4"

	"github.com/SEEK-Jobs/orgbot/pkg/aws"
	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

const (
	// pageSize is the number of items to return per page in paged responses
	pageSize = 100

	// privacyClosed specifies the repository is visible to all members of the organisation
	privacyClosed = "closed"
	// privacySecret specifies the repository can only be seen by its members and may not be nested
	privacySecret = "secret"

	// userInfoS3Key is the S3 key of the file that contains user mappings
	userInfoS3Key = "github-saml-mapping.json"
	// userCacheTTL defines the TTL of user cache before we refetch
	userCacheTTL = 2 * time.Minute

	// We currently only support the SEEK-Jobs GitHub organisation as we only have user information
	// for users within SEEK-Jobs.
	supportedOrgName = "SEEK-Jobs"
)

// userCache defines a map of GitHub org name to a multi-map of users that are members of that
// org. The multi-map is keyed of both login name and email address.
type userCache map[string]map[string]*orgbot.GitHubUser

// service provides the read-write implementation of orgbot.GitHubService.
type service struct {
	ClientFactory
	config        *orgbot.Config
	s3Client      *aws.S3
	userCacheMu   sync.Mutex
	userCacheTime time.Time
	userCache     userCache
}

// walkTeamsFunc is the type of the function called for each team in the GitHub org
// by the walkTeams function. If the function returns an error walking stops.
type walkTeamsFunc func(t *orgbot.GitHubTeam) error

// walkTeamMembersFunc is the type of the function called for each member's username within
// a team by the walkTeamMembers function. If the function returns an error walking stops.
type walkTeamMembersFunc func(u *orgbot.GitHubUser) error

// reposQuery is used for retrieving information about repos from the Graphql API.
type reposQuery struct {
	Org struct {
		Repositories struct {
			PageInfo pageInfo
			Nodes    []repoNode
		} `graphql:"repositories(first: $first, after: $cursor)"`
	} `graphql:"organization(login: $org)"`
}

// pageInfo is the information needed for paging the Graphql API.
type pageInfo struct {
	EndCursor   string
	HasNextPage bool
}

// repoNode is the repository information returned by the Graphql API.
type repoNode struct {
	Id               string
	Name             string
	IsArchived       bool
	RepositoryTopics struct {
		Nodes []topicNode
	} `graphql:"repositoryTopics(first: $first)"`
}

// repoQuery is used to retrieve a single repo from the given org with the name repoName.
type repoQuery struct {
	Data struct {
		Id               string
		Name             string
		IsArchived       bool
		RepositoryTopics struct {
			Nodes []topicNode
		} `graphql:"repositoryTopics(first: $first)"`
	} `graphql:"repository(owner: $org, name: $repoName)"`
}

// topicNode is the topic information returned by the Graphql API.
type topicNode struct {
	URL          string `graphql:"url"`
	ResourcePath string
}

// userInfo is the type that represents the audited user information stored in S3.
type userInfo struct {
	GitHubUser string `json:"github_user"`
	SAMLUser   string `json:"saml_user"`
	SCIMUser   string `json:"scim_user"`
}

// NewService returns a configured GitHubService implementation.
func NewService(c *orgbot.Config, f *ClientFactory, sess *session.Session) (orgbot.GitHubService, error) {
	s := service{
		ClientFactory: *f,
		config:        c,
		s3Client:      aws.NewS3(sess),
		userCache:     map[string]map[string]*orgbot.GitHubUser{},
	}

	return &s, nil
}

// ListTeams implements orgbot.GitHubService.
func (s *service) ListTeams(ctx context.Context, orgName string) ([]*orgbot.GitHubTeam, error) {
	var teams []*orgbot.GitHubTeam
	if err := s.walkTeams(ctx, orgName, func(t *orgbot.GitHubTeam) error {
		teams = append(teams, t)
		return nil
	}); err != nil {
		return nil, err
	}

	return teams, nil
}

// walkTeams walks all the teams in the GitHub organisation passing each team to the
// walk function. If walkFn returns an error walking stops and the error is returned.
func (s *service) walkTeams(ctx context.Context, orgName string, walkFn walkTeamsFunc) error {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return err
	}

	opts := github.ListOptions{PerPage: pageSize}

	// Loop until there are no more pages of teams
	for {
		teams, r, err := v3.Teams.ListTeams(ctx, orgName, &opts)
		if err != nil {
			return err
		}

		// For each team in the paged response
		for _, t := range teams {
			// Ignore secret teams
			if *t.Privacy == privacySecret {
				continue
			}

			if err := walkFn(asDomainTeam(t)); err != nil {
				return err
			}
		}

		// Are we done with paging through the teams?
		if r.NextPage == 0 {
			break
		}

		// Not done yet
		opts.Page = r.NextPage
	}

	return nil
}

// ListTeamMembers implements orgbot.GitHubService.
func (s *service) ListTeamMembers(ctx context.Context, orgName string, teamID orgbot.GitHubTeamID, role orgbot.GitHubTeamRole) ([]*orgbot.GitHubUser, error) {
	var users []*orgbot.GitHubUser
	if err := s.walkTeamMembers(ctx, orgName, teamID, role, func(u *orgbot.GitHubUser) error {
		users = append(users, u)
		return nil
	}); err != nil {
		return nil, err
	}

	return users, nil
}

// walkTeamMembers walks all the team members within the specified team who have the
// specified role, passing each members to the walk function. If walkFn returns an
// error walking stops and the error is returned.
func (s *service) walkTeamMembers(ctx context.Context, orgName string, teamID orgbot.GitHubTeamID, role orgbot.GitHubTeamRole, walkFn walkTeamMembersFunc) error {
	opts := github.TeamListTeamMembersOptions{
		Role:        string(role),
		ListOptions: github.ListOptions{PerPage: pageSize},
	}

	// Loop until there are no more pages of teams
	for {
		// Call our own version of listTeamMembers to avoid receiving transitive members
		users, r, err := s.listTeamMembers(ctx, int64(teamID), &opts)
		if err != nil {
			return err
		}

		// For each team in the paged response
		for _, u := range users {
			user, err := s.UserByLogin(ctx, orgName, *u.Login)
			if err != nil {
				return err
			}

			if err := walkFn(user); err != nil {
				return err
			}
		}

		// Are we done with paging through the users?
		if r.NextPage == 0 {
			break
		}

		// Not done yet
		opts.Page = r.NextPage
	}

	return nil
}

// CreateTeam implements orgbot.GitHubService.
func (s *service) CreateTeam(ctx context.Context, orgName string, team *orgbot.GitHubTeam) (*orgbot.GitHubTeam, error) {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return nil, err
	}

	newT, _, err := v3.Teams.CreateTeam(ctx, orgName, asNewTeam(team))
	if err != nil {
		return nil, err
	}

	return asDomainTeam(newT), err
}

// UpdateTeam implements orgbot.GitHubService.
func (s *service) UpdateTeam(ctx context.Context, team *orgbot.GitHubTeam) (*orgbot.GitHubTeam, error) {
	_, _, err := s.editTeam(ctx, int64(team.ID), asNewTeam(team))
	if err != nil {
		return nil, err
	}

	return team, nil
}

// DeleteTeam implements orgbot.GitHubService.
func (s *service) DeleteTeam(ctx context.Context, teamID orgbot.GitHubTeamID) error {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return err
	}

	_, err = v3.Teams.DeleteTeam(ctx, int64(teamID))
	return err
}

// AddTeamMembership implements orgbot.GitHubService.
func (s *service) AddTeamMembership(ctx context.Context, orgName string, teamID orgbot.GitHubTeamID, email string, role orgbot.GitHubTeamRole) error {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return err
	}

	opts := github.TeamAddTeamMembershipOptions{Role: string(role)}
	user, err := s.UserByEmail(ctx, orgName, email)
	if err != nil {
		return err
	}

	_, _, err = v3.Teams.AddTeamMembership(ctx, int64(teamID), user.Login, &opts)
	if err != nil {
		return err
	}

	return nil
}

// DeleteTeamMembership implements orgbot.GitHubService.
func (s *service) DeleteTeamMembership(ctx context.Context, orgName string, teamID orgbot.GitHubTeamID, email string, role orgbot.GitHubTeamRole) error {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return err
	}

	user, err := s.UserByEmail(ctx, orgName, email)
	if err != nil {
		return err
	}

	_, err = v3.Teams.RemoveTeamMembership(ctx, int64(teamID), user.Login)
	return err
}

// AddTeamRepoPermission implements orgbot.GithubService
func (s *service) AddTeamRepoPermission(ctx context.Context, orgName string, repoName string, teamID orgbot.GitHubTeamID, permission orgbot.RepoPermission) error {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return err
	}

	_, err = v3.Teams.AddTeamRepo(ctx, int64(teamID), orgName, repoName, &github.TeamAddTeamRepoOptions{Permission: string(permission)})
	return err
}

// DeleteTeamRepoPermission implements orgbot.GithubService
func (s *service) DeleteTeamRepoPermission(ctx context.Context, orgName string, repoName string, teamID orgbot.GitHubTeamID) error {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return err
	}

	_, err = v3.Teams.RemoveTeamRepo(ctx, int64(teamID), orgName, repoName)
	return err
}

// WalkRepos implements orgbot.GithubService
func (s *service) WalkRepos(ctx context.Context, orgName string, walkFn orgbot.WalkReposFunc) error {
	v4, err := s.V4Client(ctx)
	if err != nil {
		return err
	}

	cursor := ""
	opts := map[string]interface{}{
		"org":   githubv4.String(orgName),
		"first": githubv4.Int(pageSize),
	}

	for {
		opts["cursor"] = gitHubV4StringPtr(cursor)

		var q reposQuery
		err := v4.Query(ctx, &q, opts)
		if err != nil {
			return err
		}

		for _, repo := range q.Org.Repositories.Nodes {
			if repo.IsArchived {
				continue // Skip archived repositories
			}

			orgRepo, err := s.asDomainRepo(ctx, orgName, repo.Name, topicNodeToStringArray(repo.RepositoryTopics.Nodes))
			if err != nil {
				return err
			}

			if err := walkFn(orgRepo); err != nil {
				return err
			}
		}

		if !q.Org.Repositories.PageInfo.HasNextPage {
			break
		}
		cursor = q.Org.Repositories.PageInfo.EndCursor
	}

	return nil
}

// WalkRepos implements orgbot.GithubService
func (s *service) WalkReposByTeam(ctx context.Context, orgName string, teamID orgbot.GitHubTeamID, walkFn orgbot.WalkReposFunc) error {
	opts := github.ListOptions{
		PerPage: pageSize,
	}

	for {
		repos, r, err := s.listTeamRepos(ctx, int64(teamID), &opts)
		if err != nil {
			return err
		}

		for _, repo := range repos {
			if *repo.Archived {
				continue // Skip archived repositories
			}

			orgRepo, err := s.asDomainRepo(ctx, orgName, *repo.Name, repo.Topics)
			if err != nil {
				return err
			}

			if err := walkFn(orgRepo); err != nil {
				return err
			}
		}

		if r.NextPage == 0 {
			break
		}
		opts.Page = r.NextPage
	}

	return nil
}

// RepoByName implements orgbot.GithubService
func (s *service) RepoByName(ctx context.Context, orgName, repoName string) (*orgbot.Repo, error) {
	v4, err := s.V4Client(ctx)
	if err != nil {
		return nil, err
	}

	opts := map[string]interface{}{
		"org":      githubv4.String(orgName),
		"first":    githubv4.Int(pageSize),
		"repoName": githubv4.String(repoName),
	}

	var q repoQuery

	if err = v4.Query(ctx, &q, opts); err != nil {
		return nil, err
	}

	orgRepo, err := s.asDomainRepo(ctx, orgName, q.Data.Name, topicNodeToStringArray(q.Data.RepositoryTopics.Nodes))
	if err != nil {
		return nil, err
	}

	return orgRepo, nil
}

// UpdateRepoTopics implements orgbot.GithubService
func (s *service) UpdateRepoTopics(ctx context.Context, orgName, repoName string, topics []string) error {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return err
	}

	_, _, err = v3.Repositories.ReplaceAllTopics(ctx, orgName, repoName, topics)
	if err != nil {
		return err
	}

	return nil
}

// ListAdmins implements orgbot.GitHubService.
func (s *service) ListAdmins(ctx context.Context, orgName string) ([]*orgbot.GitHubUser, error) {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return nil, err
	}

	opts := github.ListMembersOptions{
		Role:        "admin",
		ListOptions: github.ListOptions{PerPage: pageSize},
	}

	var admins []*orgbot.GitHubUser

	for {
		users, r, err := v3.Organizations.ListMembers(ctx, orgName, &opts)
		if err != nil {
			return nil, err
		}

		for _, u := range users {
			admin, err := s.UserByLogin(ctx, orgName, *u.Login)
			if err != nil {
				return nil, err
			}

			admins = append(admins, admin)
		}

		if r.NextPage == 0 {
			break
		}
		opts.Page = r.NextPage
	}

	return admins, nil
}

// UserByEmail implements orgbot.GitHubService.
func (s *service) UserByEmail(ctx context.Context, orgName, email string) (*orgbot.GitHubUser, error) {
	user, err := s.userFromCacheOrUpdate(ctx, orgName, email)
	if err != nil {
		return nil, err
	}

	if user != nil {
		return user, nil
	}

	return nil, &orgbot.GitHubUserNotFoundError{OrgName: orgName, Email: email}
}

// UserByLogin implements orgbot.GitHubService.
func (s *service) UserByLogin(ctx context.Context, orgName, login string) (*orgbot.GitHubUser, error) {
	user, err := s.userFromCacheOrUpdate(ctx, orgName, login)
	if err != nil {
		return nil, err
	}

	if user != nil {
		return user, nil
	}

	return nil, &orgbot.GitHubUserNotFoundError{OrgName: orgName, Login: login}
}

// asDomainRepo converts the specified repository information to an orgbot.Repo.
func (s *service) asDomainRepo(ctx context.Context, orgName, repoName string, topics []string) (*orgbot.Repo, error) {
	v3, err := s.V3Client(ctx)
	if err != nil {
		return nil, err
	}

	// Assume there aren't more than 100 teams to a repo
	teams, r, err := v3.Repositories.ListTeams(ctx, orgName, repoName, &github.ListOptions{
		PerPage: pageSize,
	})
	if err != nil {
		return nil, err
	}

	if r.NextPage != 0 {
		return nil, fmt.Errorf("found more than 100 teams for repo %s", repoName)
	}

	orgRepo := &orgbot.Repo{
		Name:   repoName,
		Topics: topics,
	}

	for _, team := range teams {
		orgRepo.Teams = append(orgRepo.Teams, &orgbot.TeamPermission{
			TeamName:   *team.Name,
			Permission: orgbot.RepoPermission(*team.Permission),
		})
	}

	return orgRepo, nil
}

// topicNodeToStringArray turns an array of topicNodes, returned by a Graphql query, into an array of strings
func topicNodeToStringArray(nodes []topicNode) []string {
	var topics []string
	for _, v := range nodes {
		topics = append(topics, strings.TrimPrefix(v.ResourcePath, "/topics/"))
	}

	return topics
}

// gitHubV4StringPtr is a helper function that githubv4.String pointer to the specified
// string if it is not empty, otherwise nil.
func gitHubV4StringPtr(value string) *githubv4.String {
	if value == "" {
		return nil
	}
	return (*githubv4.String)(&value)
}

// asNewTeam converts the specified orgbot.GitHubTeam to a github.NewTeam.
func asNewTeam(t *orgbot.GitHubTeam) github.NewTeam {
	var parentTeamID *int64
	if t.ParentID != 0 {
		pid := int64(t.ParentID)
		parentTeamID = &pid
	}

	privacy := privacyClosed

	return github.NewTeam{
		Name:         t.Name,
		Description:  &t.Description,
		Privacy:      &privacy,
		ParentTeamID: parentTeamID,
	}
}

// asDomainTeam converts the specified github.Team to an orgbot.GitHubTeam.
func asDomainTeam(t *github.Team) *orgbot.GitHubTeam {
	var id, parentID int64
	if t.ID != nil {
		id = *t.ID
	}
	if t.Parent != nil && t.Parent.ID != nil {
		parentID = *t.Parent.ID
	}

	return &orgbot.GitHubTeam{
		ID:          orgbot.GitHubTeamID(id),
		ParentID:    orgbot.GitHubTeamID(parentID),
		Name:        *t.Name,
		Description: *t.Description,
	}
}

// userFromCacheOrUpdate returns the user in the specified org with the specified ID. The ID can be either
// the user's company email address or their login name. If the org's user set has been cached, the function
// will attempt to return the cached user; if the user set has not been cached the cache is updated first.
// If the user cannot be found in the set of users for the org then nil is returned.
func (s *service) userFromCacheOrUpdate(ctx context.Context, orgName, id string) (*orgbot.GitHubUser, error) {
	s.userCacheMu.Lock()
	defer s.userCacheMu.Unlock()

	// As we're currently reliant on user information produced by another process that monitors SEEK-Jobs
	// we can't support other organisations. When GitHub Apps can access SSO information we can go direct.
	if orgName != supportedOrgName {
		return nil, fmt.Errorf("unsupported GitHub organisation '%s'", orgName)
	}

	// Use the cached user information for the org if we have it and if it was cached within the TTL
	if users, ok := s.userCache[orgName]; ok && s.userCacheTime.Add(userCacheTTL).After(time.Now()) {
		return users[id], nil
	}

	// We need to refresh the cache...
	buf, err := s.s3Client.GetObjectAsString(s.config.GitHubAuditBucket, userInfoS3Key)
	if err != nil {
		return nil, err
	}

	var auditUsers []userInfo
	if err := json.Unmarshal([]byte(buf), &auditUsers); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal GitHub user audit information")
	}

	// Build the cache
	users := map[string]*orgbot.GitHubUser{}
	for _, u := range auditUsers {
		if u.GitHubUser == "" {
			// Ignore users that don't have a GitHub username
			continue
		}

		// Depending on how the user was on-boarded, only one (or none, when on-boarding mistakes
		// have been made) of SCIMUser or SAMLUser will report the user's company email address
		var email string
		if isEmail(u.SCIMUser) {
			email = u.SCIMUser
		} else if isEmail(u.SAMLUser) {
			email = u.SAMLUser
		} else {
			continue
		}

		// Populate the map for users with both an email and a GitHub username
		user := orgbot.GitHubUser{Login: u.GitHubUser, Email: email}
		users[user.Login] = &user
		users[user.Email] = &user
	}

	s.userCache[orgName] = users
	s.userCacheTime = time.Now()

	return users[id], nil
}

// isEmail returns whether the specified string is an email address.
func isEmail(v string) bool {
	return strings.Contains(v, "@")
}
