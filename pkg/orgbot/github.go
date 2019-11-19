package orgbot

import (
	"context"
	"fmt"
	"sync"
)

// GitHubTeamRole is the type used to describe a user's role within a team
type GitHubTeamRole string

// GitHubTeamID is the type used for team IDs
type GitHubTeamID int64

const (
	RoleMaintainer GitHubTeamRole = "maintainer" // Team maintainer role
	RoleMember     GitHubTeamRole = "member"     // Team regular member role

	// Special ID used by readOnlyService when stubbing team creation
	phonyTeamID GitHubTeamID = -1
)

// WalkReposFunc is the type of the function called for each repo in the GitHub org
// by the WalkRepos function. If the function returns an error walking stops.
type WalkReposFunc func(r *Repo) error

// GitHubService provides the domain interface for all GitHub interactions.
type GitHubService interface {
	// ListTeams returns the all non-secret teams in the specified org.
	ListTeams(ctx context.Context, orgName string) ([]*GitHubTeam, error)

	// ListTeamMembers returns the members in the specified team and role.
	ListTeamMembers(ctx context.Context, orgName string, teamID GitHubTeamID, role GitHubTeamRole) ([]*GitHubUser, error)

	// CreateTeam creates the specified team and returns the result of the creation.
	CreateTeam(ctx context.Context, orgName string, team *GitHubTeam) (*GitHubTeam, error)

	// UpdateTeam updates the specified team and returns the result of the update.
	UpdateTeam(ctx context.Context, team *GitHubTeam) (*GitHubTeam, error)

	// DeleteTeam deletes the team with the specified ID.
	DeleteTeam(ctx context.Context, teamID GitHubTeamID) error

	// AddTeamMembership adds the specified user to the specified team with the specified role.
	AddTeamMembership(ctx context.Context, orgName string, teamID GitHubTeamID, email string, role GitHubTeamRole) error

	// DeleteTeamMembership deletes the specified user from the specified role within the specified team.
	DeleteTeamMembership(ctx context.Context, orgName string, teamID GitHubTeamID, email string, role GitHubTeamRole) error

	// AddTeamRepoPermission adds the specified team permissions to the specified repository.
	AddTeamRepoPermission(ctx context.Context, orgName string, repoName string, teamID GitHubTeamID, permission RepoPermission) error

	// DeleteTeamRepoPermission removes the specified team's permissions from the specified repository.
	DeleteTeamRepoPermission(ctx context.Context, orgName string, repoName string, teamID GitHubTeamID) error

	// WalkRepos walks over all repos in the specified org, passing each to the walk function.
	WalkRepos(ctx context.Context, orgName string, walkFn WalkReposFunc) error

	// WalkReposByTeam walks over all repos that are directly accessible by the specified team,
	// passing each to the walk function.
	WalkReposByTeam(ctx context.Context, orgName string, teamID GitHubTeamID, walkFn WalkReposFunc) error

	// RepoByName returns the repo with name repoName
	RepoByName(ctx context.Context, orgName, repoName string) (*Repo, error)

	// UpdateRepoTopics updates the topics for the given repo.
	UpdateRepoTopics(ctx context.Context, orgName, repoName string, topics []string) error

	// ListAdmins returns the admins for the given org.
	ListAdmins(ctx context.Context, orgName string) ([]*GitHubUser, error)

	// UserByEmail returns the user in the specified org with the specified company email address
	// or GitHubUserNotFoundError if the user is not a member of the org.
	UserByEmail(ctx context.Context, orgName, email string) (*GitHubUser, error)

	// UserByLogin returns the user in the specified org with the specified login username
	// or GitHubUserNotFoundError if the user is not a member of the org.
	UserByLogin(ctx context.Context, orgName, login string) (*GitHubUser, error)
}

// GitHubTeam represents a team within GitHub.
type GitHubTeam struct {
	ID          GitHubTeamID // ID of this team (0 for teams that don't exist yet)
	ParentID    GitHubTeamID // ID of this team's parent (0 for top-level team)
	Name        string       // Name of this team
	Description string       // Description of this team
}

// GitHubUser represents a user in GitHub.
type GitHubUser struct {
	Login string // Username
	Email string // Company email address
}

// GitHubServiceWithStats extends the GitHubService interface to provide stats gathering functionality.
type GitHubServiceWithStats interface {
	GitHubService

	// ZeroStats resets the stats counters to zero.
	ZeroStats()

	// Stats returns the accumulated stats.
	Stats() GitHubStats
}

// GitHubStats encapsulates GitHub API operation statistics.
type GitHubStats struct {
	TeamsCreated           int // Number of teams created
	TeamsUpdated           int // Number of teams updated
	TeamsDeleted           int // Number of teams deleted
	TeamMembershipsAdded   int // Number of team memberships added
	TeamMembershipsDeleted int // Number of team memberships deleted
}

// GitHubUserNotFoundError is the type of error returned when no SSO information can be found for a user.
type GitHubUserNotFoundError struct {
	OrgName string
	Email   string
	Login   string
}

