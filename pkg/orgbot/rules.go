package orgbot

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	set "github.com/deckarep/golang-set"
)

const (
	docURL = "https://github.com/SEEK-Jobs/org/blob/master/README.md"

	// maxTeamLength specifies the maximum allowable length of a team name. This should
	// be calculated based on the maximum allowable topic length but we currently allow
	// longer than usual team names and truncate the admin topics to reduce go-live friction.
	// maxTeamNameLength = maxTopicLength - len(adminTopicPrefix)
	maxTeamNameLength = 35
)

// RuleEngine provides an extensible interface for running business rules against
// a potential org structure.
type RuleEngine interface {
	Run(ctx context.Context, org *Org) error
	Add(r Rule)
}

// Rule provides an interface for encapsulating a single business rule that is
// run against a potential org structure.
type Rule interface {
	Run(ctx context.Context, org *Org) error
}

// RuleError is the interface implemented by all errors returned in response
// to rule violations.
type RuleError interface {
	Description() string
	ConstraintViolations() string
	Link() string
	Error() string
}

// CompositeRuleError is the error returned by a RuleEngine when one or more
// rule violations have occurred; it is used to aggregate multiple violations.
type CompositeRuleError struct {
	OrgName string
	Errors  []RuleError
}

// Error implements error.
func (e *CompositeRuleError) Error() string {
	var msgs []string
	for _, re := range e.Errors {
		msg := fmt.Sprintf("Rule: %s /// Violations: %s /// See: %s",
			re.Description(), re.ConstraintViolations(), re.Link())
		msgs = append(msgs, msg)
	}
	return fmt.Sprintf(
		"the following rules are violated by org '%s': %s",
		e.OrgName, strings.Join(msgs, " ///// "))
}

// ruleEngine provides the implementation of RuleEngine.
type ruleEngine struct {
	rules []Rule
}

// NewRuleEngine returns an instance of RuleEngine with all business rules added.
func NewRuleEngine(gitHubService GitHubService) RuleEngine {
	ruleEngine := &ruleEngine{}
	ruleEngine.Add(&unknownUsersRule{gitHubService: gitHubService})
	ruleEngine.Add(&teamNamesUniqueRule{})
	ruleEngine.Add(&usersUniqueWithinTeamRule{})
	ruleEngine.Add(&teamNameLengthRule{maxLength: maxTeamNameLength})
	ruleEngine.Add(&activeTeamDeletionsRule{gitHubService: gitHubService})
	ruleEngine.Add(&crossOrgMembershipsRule{})

	return ruleEngine
}

// Run implements RuleEngine.
func (o *ruleEngine) Run(ctx context.Context, org *Org) error {
	var ruleErrors []RuleError

	for _, rule := range o.rules {
		if err := rule.Run(ctx, org); err != nil {
			// If the error is a RuleError collect it, otherwise return immediately
			ruleError, ok := err.(RuleError)
			if !ok {
				return err
			}
			ruleErrors = append(ruleErrors, ruleError)
		}
	}

	if len(ruleErrors) > 0 {
		return &CompositeRuleError{OrgName: org.Name, Errors: ruleErrors}
	}

	return nil
}

// Add implements RuleEngine.
func (o *ruleEngine) Add(r Rule) {
	o.rules = append(o.rules, r)
}

// UnknownUsersError is the error returned when one or more users are not part of the GitHub org.
type UnknownUsersError struct {
	// Violations is a map of team names to user email addresses that violate the constraint.
	Violations map[string][]string
}

// Description implements RuleError.
func (e *UnknownUsersError) Description() string {
	return "Users must be members of the GitHub org"
}

// ConstraintViolations implements RuleError.
func (e *UnknownUsersError) ConstraintViolations() string {
	var msgs []string
	for t, emails := range e.Violations {
		msgs = append(msgs, fmt.Sprintf("'%s': %s", t, quoteJoin(emails)))
	}
	return strings.Join(msgs, "; ")
}

// Link implements RuleError.
func (e *UnknownUsersError) Link() string {
	return docURL + "#unknown-users"
}

// Error implements RuleError.
func (e *UnknownUsersError) Error() string {
	return fmt.Sprintf("%s: %s", e.Description(), e.ConstraintViolations())
}

// unknownUsersRule provides an implementation of Rule that verifies that all
// teams within an org have unique names.
type unknownUsersRule struct {
	gitHubService GitHubService
}

