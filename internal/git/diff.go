package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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

// ComputeDiff computes the diff based on selected commits.
// Uses git diff command for reliable working directory support.
//
// Selection logic:
// - Uncommitted + commits selected: git diff <base> (full working dir diff)
// - Only uncommitted selected: git diff HEAD (just working dir changes)
// - Only commits selected: git diff <base>..HEAD (just committed changes)
// - Nothing selected: empty diff
func ComputeDiff(repoPath, baseBranch string, commits []Commit) ([]FileDiff, error) {
	return ComputeDiffWithContext(repoPath, baseBranch, commits, 3)
}

// ComputeDiffWithContext computes the diff with specified context lines.
func ComputeDiffWithContext(repoPath, baseBranch string, commits []Commit, contextLines int) ([]FileDiff, error) {
	// Check what's selected
	uncommittedSelected := false
	anyCommitSelected := false

	for _, c := range commits {
		if c.Selected {
			if c.IsUncommitted {
				uncommittedSelected = true
			} else {
				anyCommitSelected = true
			}
		}
	}

	// Nothing selected = no diff
	if !uncommittedSelected && !anyCommitSelected {
		return nil, nil
	}

	contextArg := fmt.Sprintf("-U%d", contextLines)
	var args []string
	if uncommittedSelected && anyCommitSelected {
		// Full diff: base to working directory
		args = []string{"diff", contextArg, baseBranch}
	} else if uncommittedSelected {
		// Just uncommitted: HEAD to working directory
		args = []string{"diff", contextArg, "HEAD"}
	} else {
		// Just commits: base to HEAD
		args = []string{"diff", contextArg, baseBranch + "..HEAD"}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		// Try with origin/ prefix for base branch
		if len(args) > 2 && strings.Contains(args[2], baseBranch) {
			args[2] = strings.Replace(args[2], baseBranch, "origin/"+baseBranch, 1)
			cmd = exec.Command("git", args...)
			cmd.Dir = repoPath
			output, err = cmd.Output()
			if err != nil {
				return nil, err
			}
		} else {
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
			var oldNum, newNum int

			// Track current line numbers based on hunk start
			oldLineNum := currentHunk.OldStart
			newLineNum := currentHunk.NewStart
			for _, l := range currentHunk.Lines {
				switch l.Type {
				case LineRemoved:
					oldLineNum++
				case LineAdded:
					newLineNum++
				case LineContext:
					oldLineNum++
					newLineNum++
				}
			}

			switch line[0] {
			case '+':
				lineType = LineAdded
				content = line[1:]
				currentDiff.Added++
				newNum = newLineNum
			case '-':
				lineType = LineRemoved
				content = line[1:]
				currentDiff.Removed++
				oldNum = oldLineNum
			case ' ':
				lineType = LineContext
				content = line[1:]
				oldNum = oldLineNum
				newNum = newLineNum
			case '\\':
				// "\ No newline at end of file" - skip
				continue
			default:
				lineType = LineContext
				content = line
				oldNum = oldLineNum
				newNum = newLineNum
			}

			currentHunk.Lines = append(currentHunk.Lines, DiffLine{
				Type:    lineType,
				Content: content,
				OldNum:  oldNum,
				NewNum:  newNum,
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
