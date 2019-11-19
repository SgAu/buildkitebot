package cli

import (
	"os"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/SEEK-Jobs/orgbot/pkg/build"
)

// versionTemplate provides a Go template for displaying extended version information.
var versionTemplate = template.Must(template.New("version").Parse(`{{ with . -}}
Version:    {{.Version}}
Go version: {{.GoVersion}}
Git commit: {{.GitCommit}}
Built:      {{.BuildTime}}
OS/Arch:    {{.OperatingSystem}}/{{.Architecture}}
{{ end }}`))

// newVersionCommand returns the "orgctl version" sub-command which prints version information.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Prints version information",
		RunE: func(c *cobra.Command, args []string) error {
			return versionTemplate.Execute(os.Stdout, build.GetInfo())
		},
	}
}
