package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Commit represents a git commit with selection state for filtering
type Commit struct {
	Hash        plumbing.Hash
	Subject     string
	Author      string
	Selected    bool
	IsUncommitted bool // True for the virtual "uncommitted changes" entry
}

// ListCommits returns commits between baseBranch and HEAD, plus a virtual
// "uncommitted changes" entry if there are working directory changes.
func ListCommits(repoPath, baseBranch string) ([]Commit, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	var commits []Commit

	// Check for uncommitted changes and add virtual commit
	if hasUncommittedChanges(repoPath) {
		commits = append(commits, Commit{
			Subject:       "(uncommitted changes)",
			Author:        "",
			Selected:      true,
			IsUncommitted: true,
		})
	}

	// Get HEAD
	headRef, err := repo.Head()
	if err != nil {
		return commits, nil // Return just uncommitted if no HEAD
	}

	// Get base branch reference
	baseRef, err := repo.Reference(plumbing.NewBranchReferenceName(baseBranch), true)
	if err != nil {
		// Try remote reference
		baseRef, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", baseBranch), true)
		if err != nil {
			return commits, nil // Return just uncommitted if no base
		}
	}

	// Get base commit
	baseCommit, err := repo.CommitObject(baseRef.Hash())
	if err != nil {
		return commits, nil
	}

	// Walk from HEAD back to find commits not reachable from base
	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return commits, nil
	}

	// Build set of commits reachable from base
	baseCommits := make(map[plumbing.Hash]bool)
	err = buildCommitSet(baseCommit, baseCommits)
	if err != nil {
		return commits, nil
	}

	// Collect commits from HEAD that aren't in base
	var branchCommits []Commit
	err = collectBranchCommits(headCommit, baseCommits, &branchCommits)
	if err != nil {
		return commits, nil
	}

	// Mark all as selected by default and append
	for i := range branchCommits {
		branchCommits[i].Selected = true
	}
	commits = append(commits, branchCommits...)

	return commits, nil
}

// hasUncommittedChanges checks if there are staged or unstaged changes
func hasUncommittedChanges(repoPath string) bool {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false
	}

	wt, err := repo.Worktree()
	if err != nil {
		return false
	}

	status, err := wt.Status()
	if err != nil {
		return false
	}

	return !status.IsClean()
}

// buildCommitSet builds a set of all commits reachable from the given commit
func buildCommitSet(c *object.Commit, set map[plumbing.Hash]bool) error {
	if set[c.Hash] {
		return nil
	}
	set[c.Hash] = true

	parents := c.Parents()
	defer parents.Close()

	for {
		parent, err := parents.Next()
		if err != nil {
			break
		}
		if err := buildCommitSet(parent, set); err != nil {
			return err
		}
	}
	return nil
}

// collectBranchCommits collects commits from c that aren't in baseSet
func collectBranchCommits(c *object.Commit, baseSet map[plumbing.Hash]bool, commits *[]Commit) error {
	if baseSet[c.Hash] {
		return nil
	}

	// Check if we already have this commit
	for _, existing := range *commits {
		if existing.Hash == c.Hash {
			return nil
		}
	}

	*commits = append(*commits, Commit{
		Hash:    c.Hash,
		Subject: firstLine(c.Message),
		Author:  c.Author.Name,
	})

	parents := c.Parents()
	defer parents.Close()

	for {
		parent, err := parents.Next()
		if err != nil {
			break
		}
		if err := collectBranchCommits(parent, baseSet, commits); err != nil {
			return err
		}
	}
	return nil
}

// firstLine returns the first line of a string
func firstLine(s string) string {
	for i, c := range s {
		if c == '\n' {
			return s[:i]
		}
	}
	return s
}

// SelectedHashes returns the hashes of selected commits
func SelectedHashes(commits []Commit) []plumbing.Hash {
	var hashes []plumbing.Hash
	for _, c := range commits {
		if c.Selected {
			hashes = append(hashes, c.Hash)
		}
	}
	return hashes
}
