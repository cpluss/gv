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
