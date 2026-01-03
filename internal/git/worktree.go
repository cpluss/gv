package git

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree
type Worktree struct {
	Path      string
	Branch    string
	Head      string
	IsCurrent bool
	IsBare    bool
}

// ListWorktrees discovers all worktrees for the repository at the given path.
// Uses git worktree list --porcelain since go-git doesn't support worktree listing.
func ListWorktrees(repoPath string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseWorktreeOutput(string(output))
}

// parseWorktreeOutput parses the porcelain output of git worktree list
func parseWorktreeOutput(output string) ([]Worktree, error) {
	var worktrees []Worktree
	var current *Worktree

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current = &Worktree{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if strings.HasPrefix(line, "HEAD ") {
			if current != nil {
				current.Head = strings.TrimPrefix(line, "HEAD ")
			}
		} else if strings.HasPrefix(line, "branch ") {
			if current != nil {
				branch := strings.TrimPrefix(line, "branch ")
				// Convert refs/heads/foo to foo
				current.Branch = strings.TrimPrefix(branch, "refs/heads/")
			}
		} else if line == "bare" {
			if current != nil {
				current.IsBare = true
			}
		} else if line == "detached" {
			if current != nil {
				current.Branch = "(detached)"
			}
		}
	}

	// Don't forget the last worktree
	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees, scanner.Err()
}

// FindCurrentWorktree returns the worktree that contains the given path
func FindCurrentWorktree(worktrees []Worktree, currentPath string) int {
	absPath, err := filepath.Abs(currentPath)
	if err != nil {
		return 0
	}

	for i, wt := range worktrees {
		wtAbs, err := filepath.Abs(wt.Path)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, wtAbs) {
			return i
		}
	}
	return 0
}

// GetMainBranch attempts to determine the main branch name
func GetMainBranch(repoPath string) string {
	// Try to get from origin HEAD
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		return strings.TrimPrefix(ref, "refs/remotes/origin/")
	}

	// Fall back to checking if main or master exists
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = repoPath
		if err := cmd.Run(); err == nil {
			return branch
		}
	}

	return "main" // Default assumption
}