// Error implements error.
func (e *GitHubUserNotFoundError) Error() string {
	id := e.Email
	if e.Email == "" {
		id = e.Login
	}
	return fmt.Sprintf("could not find SSO information for user '%s' in org '%s'", id, e.OrgName)
}

// readOnlyService provides a GitHubService adapter that only allows read operations to be submitted
// to its delegate. All write operations result in a no-op.
type readOnlyService struct {
	delegate GitHubService
}

// NewReadOnlyGitHubService returns a configured read-only GitHub service implementation.
func NewReadOnlyGitHubService(delegate GitHubService) GitHubService {
	return &readOnlyService{delegate: delegate}
}

// ListTeams implements orgbot.GitHubService.
func (s *readOnlyService) ListTeams(ctx context.Context, orgName string) ([]*GitHubTeam, error) {
	return s.delegate.ListTeams(ctx, orgName)
}

// ListTeamMembers implements orgbot.GitHubService.
func (s *readOnlyService) ListTeamMembers(ctx context.Context, orgName string, teamID GitHubTeamID, role GitHubTeamRole) ([]*GitHubUser, error) {
	// The phonyTeamID is used by CreateTeam below and recognised here so that we don't attempt to list
	// team members for a team that doesn't actually exist.
	if teamID == phonyTeamID {
		return nil, nil
	}

	return s.delegate.ListTeamMembers(ctx, orgName, teamID, role)
}

// CreateTeam implements orgbot.GitHubService.
func (s *readOnlyService) CreateTeam(ctx context.Context, orgName string, team *GitHubTeam) (*GitHubTeam, error) {
	// Return a copy of the specified team with a random ID assigned
	newT := *team
	newT.ID = phonyTeamID
	return &newT, nil
}

// UpdateTeam implements orgbot.GitHubService.
func (s *readOnlyService) UpdateTeam(ctx context.Context, team *GitHubTeam) (*GitHubTeam, error) {
	return team, nil
}

// DeleteTeam implements orgbot.GitHubService.
func (s *readOnlyService) DeleteTeam(ctx context.Context, teamID GitHubTeamID) error {
	return nil
}

// AddTeamMembership implements orgbot.GitHubService.
func (s *readOnlyService) AddTeamMembership(ctx context.Context, orgName string, teamID GitHubTeamID, email string, role GitHubTeamRole) error {
	return nil
}

// DeleteTeamMembership implements orgbot.GitHubService.
func (s *readOnlyService) DeleteTeamMembership(ctx context.Context, orgName string, teamID GitHubTeamID, email string, role GitHubTeamRole) error {
	return nil
}

// AddTeamRepoPermission implements orgbot.GitHubService.
func (s *readOnlyService) AddTeamRepoPermission(ctx context.Context, orgName string, repoName string, teamID GitHubTeamID, permission RepoPermission) error {
	return nil
}

// DeleteTeamRepoPermission implements orgbot.GitHubService.
func (s *readOnlyService) DeleteTeamRepoPermission(ctx context.Context, orgName string, repoName string, teamID GitHubTeamID) error {
	return nil
}

// WalkRepos implements orgbot.GitHubService.
func (s *readOnlyService) WalkRepos(ctx context.Context, orgName string, walkFn WalkReposFunc) error {
	return s.delegate.WalkRepos(ctx, orgName, walkFn)
}

// WalkReposByTeam implements orgbot.GitHubService.
func (s *readOnlyService) WalkReposByTeam(ctx context.Context, orgName string, teamID GitHubTeamID, walkFn WalkReposFunc) error {
	return s.delegate.WalkReposByTeam(ctx, orgName, teamID, walkFn)
}

// RepoByName implments orgbot.GithubService
func (s *readOnlyService) RepoByName(ctx context.Context, orgName, repoName string) (*Repo, error) {
	return s.delegate.RepoByName(ctx, orgName, repoName)
}

// UpdateRepoTopics implements orgbot.GitHubService
func (s *readOnlyService) UpdateRepoTopics(ctx context.Context, orgName, repoName string, topics []string) error {
	return nil
}

// ListAdmins implements orgbot.GitHubService.
func (s *readOnlyService) ListAdmins(ctx context.Context, orgName string) ([]*GitHubUser, error) {
	return s.delegate.ListAdmins(ctx, orgName)
}

// UserByEmail implements orgbot.GitHubService.
func (s *readOnlyService) UserByEmail(ctx context.Context, orgName, email string) (*GitHubUser, error) {
	return s.delegate.UserByEmail(ctx, orgName, email)
}

// UserByLogin implements orgbot.GitHubService.
func (s *readOnlyService) UserByLogin(ctx context.Context, orgName, login string) (*GitHubUser, error) {
	return s.delegate.UserByLogin(ctx, orgName, login)
}

// statsService provides a GitHubService adapter that gathers statistics about GitHub
// API operations after calling its delegate.
type statsService struct {
	delegate GitHubService
	stats    GitHubStats

	sync.Mutex
}

// NewStatsGitHubService returns a configured GitHubServiceWithStats implementation.
func NewStatsGitHubService(delegate GitHubService) GitHubServiceWithStats {
	return &statsService{delegate: delegate, stats: GitHubStats{}}
}