// Run implements Rule.
func (r *unknownUsersRule) Run(ctx context.Context, org *Org) error {
	violations := map[string][]string{}

	// run recursively descends into the team hierarchy testing for valid members within each team
	var run func([]*Team) error
	run = func(teams []*Team) error {
		for _, t := range teams {
			for _, email := range append(t.Maintainers, t.Members...) {
				_, err := r.gitHubService.UserByEmail(ctx, org.Name, email)
				if err != nil {
					if _, ok := err.(*GitHubUserNotFoundError); !ok {
						return err
					}
					violations[t.Name] = append(violations[t.Name], email)
				}
			}
			if err := run(t.Children); err != nil {
				return err
			}
		}
		return nil
	}

	if err := run(org.Teams); err != nil {
		return err
	}

	if len(violations) > 0 {
		return &UnknownUsersError{Violations: violations}
	}

	return nil
}

// TeamNamesUniqueError is the error returned when one or more teams have the same name.
type TeamNamesUniqueError struct {
	// Violations is an array of team names that violate the constraint.
	Violations []string
}

// Description implements RuleError.
func (e *TeamNamesUniqueError) Description() string {
	return "Team names must be unique within an organisation"
}

// ConstraintViolations implements RuleError.
func (e *TeamNamesUniqueError) ConstraintViolations() string {
	return quoteJoin(e.Violations)
}

// Description implements RuleError.
func (e *TeamNamesUniqueError) Link() string {
	return docURL + "#duplicate-team-names"
}

// Error implements RuleError.
func (e *TeamNamesUniqueError) Error() string {
	return fmt.Sprintf("%s: %s", e.Description(), e.ConstraintViolations())
}

// teamNamesUniqueRule provides an implementation of Rule that verifies that all
// teams within an org have unique names.
type teamNamesUniqueRule struct{}

// Run implements Rule.
func (r *teamNamesUniqueRule) Run(ctx context.Context, org *Org) error {
	var violations []string
	unique := set.NewSet()

	// run recursively descends into the team hierarchy testing for team name uniqueness
	var run func([]*Team)
	run = func(teams []*Team) {
		for _, t := range teams {
			if !unique.Add(t.Name) {
				violations = append(violations, t.Name)
			}
			run(t.Children)
		}
	}

	run(org.Teams)

	if len(violations) > 0 {
		return &TeamNamesUniqueError{Violations: violations}
	}

	return nil
}

// UsersUniqueWithinTeamError is the error returned when users within a team are not unique.
type UsersUniqueWithinTeamError struct {
	// Violations is a map of team names to user email addresses within those teams that violate the constraint.
	Violations map[string][]string
}

// Description implements RuleError.
func (e *UsersUniqueWithinTeamError) Description() string {
	return "Users must not be repeated within a team"
}

// ConstraintViolations implements RuleError.
func (e *UsersUniqueWithinTeamError) ConstraintViolations() string {
	var msgs []string
	for t, emails := range e.Violations {
		msgs = append(msgs, fmt.Sprintf("'%s': %s", t, quoteJoin(emails)))
	}
	return strings.Join(msgs, "; ")
}

// Description implements RuleError.
func (e *UsersUniqueWithinTeamError) Link() string {
	return docURL + "#duplicate-team-members"
}

// Error implements RuleError.
func (e *UsersUniqueWithinTeamError) Error() string {
	return fmt.Sprintf("%s: %s", e.Description(), e.ConstraintViolations())
}

// usersUniqueWithinTeamRule provides an implementation of Rule that verifies that
// users within a team are not repeated either within a role or across roles.
type usersUniqueWithinTeamRule struct{}

// Run implements Rule.
func (r *usersUniqueWithinTeamRule) Run(ctx context.Context, org *Org) error {
	violations := map[string][]string{}

	// run recursively descends into the team hierarchy testing for member uniqueness within each team
	var run func([]*Team)
	run = func(teams []*Team) {
		for _, t := range teams {
			unique := set.NewSet()
			for _, email := range append(t.Maintainers, t.Members...) {
				if !unique.Add(email) {
					violations[t.Name] = append(violations[t.Name], email)
				}
			}
			run(t.Children)
		}
	}

	run(org.Teams)

	if len(violations) > 0 {
		return &UsersUniqueWithinTeamError{Violations: violations}
	}

	return nil
}

// CrossOrgMembershipsError is the error returned when one or more users have been added
// to teams to which they are not allowed to be members due to the business they belong to.
type CrossOrgMembershipsError struct {
	// Violations is a map of team name to an array of email addresses of users within those
	// teams who are in violation of the constraint.
	Violations map[string][]string
}

