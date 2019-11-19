package orgbot

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/SEEK-Jobs/orgbot/pkg/yaml"
)

func TestMerge(t *testing.T) {
	want := &Org{
		Name: "Org-A",
		Teams: []*Team{
			{
				Name:        "Team A",
				Description: "Team A description",
				Members: []string{
					"team-a-member-1@seek.com.au",
					"team-a-member-2@seek.com.au",
				},
			},
			{
				Name:        "Team B",
				Description: "Team B description",
				Members: []string{
					"team-b-member-1@seek.com.au",
					"team-b-member-2@seek.com.au",
				},
				Children: []*Team{
					{
						Name: "Team C",
						Previously: []string{
							"Team X",
						},
						Description: "Team C description",
						Members: []string{
							"team-c-member-1@seek.com.au",
							"team-c-member-2@seek.com.au",
						},
					},
				},
			},
		},
	}

	got, err := MergeOrg(yaml.NewCodec(), "test_data/valid")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func TestMergeValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		wantErr error
	}{
		{
			name: "BadDirName",
			dir:  "test_data/bad-dir-name",
			wantErr: &InvalidTeamDirNameError{
				Dir:      "test_data/bad-dir-name/Team-A",
				ValidDir: "test_data/bad-dir-name/team-a",
				TeamName: "Team A",
			},
		},
		{
			name: "BadTeamFileName",
			dir:  "test_data/bad-team-file-name",
			wantErr: &MissingControlFileError{
				Dir:  "test_data/bad-team-file-name/team-a",
				File: teamFile,
			},
		},
		{
			name:    "HasMaintainers",
			dir:     "test_data/has-maintainers",
			wantErr: nil, // This is just a generic YAML type error
		},
		{
			name: "MissingOrgFile",
			dir:  "test_data/missing-org-file",
			wantErr: &MissingControlFileError{
				Dir:  "test_data/missing-org-file",
				File: orgFile,
			},
		},
		{
			name: "MultipleOrgFiles",
			dir:  "test_data/multiple-org-files",
			wantErr: &UnexpectedFileError{
				Path:   "test_data/multiple-org-files/team-a/org.yaml",
				Reason: multipleOrgFilesReason,
			},
		},
		{
			name: "SiblingFiles",
			dir:  "test_data/sibling-files",
			wantErr: &UnexpectedFileError{
				Path:   "test_data/sibling-files/team.yaml",
				Reason: noSiblingFilesReason,
			},
		},
		{
			name: "UnexpectedFile",
			dir:  "test_data/unexpected-file",
			wantErr: &UnexpectedFileError{
				Path:   "test_data/unexpected-file/foobar.txt",
				Reason: unrecognisedFileReason,
			},
		},
	}

	for _, test := range tests {
		_, err := MergeOrg(yaml.NewCodec(), test.dir)
		if test.wantErr != nil {
			if diff := cmp.Diff(test.wantErr, err); diff != "" {
				t.Errorf("Test case '%s': (-want +got)\n%s", test.name, diff)
			}
		} else if err == nil {
			t.Errorf("Test case '%s': expected error but got nil\n", test.name)
		}
	}
}

func TestUnmergeNormaliseName(t *testing.T) {
	t.Run("ValidDir", unmergeNormaliseNameValidName)
	t.Run("SpacesGetReplaced", unmergeNormaliseNameSpacesGetReplaced)
	t.Run("BadCharactersAreDropped", unmergeNormaliseNameBadCharactersAreDropped)
	t.Run("BadCharacters", unmergeNormaliseNameBadCharacters)
	t.Run("DoesntStartOrEndWithAHyphen", unmergeNormaliseNameDoesntStartOrEndWithAHyphen)
	t.Run("MiddleHyphenIsntRemoved", unmergeNormaliseNameMiddleHyphenIsntRemoved)
	t.Run("MultipleHyphensAreRemoved", unmergeNormaliseNameMultipleHyphensAreRemoved)
	t.Run("UnderscoresAreReplaced", unmergeNormaliseNameUnderscoresAreReplaced)
}

func unmergeNormaliseNameValidName(t *testing.T) {
	originalName := "abcdefghijklmnopqrstuvxyz-0123456789goodname"
	fixedName := normaliseName(originalName)

	if diff := cmp.Diff(originalName, fixedName); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func unmergeNormaliseNameSpacesGetReplaced(t *testing.T) {
	originalName := "this name has spaces"
	expectedName := "this-name-has-spaces"
	fixedName := normaliseName(originalName)

	if diff := cmp.Diff(expectedName, fixedName); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func unmergeNormaliseNameBadCharactersAreDropped(t *testing.T) {
	originalName := "†his name is ®eå¬ll¥ bad 👽:ø😀😁😂🤣"
	expectedName := "his-name-is-ell-bad"
	fixedName := normaliseName(originalName)

	if diff := cmp.Diff(expectedName, fixedName); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func unmergeNormaliseNameBadCharacters(t *testing.T) {
	originalName := "👽\\/:*?!\"<>|;,[]()^#%&@+={}'~`."
	expectedName := ""
	fixedName := normaliseName(originalName)

	if diff := cmp.Diff(expectedName, fixedName); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func unmergeNormaliseNameDoesntStartOrEndWithAHyphen(t *testing.T) {
	originalName := " _Team Name_ "
	expectedName := "team-name"
	fixedName := normaliseName(originalName)

	if diff := cmp.Diff(expectedName, fixedName); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func unmergeNormaliseNameMiddleHyphenIsntRemoved(t *testing.T) {
	originalName := "_-fds0-_fds-_"
	expectedName := "fds0-fds"
	fixedName := normaliseName(originalName)

	if diff := cmp.Diff(expectedName, fixedName); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func unmergeNormaliseNameMultipleHyphensAreRemoved(t *testing.T) {
	originalName := "test - name"
	expectedName := "test-name"
	fixedName := normaliseName(originalName)

	if diff := cmp.Diff(expectedName, fixedName); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}

func unmergeNormaliseNameUnderscoresAreReplaced(t *testing.T) {
	originalName := "test_name"
	expectedName := "test-name"
	fixedName := normaliseName(originalName)

	if diff := cmp.Diff(expectedName, fixedName); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}
