package orgbot

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	orgFile  = "org.yaml"  // Control file that describes an org
	teamFile = "team.yaml" // Control file that describes a team within an org
)

var (
	unrecognisedFileReason = fmt.Sprintf("only %s and %s files are supported", orgFile, teamFile)
	multipleOrgFilesReason = fmt.Sprintf("multiple %s files detected", orgFile)
	noSiblingFilesReason   = fmt.Sprintf("%s and %s files can't be siblings", orgFile, teamFile)
)

// MissingControlFileError indicates the absence of a control file in a directory where one is expected.
type MissingControlFileError struct {
	Dir  string // Directory where a control file was expected but not found
	File string // Name of the expected control file
}

// Error implements error.
func (e *MissingControlFileError) Error() string {
	return fmt.Sprintf("expected '%s' in directory '%s' but none found", e.File, e.Dir)
}

// UnexpectedFileError indicates the presence of an unexpected file within the organisational structure.
type UnexpectedFileError struct {
	Path   string // Unexpected file path
	Reason string // Reason for error
}

// Error implements error.
func (e *UnexpectedFileError) Error() string {
	return fmt.Sprintf("unexpected file '%s': %s", e.Path, e.Reason)
}

// InvalidTeamDirNameError indicates a badly named directory
type InvalidTeamDirNameError struct {
	Dir      string // Badly named directory path
	ValidDir string // Correctly named directory path
	TeamName string // Name of the team
}

// Error implements error.
func (e *InvalidTeamDirNameError) Error() string {
	return fmt.Sprintf("team directory '%s' should be named '%s' to be consistent with normalisation of team name '%s'",
		e.Dir, e.ValidDir, e.TeamName)
}

// MergeOrg descends into the specified directory, reading the org.yaml and team.yaml
// files, and constructs and returns an Org.
func MergeOrg(codec Codec, dir string) (*Org, error) {
	// Read the top level org.yaml file which we expect to exist in the specified directory
	orgDir := dir
	org, err := readOrg(codec, orgDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &MissingControlFileError{Dir: dir, File: orgFile}
		}
		return nil, err
	}

	// descend descends into the directory reading the team files and updating the org struct above.
	var descend func(string, *Team) error
	descend = func(dir string, parentTeam *Team) error {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, f := range files {
			path := filepath.Join(dir, f.Name())

			if f.IsDir() {
				// Every subdirectory must contain a team.yaml file
				t, err := readTeam(codec, path)
				if err != nil {
					if os.IsNotExist(err) {
						return &MissingControlFileError{Dir: path, File: teamFile}
					}
					return err
				}

				// Check that the team directory is named correctly
				normTeamName := normaliseName(t.Name)
				if f.Name() != normTeamName {
					return &InvalidTeamDirNameError{
						Dir:      filepath.Join(dir, f.Name()),
						ValidDir: filepath.Join(dir, normTeamName),
						TeamName: t.Name,
					}
				}

				// Recursively read the team's children
				if err := descend(path, t); err != nil {
					return err
				}

				// Append the team to the appropriate location
				if parentTeam != nil {
					parentTeam.Children = append(parentTeam.Children, t)
				} else {
					org.Teams = append(org.Teams, t)
				}

				continue
			}

			// org.yaml files are only allowed in the top level org directory
			if f.Name() == orgFile {
				if dir != orgDir {
					return &UnexpectedFileError{Path: path, Reason: multipleOrgFilesReason}
				}
				continue
			}

			// team.yaml files are only allowed in subdirectories of the top level org directory
			if f.Name() == teamFile {
				if dir == orgDir {
					return &UnexpectedFileError{Path: path, Reason: noSiblingFilesReason}
				}
				continue
			}

			// All files other than org.yaml and team.yaml are unrecognised
			return &UnexpectedFileError{Path: path, Reason: unrecognisedFileReason}
		}

		return nil
	}

	if err := descend(dir, nil); err != nil {
		return nil, err
	}

	return org, nil
}

// UnmergeOrg decomposes the specified Org into a hierarchy of teams and writes them to the specified
// directory under a top-level directory named after the organisation. Directory names are normalised
// representations of the team/org name.
func UnmergeOrg(codec Codec, org *Org, dir string) error {
	orgDir := filepath.Join(dir, normaliseName(org.Name))

	// Create the org directory
	if err := os.Mkdir(orgDir, 0755); err != nil {
		return err
	}

	// Write org.yaml
	if err := writeOrg(codec, *org, orgDir); err != nil {
		return err
	}

	// descend walks across and down the team hierarchy creating subdirectories
	// and writing team.yaml files into the specified directory.
	var descend func(string, []*Team) error
	descend = func(dir string, teams []*Team) error {
		for _, t := range teams {
			teamDir := filepath.Join(dir, normaliseName(t.Name))

			// Create the team directory
			if err := os.Mkdir(teamDir, 0755); err != nil {
				return err
			}

			// Write team.yaml
			if err := writeTeam(codec, *t, teamDir); err != nil {
				return err
			}

			// Recurse
			if err := descend(teamDir, t.Children); err != nil {
				return err
			}
		}
		return nil
	}

	if err := descend(orgDir, org.Teams); err != nil {
		return err
	}

	return nil
}

// readOrg reads the org.yaml file from the specified directory and returns it as an Org.
func readOrg(codec Codec, dir string) (*Org, error) {
	path := filepath.Join(dir, orgFile)
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	org := Org{}
	if err = codec.Decode(buf, &org); err != nil {
		return nil, errors.Wrapf(err, "could not unmarshal %s", path)
	}

	return &org, nil
}

// writeOrg writes an org.yaml file for the org to the specified directory after removing
// the teams from the org (as they will be represented within the directory hierarchy).
func writeOrg(codec Codec, org Org, dir string) error {
	org.Teams = nil

	buf, err := codec.Encode(&org)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(dir, orgFile), buf, 0644); err != nil {
		return err
	}

	return nil
}

// readTeam reads the team.yaml file from the specified directory and returns it as a Team.
func readTeam(codec Codec, dir string) (*Team, error) {
	path := filepath.Join(dir, teamFile)
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	team := Team{}
	if err = codec.Decode(buf, &team); err != nil {
		return nil, errors.Wrapf(err, "could not unmarshal %s", path)
	}

	return &team, nil
}

// writeTeam writes a team.yaml file for the team to the specified directory after removing
// the child teams (as they will be represented within the directory hierarchy).
func writeTeam(codec Codec, t Team, dir string) error {
	t.Children = nil

	buf, err := codec.Encode(&t)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(dir, teamFile), buf, 0644); err != nil {
		return err
	}

	return nil
}
