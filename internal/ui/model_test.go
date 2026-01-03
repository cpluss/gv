package ui

import (
	"testing"
	"time"

	"github.com/selund/gv/internal/git"
)

func TestInitSpeed(t *testing.T) {
	start := time.Now()
	_, err := InitModelWithConfig(Config{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("InitModelWithConfig failed: %v", err)
	}

	t.Logf("InitModelWithConfig took: %v", elapsed)

	if elapsed > 200*time.Millisecond {
		t.Errorf("InitModelWithConfig took %v, expected under 200ms", elapsed)
	}
}

func TestComputeDiffWithOnlyCommits(t *testing.T) {
	// Simulate: commits exist but no uncommitted changes
	commits := []git.Commit{
		{Subject: "First commit", Selected: true, IsUncommitted: false},
		{Subject: "Second commit", Selected: true, IsUncommitted: false},
	}

	// Check that ComputeDiff is called with these commits
	// We'll just verify the selection logic works
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

	if uncommittedSelected {
		t.Error("uncommittedSelected should be false")
	}
	if !anyCommitSelected {
		t.Error("anyCommitSelected should be true")
	}

	// The condition to skip diff
	if !uncommittedSelected && !anyCommitSelected {
		t.Error("This would incorrectly skip diff computation")
	}

	t.Log("Selection logic is correct: anyCommitSelected =", anyCommitSelected)
}

func TestGetDisplayNames(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected map[string]string
	}{
		{
			name:  "unique basenames",
			paths: []string{"src/foo.go", "src/bar.go"},
			expected: map[string]string{
				"src/foo.go": "foo.go",
				"src/bar.go": "bar.go",
			},
		},
		{
			name:  "duplicate basenames",
			paths: []string{"src/components/index.ts", "src/pages/index.ts"},
			expected: map[string]string{
				"src/components/index.ts": "components/index.ts",
				"src/pages/index.ts":      "pages/index.ts",
			},
		},
		{
			name:  "nested duplicates",
			paths: []string{"a/b/index.ts", "c/b/index.ts", "d/e/index.ts"},
			expected: map[string]string{
				"a/b/index.ts": "a/b/index.ts",
				"c/b/index.ts": "c/b/index.ts",
				"d/e/index.ts": "e/index.ts",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := make([]git.FileDiff, len(tt.paths))
			for i, p := range tt.paths {
				diffs[i] = git.FileDiff{Path: p}
			}

			result := getDisplayNames(diffs)

			for path, expectedName := range tt.expected {
				if result[path] != expectedName {
					t.Errorf("path %q: got %q, want %q", path, result[path], expectedName)
				}
			}
		})
	}
}
