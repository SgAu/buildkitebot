package orgbot

// Org represents the desired state for an org.
type Org struct {
	Name  string  `json:"name,omitempty" yaml:"name,omitempty"`
	Teams []*Team `json:"teams,omitempty" yaml:"teams,omitempty"`
}

// Team represents the desired state for a team.
type Team struct {
	Name        string   `json:"name,omitempty" yaml:"name,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Previously  []string `json:"previously,omitempty" yaml:"previously,omitempty"`

	// Purposefully disallow maintainers to be specified in the YAML definitions.
	// The domain logic caters for maintainers but this is the switch for allowing
	// them to be specified in the first place. Note that this also prevents them
	// from being output by the DumpOrg.
	Maintainers     []string `json:"-" yaml:"-"`
	Members         []string `json:"members,omitempty" yaml:"members,omitempty"`
	RestrictMembers []string `json:"restrictMembers,omitempty" yaml:"restrictMembers,omitempty"`
	Children        []*Team  `json:"teams,omitempty" yaml:"teams,omitempty"`
}

// RepoPermission is the type used for team permissions on repos
type RepoPermission string

const (
	RepoPermissionRead  RepoPermission = "pull"  // Team read permission on a repo
	RepoPermissionWrite RepoPermission = "push"  // Team write permission on a repo
	RepoPermissionAdmin RepoPermission = "admin" // Team admin permission on a repo
)

// Repo represents a GitHub repository.
type Repo struct {
	Name   string            `json:"name,omitempty" yaml:"name,omitempty"`
	Topics []string          `json:"topics,omitempty" yaml:"topics,omitempty"`
	Teams  []*TeamPermission `json:"teams,omitempty" yaml:"teams,omitempty"`
}

// TeamPermission represents a team's access permissions to a repository.
type TeamPermission struct {
	TeamName   string
	Permission RepoPermission
}