// Description implements RuleError.
func (e *CrossOrgMembershipsError) Description() string {
	return "Team membership violations were detected"
}

// ConstraintViolations implements RuleError.
func (e *CrossOrgMembershipsError) ConstraintViolations() string {
	var msgs []string
	for t, emails := range e.Violations {
		msgs = append(msgs, fmt.Sprintf("'%s': %s", t, quoteJoin(emails)))
	}

	return strings.Join(msgs, "; ")
}

// Description implements RuleError.
func (e *CrossOrgMembershipsError) Link() string {
	return docURL + "#cross-organisation-memberships"
}

// Error implements RuleError.
func (e *CrossOrgMembershipsError) Error() string {
	return fmt.Sprintf("%s: %s", e.Description(), e.ConstraintViolations())
}

// crossOrgMembershipsRule provides an implementation of Rule that verifies that users
// are only members of teams that are allowed based on the domain of their company email
// address. We maintain separate team hierarchies for homogeneous and non-homogeneous user sets.
type crossOrgMembershipsRule struct {
}

// Run implements Rule.
func (r *crossOrgMembershipsRule) Run(ctx context.Context, org *Org) error {
	violations := map[string][]string{}

	// run recursively descends into the team hierarchy testing that team members are valid
	// based on their company email address. The valid gitHubUserEmails for a team cascade downwards;
	// i.e., if ".*@foobar.com" is valid for team X then they are also valid for the children of
	// team X. Put another way, to be allowed to join a team, you must also be allowed to be a
	// member of the team's parent, and it's parent, and so on. Top-level teams must specify at least
	// one restrictMembers element to be allowed to have any members; non-top-level teams receive
	// a '.*' restrictMembers element if they do not specify any themselves. This has the effect
	// of forcing top-level teams to specify some sort of restriction and then descendant teams
	// inheriting that restriction unless they wish to be more restrictive.
	var run func([]*Team, [][]string) error
	run = func(teams []*Team, restrictMembersStack [][]string) error {
		for _, t := range teams {
			restrictMembers := t.RestrictMembers
			// If this is a non-top-level team and no restrictions were specified, allow all -
			// this has the effect of inheriting the parent's restrictions
			if restrictMembersStack != nil && len(t.RestrictMembers) == 0 {
				restrictMembers = []string{".*"}
			}

			// Prepend the current team's membership restrictions to the end of the stack. We'll traverse
			// the stack from start to finish, ensuring that team members are allowed by each block.
			childRestrictMembersStack := append(restrictMembersStack, restrictMembers)

			// Verify that all team members' email addresses are whitelisted
			for _, email := range append(t.Maintainers, t.Members...) {

				// For a membership to be considered valid, the user must be considered an allowed
				// member of all ancestor teams, as well as the team the member is listed on.
				for _, restrictMembers := range childRestrictMembersStack {

					// A member is considered valid if it matches at least one pattern specified by the team.
					matched := false
					for _, allowedPattern := range restrictMembers {
						allowedRegex, err := regexp.Compile(allowedPattern)
						if err != nil {
							return err
						}

						if allowedRegex.MatchString(email) {
							matched = true
							break
						}
					}

					// If a match was not found for the user append a violation
					if !matched {
						violations[t.Name] = append(violations[t.Name], email)
					}
				}
			}

			// Recurse
			if err := run(t.Children, childRestrictMembersStack); err != nil {
				return err
			}
		}

		return nil
	}

	if err := run(org.Teams, nil); err != nil {
		return err
	}

	if len(violations) > 0 {
		return &CrossOrgMembershipsError{Violations: violations}
	}

	return nil
}

// ActiveTeamDeletionsError is the type of error returned by an apply operation when a
// team deletion has been attempted but the team is still listed on one or more repositories.
type ActiveTeamDeletionsError struct {
	// Violations is a map of team name to repository names that in violation of the constraint.
	Violations map[string][]string
}

// Description implements RuleError.
func (e *ActiveTeamDeletionsError) Description() string {
	return fmt.Sprintf("Teams cannot be deleted if they are listed on one or more repositories")
}

// ConstraintViolations implements RuleError.
func (e *ActiveTeamDeletionsError) ConstraintViolations() string {
	var msgs []string
	for t, repos := range e.Violations {
		msgs = append(msgs, fmt.Sprintf("'%s': %s", t, quoteJoin(repos)))
	}

	return strings.Join(msgs, "; ")
}