// ListTeams implements orgbot.GitHubService.
func (s *statsService) ListTeams(ctx context.Context, orgName string) ([]*GitHubTeam, error) {
	return s.delegate.ListTeams(ctx, orgName)
}

// ListTeamMembers implements orgbot.GitHubService.
func (s *statsService) ListTeamMembers(ctx context.Context, orgName string, teamID GitHubTeamID, role GitHubTeamRole) ([]*GitHubUser, error) {
	return s.delegate.ListTeamMembers(ctx, orgName, teamID, role)
}

// CreateTeam implements orgbot.GitHubService.
func (s *statsService) CreateTeam(ctx context.Context, orgName string, team *GitHubTeam) (*GitHubTeam, error) {
	team, err := s.delegate.CreateTeam(ctx, orgName, team)
	if err != nil {
		return nil, err
	}

	s.Lock()
	defer s.Unlock()

	s.stats.TeamsCreated++
	return team, nil
}

// UpdateTeam implements orgbot.GitHubService.
func (s *statsService) UpdateTeam(ctx context.Context, team *GitHubTeam) (*GitHubTeam, error) {
	team, err := s.delegate.UpdateTeam(ctx, team)
	if err != nil {
		return nil, err
	}

	s.Lock()
	defer s.Unlock()

	s.stats.TeamsUpdated++
	return team, nil
}

// DeleteTeam implements orgbot.GitHubService.
func (s *statsService) DeleteTeam(ctx context.Context, teamID GitHubTeamID) error {
	if err := s.delegate.DeleteTeam(ctx, teamID); err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	s.stats.TeamsDeleted++
	return nil
}

// AddTeamMembership implements orgbot.GitHubService.
func (s *statsService) AddTeamMembership(ctx context.Context, orgName string, teamID GitHubTeamID, email string, role GitHubTeamRole) error {
	if err := s.delegate.AddTeamMembership(ctx, orgName, teamID, email, role); err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	s.stats.TeamMembershipsAdded++
	return nil
}

// DeleteTeamMembership implements orgbot.GitHubService.
func (s *statsService) DeleteTeamMembership(ctx context.Context, orgName string, teamID GitHubTeamID, email string, role GitHubTeamRole) error {
	if err := s.delegate.DeleteTeamMembership(ctx, orgName, teamID, email, role); err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	s.stats.TeamMembershipsDeleted++
	return nil
}

// AddTeamRepoPermission implements orgbot.GitHubService.
func (s *statsService) AddTeamRepoPermission(ctx context.Context, orgName string, repoName string, teamID GitHubTeamID, permission RepoPermission) error {
	return s.delegate.AddTeamRepoPermission(ctx, orgName, repoName, teamID, permission)
}

// DeleteTeamRepoPermission implements orgbot.GitHubService.
func (s *statsService) DeleteTeamRepoPermission(ctx context.Context, orgName string, repoName string, teamID GitHubTeamID) error {
	return s.delegate.DeleteTeamRepoPermission(ctx, orgName, repoName, teamID)
}

// WalkRepos implements orgbot.GitHubService.
func (s *statsService) WalkRepos(ctx context.Context, orgName string, walkFn WalkReposFunc) error {
	return s.delegate.WalkRepos(ctx, orgName, walkFn)
}

// WalkReposByTeam implements orgbot.GitHubService.
func (s *statsService) WalkReposByTeam(ctx context.Context, orgName string, teamID GitHubTeamID, walkFn WalkReposFunc) error {
	return s.delegate.WalkReposByTeam(ctx, orgName, teamID, walkFn)
}

// RepoByName implments orgbot.GithubService
func (s *statsService) RepoByName(ctx context.Context, orgName, repoName string) (*Repo, error) {
	return s.delegate.RepoByName(ctx, orgName, repoName)
}

// UpdateRepoTopics implements orgbot.GithubService
func (s *statsService) UpdateRepoTopics(ctx context.Context, orgName, repoName string, topics []string) error {
	return s.delegate.UpdateRepoTopics(ctx, orgName, repoName, topics)
}

// ListAdmins implements orgbot.GitHubService.
func (s *statsService) ListAdmins(ctx context.Context, orgName string) ([]*GitHubUser, error) {
	return s.delegate.ListAdmins(ctx, orgName)
}

// UserByEmail implements orgbot.GitHubService.
func (s *statsService) UserByEmail(ctx context.Context, orgName, email string) (*GitHubUser, error) {
	return s.delegate.UserByEmail(ctx, orgName, email)
}

// UserByLogin implements orgbot.GitHubService.
func (s *statsService) UserByLogin(ctx context.Context, orgName, login string) (*GitHubUser, error) {
	return s.delegate.UserByLogin(ctx, orgName, login)
}

// ZeroStats implements orgbot.GitHubServiceWithStats.
func (s *statsService) ZeroStats() {
	s.stats = GitHubStats{}
}

// Stats implements orgbot.GitHubServiceWithStats.
func (s *statsService) Stats() GitHubStats {
	return s.stats
}
