package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// Commit represents a git commit with selection state for filtering
type Commit struct {
	Hash          plumbing.Hash
	Subject       string
	Author        string
	Selected      bool
	IsUncommitted bool // True for the virtual "uncommitted changes" entry
}

// ListCommits returns commits between baseBranch and HEAD, plus a virtual
// "uncommitted changes" entry if there are working directory changes.
func ListCommits(repoPath, baseBranch string) ([]Commit, error) {
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

	baseRef, ok := resolveBaseRef(repoPath, baseBranch)
	if !ok {
		return commits, nil
	}

	branchCommits, err := listCommitRange(repoPath, baseRef+"..HEAD")
	if err != nil {
		return commits, nil
	}
	commits = append(commits, branchCommits...)

	return commits, nil
}

// hasUncommittedChanges checks if there are staged or unstaged changes
func hasUncommittedChanges(repoPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(bytes.TrimSpace(out)) > 0
}

func resolveBaseRef(repoPath, baseBranch string) (string, bool) {
	if baseBranch == "" {
		return "", false
	}
	if hasRef(repoPath, baseBranch) {
		return baseBranch, true
	}
	remoteRef := "origin/" + baseBranch
	if hasRef(repoPath, remoteRef) {
		return remoteRef, true
	}
	return "", false
}

func hasRef(repoPath, ref string) bool {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", ref+"^{commit}")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func listCommitRange(repoPath, revRange string) ([]Commit, error) {
	cmd := exec.Command("git", "-C", repoPath, "log", "--topo-order", "--format=%H%x00%s%x00%an", revRange)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", revRange, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	commits := make([]Commit, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) < 2 {
			continue
		}
		hash := plumbing.NewHash(parts[0])
		subject := parts[1]
		author := ""
		if len(parts) > 2 {
			author = parts[2]
		}
		commits = append(commits, Commit{
			Hash:     hash,
			Subject:  subject,
			Author:   author,
			Selected: true,
		})
	}

	return commits, nil
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
