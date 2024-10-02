package vcs_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/grafana/alloy/internal/vcs"
	"github.com/stretchr/testify/require"
)

func Test_BasicUsage_Branch(t *testing.T) {
	branchName := "master"
	origRepo, repoDirectory := initRepository(t, branchName)

	msg1 := origRepo.commit()

	newRepoDir := t.TempDir()
	newRepo, err := vcs.NewGitRepo(context.Background(), newRepoDir, vcs.GitRepoOptions{
		Repository: repoDirectory,
		Revision:   branchName,
	})
	require.NoError(t, err)

	bb, err := newRepo.ReadFile("a.txt")
	require.NoError(t, err)
	require.Equal(t, msg1, string(bb))

	msg2 := origRepo.commit()

	err = newRepo.Update(context.Background())
	require.NoError(t, err)

	bb, err = newRepo.ReadFile("a.txt")
	require.NoError(t, err)
	require.Equal(t, msg2, string(bb))
}

// If GitRepo's implementation depends on fast forwarding,
// it may break when branches are diverging.
func Test_NonFastForward(t *testing.T) {
	firstBranch := "first-branch"
	secondBranch := "second-branch"

	repo, repoDirectory := initRepository(t, firstBranch)

	// Make sure all branches diverge.
	repo.commit()
	repo.createBranch(secondBranch)
	firstBranchMsg := repo.commit()
	repo.checkout(secondBranch)
	repo.commit()

	newRepoDir := t.TempDir()
	tracker, err := vcs.NewGitRepo(context.Background(), newRepoDir, vcs.GitRepoOptions{
		Repository: repoDirectory,
		Revision:   firstBranch,
	})
	require.NoError(t, err)

	repo.validate(tracker, firstBranchMsg)

	err = tracker.Update(context.Background())
	require.NoError(t, err)

	repo.checkout(firstBranch)
	msg := repo.commit()

	err = tracker.Update(context.Background())
	require.NoError(t, err)

	repo.validate(tracker, msg)
}

type testRepository struct {
	t           *testing.T
	repo        *git.Repository
	worktree    *git.Worktree
	commitCount uint
	filename    string
}

func (repo *testRepository) CurrentRef() (string, error) {
	ref, err := repo.repo.Head()
	if err != nil {
		return "", nil
	}
	return ref.Name().Short(), nil
}

func (repo *testRepository) WriteFile(path string, contents []byte) error {
	f, err := repo.worktree.Filesystem.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(contents)
	return err
}

// initRepository creates a new, uninitialized Git repository in a temporary
// directory. The Git repository is deleted when the test exits.
func initRepository(t *testing.T, branchName string) (*testRepository, string) {
	t.Helper()

	worktreeDir := t.TempDir()
	repo, err := git.PlainInit(worktreeDir, false)
	require.NoError(t, err)

	// Create a placeholder config for the repo.
	{
		cfg := config.NewConfig()
		cfg.User.Name = "Go test"
		cfg.User.Email = "go-test@example.com"

		err := repo.SetConfig(cfg)
		require.NoError(t, err)
	}

	worktree, err := repo.Worktree()
	require.NoError(t, err)

	r := &testRepository{
		t:           t,
		repo:        repo,
		worktree:    worktree,
		commitCount: 0,
		filename:    "a.txt",
	}

	r.commit()
	r.createBranch(branchName)
	r.checkout(branchName)

	return r, worktreeDir
}

// The following examples were used for inspiration::
// https://github.com/go-git/go-git/blob/master/_examples/branch/main.go
// https://github.com/go-git/go-git/issues/632
func (r *testRepository) createBranch(branchName string) {
	head, err := r.repo.Head()
	require.NoError(r.t, err)

	branchName = `refs/heads/` + branchName
	ref := plumbing.NewHashReference(plumbing.ReferenceName(branchName), head.Hash())
	require.NotNil(r.t, ref)

	err = r.repo.Storer.SetReference(ref)
	require.NoError(r.t, err)

	require.NoError(r.t, err)
}

func (r *testRepository) checkout(branchName string) {
	ref := plumbing.NewBranchReferenceName(branchName)
	require.NoError(r.t, ref.Validate())

	err := r.worktree.Checkout(&git.CheckoutOptions{
		Branch: ref,
	})
	require.NoError(r.t, err)
}

// Write a unique message on a file into the repository and commit it.
func (r *testRepository) commit() string {
	r.commitCount += 1
	msg := fmt.Sprintf("commit %d", r.commitCount)

	err := r.WriteFile(r.filename, []byte(msg))
	require.NoError(r.t, err)

	_, err = r.worktree.Add(".")
	require.NoError(r.t, err)

	_, err = r.worktree.Commit(msg, &git.CommitOptions{})
	require.NoError(r.t, err)

	return msg
}

func (r *testRepository) validate(tracker *vcs.GitRepo, expectedMsg string) {
	bb, err := tracker.ReadFile(r.filename)
	require.NoError(r.t, err)
	require.Equal(r.t, expectedMsg, string(bb))
}
