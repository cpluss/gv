package git

import "testing"

func TestFindCurrentWorktree(t *testing.T) {
	tests := []struct {
		name        string
		worktrees   []Worktree
		currentPath string
		expected    int
	}{
		{
			name: "exact match first",
			worktrees: []Worktree{
				{Path: "/app"},
				{Path: "/app-feature"},
			},
			currentPath: "/app",
			expected:    0,
		},
		{
			name: "exact match second",
			worktrees: []Worktree{
				{Path: "/app"},
				{Path: "/app-feature"},
			},
			currentPath: "/app-feature",
			expected:    1,
		},
		{
			name: "subdirectory match",
			worktrees: []Worktree{
				{Path: "/app"},
				{Path: "/app-feature"},
			},
			currentPath: "/app-feature/src/lib",
			expected:    1,
		},
		{
			name: "no false prefix match",
			worktrees: []Worktree{
				{Path: "/app"},
				{Path: "/app-feature"},
			},
			currentPath: "/app-feature-two",
			expected:    0, // Falls back to first if no match
		},
		{
			name: "nested worktrees select most specific",
			worktrees: []Worktree{
				{Path: "/workspace/main"},
				{Path: "/workspace/main/features/auth"},
			},
			currentPath: "/workspace/main/features/auth/src",
			expected:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindCurrentWorktree(tt.worktrees, tt.currentPath)
			if result != tt.expected {
				t.Errorf("got %d, want %d", result, tt.expected)
			}
		})
	}
}
