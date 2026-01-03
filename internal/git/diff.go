package git

import (
	"bufio"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// FileDiff represents the diff for a single file
type FileDiff struct {
	Path      string
	OldPath   string // For renames
	Added     int
	Removed   int
	Hunks     []Hunk
	Collapsed bool
	IsBinary  bool
}

// Hunk represents a diff hunk
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// DiffLine represents a single line in a diff
type DiffLine struct {
	Type    LineType
	Content string
	OldNum  int
	NewNum  int
}

// LineType indicates whether a line was added, removed, or context
type LineType int

const (
	LineContext LineType = iota
	LineAdded
	LineRemoved
)

var (
	diffFileHeaderRe = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)
	hunkHeaderRe     = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
)

// ComputeDiff computes the diff between base branch and working directory.
// This includes both committed and uncommitted changes.
// Uses git diff command for reliable working directory support.
func ComputeDiff(repoPath, baseBranch string, selectedHashes []plumbing.Hash) ([]FileDiff, error) {
	// Use git diff to compare base branch to working directory
	// This shows: committed changes + staged changes + unstaged changes
	cmd := exec.Command("git", "diff", baseBranch)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		// Try with origin/ prefix
		cmd = exec.Command("git", "diff", "origin/"+baseBranch)
		cmd.Dir = repoPath
		output, err = cmd.Output()
		if err != nil {
			return nil, err
		}
	}

	return parseDiffOutput(string(output))
}

// parseDiffOutput parses unified diff output into FileDiff structs
func parseDiffOutput(output string) ([]FileDiff, error) {
	var diffs []FileDiff
	var currentDiff *FileDiff
	var currentHunk *Hunk

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// New file diff
		if matches := diffFileHeaderRe.FindStringSubmatch(line); matches != nil {
			// Save previous diff
			if currentDiff != nil {
				if currentHunk != nil {
					currentDiff.Hunks = append(currentDiff.Hunks, *currentHunk)
				}
				diffs = append(diffs, *currentDiff)
			}

			currentDiff = &FileDiff{
				OldPath: matches[1],
				Path:    matches[2],
			}
			if currentDiff.OldPath == currentDiff.Path {
				currentDiff.OldPath = ""
			}
			currentHunk = nil
			continue
		}

		if currentDiff == nil {
			continue
		}

		// Binary file
		if strings.HasPrefix(line, "Binary files") {
			currentDiff.IsBinary = true
			continue
		}

		// New hunk
		if matches := hunkHeaderRe.FindStringSubmatch(line); matches != nil {
			// Save previous hunk
			if currentHunk != nil {
				currentDiff.Hunks = append(currentDiff.Hunks, *currentHunk)
			}

			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}

			currentHunk = &Hunk{
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
			}
			continue
		}

		// Skip other header lines
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") || strings.HasPrefix(line, "old mode") ||
			strings.HasPrefix(line, "new mode") || strings.HasPrefix(line, "similarity") ||
			strings.HasPrefix(line, "rename from") || strings.HasPrefix(line, "rename to") {
			continue
		}

		// Diff content lines
		if currentHunk != nil && len(line) > 0 {
			var lineType LineType
			var content string

			switch line[0] {
			case '+':
				lineType = LineAdded
				content = line[1:]
				currentDiff.Added++
			case '-':
				lineType = LineRemoved
				content = line[1:]
				currentDiff.Removed++
			case ' ':
				lineType = LineContext
				content = line[1:]
			case '\\':
				// "\ No newline at end of file" - skip
				continue
			default:
				lineType = LineContext
				content = line
			}

			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:    lineType,
				Content: content,
			})
		}
	}

	// Don't forget the last file/hunk
	if currentDiff != nil {
		if currentHunk != nil {
			currentDiff.Hunks = append(currentDiff.Hunks, *currentHunk)
		}
		diffs = append(diffs, *currentDiff)
	}

	return diffs, scanner.Err()
}

// ComputeStats returns total added/removed lines across all file diffs
func ComputeStats(diffs []FileDiff) (added, removed int) {
	for _, d := range diffs {
		added += d.Added
		removed += d.Removed
	}
	return
}
