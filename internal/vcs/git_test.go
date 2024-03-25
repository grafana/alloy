package vcs_test

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/grafana/alloy/internal/vcs"
	"github.com/stretchr/testify/require"
)

func Test_GitRepo(t *testing.T) {
	origRepo := initRepository(t)

	// Write a file into the repository and commit it.
	{
		err := origRepo.WriteFile("a.txt", []byte("Hello, world!"))
		require.NoError(t, err)

		_, err = origRepo.Worktree.Add(".")
		require.NoError(t, err)

		_, err = origRepo.Worktree.Commit("initial commit", &git.CommitOptions{})
		require.NoError(t, err)
	}

	origRef, err := origRepo.CurrentRef()
	require.NoError(t, err)

	newRepoDir := t.TempDir()
	newRepo, err := vcs.NewGitRepo(context.Background(), newRepoDir, vcs.GitRepoOptions{
		Repository: origRepo.Directory,
		Revision:   origRef,
	})
	require.NoError(t, err)

	bb, err := newRepo.ReadFile("a.txt")
	require.NoError(t, err)
	require.Equal(t, "Hello, world!", string(bb))

	// Update the file.
	{
		err := origRepo.WriteFile("a.txt", []byte("See you later!"))
		require.NoError(t, err)

		_, err = origRepo.Worktree.Add(".")
		require.NoError(t, err)

		_, err = origRepo.Worktree.Commit("commit 2", &git.CommitOptions{})
		require.NoError(t, err)
	}

	err = newRepo.Update(context.Background())
	require.NoError(t, err)

	bb, err = newRepo.ReadFile("a.txt")
	require.NoError(t, err)
	require.Equal(t, "See you later!", string(bb))
}

type testRepository struct {
	Directory string
	Repo      *git.Repository
	Worktree  *git.Worktree
}

func (repo *testRepository) CurrentRef() (string, error) {
	ref, err := repo.Repo.Head()
	if err != nil {
		return "", nil
	}
	return ref.Name().Short(), nil
}

func (repo *testRepository) WriteFile(path string, contents []byte) error {
	f, err := repo.Worktree.Filesystem.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(contents)
	return err
}

// initRepository creates a new, uninitialized Git repository in a temporary
// directory. The Git repository is deleted when the test exits.
func initRepository(t *testing.T) *testRepository {
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

	return &testRepository{
		Directory: worktreeDir,
		Repo:      repo,
		Worktree:  worktree,
	}
}
