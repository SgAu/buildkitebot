package orgbot

import (
	"regexp"
	"sort"
	"strings"
)

// SortOrg sorts the teams and team members within the specified org
// to be in alphanumeric order to assist with comparisons.
func SortOrg(org *Org) {
	sort.Slice(org.Teams, func(i, j int) bool {
		return org.Teams[i].Name < org.Teams[j].Name
	})

	for _, v := range org.Teams {
		SortTeam(v)
	}
}

// SortTeam sorts the members, maintainers and child teams of the specified team.
func SortTeam(team *Team) {
	sort.Strings(team.Maintainers)
	sort.Strings(team.Members)

	// Sort children by name for consistency
	if team.Children != nil {
		sort.Slice(team.Children, func(i, j int) bool {
			return team.Children[i].Name < team.Children[j].Name
		})

		for _, v := range team.Children {
			SortTeam(v)
		}
	}
}

// findGitHubTeamFromDesired returns the GitHubTeam in the slice whose name or previous
// names match the name of the desired team.
func findGitHubTeamFromDesired(teams []*GitHubTeam, want *Team) *GitHubTeam {
	for _, name := range append([]string{want.Name}, want.Previously...) {
		for _, t := range teams {
			if name == t.Name {
				return t
			}
		}
	}
	return nil
}

// normaliseName normalises the specified name by removing non-alphanumeric characters,
// replacing spaces and underscores with hyphens, etc.
func normaliseName(name string) string {
	name = strings.ToLower(name)
	// Replace spaces with dashes
	noSpacesRegexp := regexp.MustCompile(`([ _]+)`)
	s := noSpacesRegexp.ReplaceAllString(name, "-")

	// Only allow good characters, and drop the rest
	goodCharRegexp := regexp.MustCompile(`([^abcdefghijklmnopqrstuvwxyz0123456789\-_])`)
	s = goodCharRegexp.ReplaceAllString(s, "")

	// Don't have multiple hyphens
	multipleHyphensRegexp := regexp.MustCompile(`([-]{2,})`)
	s = multipleHyphensRegexp.ReplaceAllString(s, "-")

	// Don't start or end the name with a hyphen or underscore
	startAndEndRegexp := regexp.MustCompile(`(^[\-_]+)|([\-_]+)$`)
	s = startAndEndRegexp.ReplaceAllString(s, "")

	return s
}
