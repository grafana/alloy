package vcs

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

type GitRepoOptions struct {
	Repository string
	Revision   string
	Auth       GitAuthConfig
}

// GitRepo manages a Git repository for the purposes of retrieving a file from
// it.
type GitRepo struct {
	opts     GitRepoOptions
	repo     *git.Repository
	workTree *git.Worktree
	auth     transport.AuthMethod
}

// NewGitRepo creates a new instance of a GitRepo, where the Git repository is
// managed at storagePath.
//
// 1. If storagePath is empty on disk, NewGitRepo initializes GitRepo by cloning the repository.
// 2. After GitRepo is initialized/opened, a git fetch is don.
// 3. Then, a git checkout is done to the Revision specified in GitRepoOptions.
func NewGitRepo(ctx context.Context, storagePath string, opts GitRepoOptions) (*GitRepo, error) {
	var (
		repo *git.Repository
		err  error
	)

	authConfig, err := opts.Auth.Convert()
	if err != nil {
		return nil, DownloadFailedError{
			Repository: opts.Repository,
			Inner:      err,
		}
	}

	if !isRepoCloned(storagePath) {
		repo, err = git.PlainCloneContext(ctx, storagePath, false, &git.CloneOptions{
			URL:               opts.Repository,
			ReferenceName:     plumbing.HEAD,
			Auth:              authConfig,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
			Tags:              git.AllTags,
		})
	} else {
		repo, err = git.PlainOpen(storagePath)
	}
	if err != nil {
		return nil, DownloadFailedError{
			Repository: opts.Repository,
			Inner:      err,
		}
	}

	// Fetch the latest contents. This may be a no-op if we just did a clone.
	wt, err := repo.Worktree()
	if err != nil {
		return nil, DownloadFailedError{
			Repository: opts.Repository,
			Inner:      err,
		}
	}

	gitRepo := &GitRepo{
		opts:     opts,
		repo:     repo,
		workTree: wt,
		auth:     authConfig,
	}

	err = gitRepo.Update(ctx)
	if err != nil {
		return nil, err
	}

	return gitRepo, nil
}

func isRepoCloned(dir string) bool {
	fi, dirError := os.ReadDir(filepath.Join(dir, git.GitDirName))
	return dirError == nil && len(fi) > 0
}

// Update updates the repository by pulling new content and re-checking out to
// latest version of Revision.
func (repo *GitRepo) Update(ctx context.Context) error {
	fetchErr := repo.repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		Force:      true,
		Auth:       repo.auth,
	})
	if fetchErr != nil && !errors.Is(fetchErr, git.NoErrAlreadyUpToDate) {
		return UpdateFailedError{
			Repository: repo.opts.Repository,
			Inner:      fetchErr,
		}
	}

	checkoutErr := checkout(repo.opts.Revision, repo.repo)
	if checkoutErr != nil {
		if errors.Is(checkoutErr, plumbing.ErrReferenceNotFound) {
			return InvalidRevisionError{repo.opts.Revision}
		}

		return UpdateFailedError{
			Repository: repo.opts.Repository,
			Inner:      checkoutErr,
		}
	}

	return nil
}

// ReadFile returns a file from the repository specified by path.
func (repo *GitRepo) ReadFile(path string) ([]byte, error) {
	f, err := repo.workTree.Filesystem.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

// Stat returns info from the repository specified by path.
func (repo *GitRepo) Stat(path string) (fs.FileInfo, error) {
	f, err := repo.workTree.Filesystem.Stat(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// ReadDir returns info about the content of the directory in the repository.
func (repo *GitRepo) ReadDir(path string) ([]fs.FileInfo, error) {
	f, err := repo.workTree.Filesystem.ReadDir(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// CurrentRevision returns the current revision of the repository (by SHA).
func (repo *GitRepo) CurrentRevision() (string, error) {
	ref, err := repo.repo.Head()
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}

// Depending on the type of revision we need to handle checkout differently.
// Tags are checked out as branches
// Branches as branches
// Commits are commits
func checkout(rev string, repo *git.Repository) error {
	// Try looking for the revision in the following order:
	//
	// 1. Search by tag name.
	// 2. Search by remote ref name.
	// 3. Try to resolve the revision directly.
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	if tagRef, err := repo.Tag(rev); err == nil {
		return wt.Checkout(&git.CheckoutOptions{
			Branch: tagRef.Name(),
			Force:  true,
		})
	}

	if remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", rev), true); err == nil {
		return wt.Checkout(&git.CheckoutOptions{
			Branch: remoteRef.Name(),
			Force:  true,
		})
	}

	if hash, err := repo.ResolveRevision(plumbing.Revision(rev)); err == nil {
		return wt.Checkout(&git.CheckoutOptions{
			Hash:  *hash,
			Force: true,
		})
	}

	return plumbing.ErrReferenceNotFound
}
