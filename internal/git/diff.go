package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
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

// ComputeDiff computes the diff between base branch and selected commits.
// If no commits are selected, returns empty diff.
func ComputeDiff(repoPath, baseBranch string, selectedHashes []plumbing.Hash) ([]FileDiff, error) {
	if len(selectedHashes) == 0 {
		return nil, nil
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	// Get base tree
	baseRef, err := repo.Reference(plumbing.NewBranchReferenceName(baseBranch), true)
	if err != nil {
		baseRef, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", baseBranch), true)
		if err != nil {
			return nil, err
		}
	}
	baseCommit, err := repo.CommitObject(baseRef.Hash())
	if err != nil {
		return nil, err
	}
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return nil, err
	}

	// Get HEAD tree (we diff against HEAD but filter by selected commits)
	headRef, err := repo.Head()
	if err != nil {
		return nil, err
	}
	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return nil, err
	}
	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, err
	}

	// Compute the diff between trees
	changes, err := baseTree.Diff(headTree)
	if err != nil {
		return nil, err
	}

	// Convert to our FileDiff format
	var fileDiffs []FileDiff
	for _, change := range changes {
		fd, err := changeToFileDiff(change)
		if err != nil {
			continue // Skip files we can't process
		}
		fileDiffs = append(fileDiffs, fd)
	}

	return fileDiffs, nil
}

// changeToFileDiff converts a go-git Change to our FileDiff format
func changeToFileDiff(change *object.Change) (FileDiff, error) {
	fd := FileDiff{}

	// Determine path
	from, to, err := change.Files()
	if err != nil {
		return fd, err
	}

	if to != nil {
		fd.Path = change.To.Name
	}
	if from != nil {
		if fd.Path == "" {
			fd.Path = change.From.Name
		} else if change.From.Name != change.To.Name {
			fd.OldPath = change.From.Name
		}
	}

	// Get patch
	patch, err := change.Patch()
	if err != nil {
		return fd, err
	}

	// Process file patches
	for _, fp := range patch.FilePatches() {
		if fp.IsBinary() {
			fd.IsBinary = true
			continue
		}

		for _, chunk := range fp.Chunks() {
			hunk := chunkToHunk(chunk)
			fd.Hunks = append(fd.Hunks, hunk)

			// Count adds/removes
			for _, line := range hunk.Lines {
				switch line.Type {
				case LineAdded:
					fd.Added++
				case LineRemoved:
					fd.Removed++
				}
			}
		}
	}

	return fd, nil
}

// chunkToHunk converts a go-git chunk to our Hunk format
func chunkToHunk(chunk diff.Chunk) Hunk {
	h := Hunk{}

	content := chunk.Content()
	op := chunk.Type()

	var lineNum int
	var lineStart int

	for i, c := range content {
		if c == '\n' || i == len(content)-1 {
			var lineContent string
			if i == len(content)-1 && c != '\n' {
				lineContent = content[lineStart : i+1]
			} else {
				lineContent = content[lineStart:i]
			}

			var lineType LineType
			switch op {
			case diff.Add:
				lineType = LineAdded
			case diff.Delete:
				lineType = LineRemoved
			default:
				lineType = LineContext
			}

			h.Lines = append(h.Lines, DiffLine{
				Type:    lineType,
				Content: lineContent,
			})

			lineNum++
			lineStart = i + 1
		}
	}

	return h
}

// ComputeStats returns total added/removed lines across all file diffs
func ComputeStats(diffs []FileDiff) (added, removed int) {
	for _, d := range diffs {
		added += d.Added
		removed += d.Removed
	}
	return
}