// Link implements RuleError.
func (e *ActiveTeamDeletionsError) Link() string {
	return docURL + "#active-team-deletions"
}

// Error implements RuleError.
func (e *ActiveTeamDeletionsError) Error() string {
	return fmt.Sprintf("%s: %s", e.Description(), e.ConstraintViolations())
}

// activeTeamDeletionsRule provides an implementation of Rule that verifies that the
// total number of attempted team deletions is under the configured threshold.
type activeTeamDeletionsRule struct {
	gitHubService GitHubService
}

// Run implements Rule.
func (r *activeTeamDeletionsRule) Run(ctx context.Context, org *Org) error {
	// What teams currently exist?
	haveTeams, err := r.gitHubService.ListTeams(ctx, org.Name)
	if err != nil {
		return err
	}

	// Arrange the existing teams into a map keyed by team ID
	haveTeamsByID := map[GitHubTeamID]*GitHubTeam{}
	for _, have := range haveTeams {
		haveTeamsByID[have.ID] = have
	}

	// process recursively processes the specified desired teams removing teams from
	// haveTeamsByID map as we see them
	var process func([]*Team)
	process = func(teams []*Team) {
		for _, t := range teams {
			if have := findGitHubTeamFromDesired(haveTeams, t); have != nil {
				delete(haveTeamsByID, have.ID)
			}
			process(t.Children)
		}
	}
	process(org.Teams)

	// The teams left in the haveTeamsByID map have been requested to be deleted. Loop over them
	// and check that they are not listed on any repos; if they are gather the information in violations.
	violations := map[string][]string{}
	for _, have := range haveTeamsByID {
		repoNames, err := listTeamRepoNames(ctx, r.gitHubService, org.Name, have.ID)
		if err != nil {
			return err
		}

		if len(repoNames) > 0 {
			violations[have.Name] = repoNames
		}
	}

	if len(violations) > 0 {
		return &ActiveTeamDeletionsError{Violations: violations}
	}

	return nil

}

// listTeamRepoNames returns a slice of all repository names directly accessible to the specified team.
func listTeamRepoNames(ctx context.Context, gitHubService GitHubService, orgName string, teamID GitHubTeamID) ([]string, error) {
	var repoNames []string
	if err := gitHubService.WalkReposByTeam(ctx, orgName, teamID, func(r *Repo) error {
		repoNames = append(repoNames, r.Name)
		return nil
	}); err != nil {
		return nil, err
	}

	return repoNames, nil
}

// quoteJoin returns a each string quoted and joined with a comma.
func quoteJoin(ss []string) string {
	var quoted []string
	for _, s := range ss {
		quoted = append(quoted, fmt.Sprintf("'%s'", s))
	}

	return strings.Join(quoted, ", ")
}

// TeamNameLengthError is the error returned when a team's name is too long.
type TeamNameLengthError struct {
	// Violations is a map of team names that violate the constraint.
	Violations []string
	// MaxNameLength is the maximum length a team name can be
	MaxNameLength int
}

// Description implements RuleError.
func (e *TeamNameLengthError) Description() string {
	return fmt.Sprintf("Team names must not exceed %d characters", e.MaxNameLength)
}

// ConstraintViolations implements RuleError.
func (e *TeamNameLengthError) ConstraintViolations() string {
	return quoteJoin(e.Violations)
}

// Link implements RuleError.
func (e *TeamNameLengthError) Link() string {
	return docURL + "#team-name-length"
}

// Error implements RuleError.
func (e *TeamNameLengthError) Error() string {
	return fmt.Sprintf("%s: %s", e.Description(), e.ConstraintViolations())
}

// teamNameLengthRule provides an implementation of Rule that verifies that all
// teams within an org have names that are short enough to be a topic without
// being truncated.
type teamNameLengthRule struct {
	maxLength int
}

// Run implements Rule.
func (r *teamNameLengthRule) Run(ctx context.Context, org *Org) error {
	var violations []string

	// run recursively descends into the team hierarchy testing for a valid name for each team
	var run func([]*Team)
	run = func(teams []*Team) {
		for _, t := range teams {
			if len(t.Name) > r.maxLength {
				violations = append(violations, t.Name)
			}
			run(t.Children)
		}
		return
	}

	run(org.Teams)

	if len(violations) > 0 {
		return &TeamNameLengthError{Violations: violations, MaxNameLength: r.maxLength}
	}

	return nil
}
