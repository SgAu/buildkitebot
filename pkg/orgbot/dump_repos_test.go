package orgbot

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
)

func TestDumpReposTopics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plat := NewTestPlatform(ctrl)
	ctx := context.Background()

	haveRepos := testRepos()

	plat.MockGitHubService.
		EXPECT().
		WalkRepos(ctx, "SEEK-Jobs", gomock.Any()).
		DoAndReturn(func(ctx context.Context, orgName string, walkFn WalkReposFunc) error {
			for _, r := range haveRepos {
				if err := walkFn(r); err != nil {
					return err
				}
			}
			return nil
		})

	repos, err := DumpRepos(ctx, plat, "SEEK-Jobs")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(haveRepos, repos); diff != "" {
		t.Errorf("(-want +got)\n%s", diff)
	}
}
