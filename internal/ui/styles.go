package ui

import "github.com/charmbracelet/lipgloss"

// Styles contains all the styling for the UI
type Styles struct {
	// Layout
	Header       lipgloss.Style
	Footer       lipgloss.Style
	StatusBar    lipgloss.Style
	FileHeader   lipgloss.Style
	DiffPane     lipgloss.Style
	PopupOverlay lipgloss.Style
	Popup        lipgloss.Style

	// Diff content
	LineNumber  lipgloss.Style
	LineAdded   lipgloss.Style
	LineRemoved lipgloss.Style
	LineContext lipgloss.Style
	AddedBg     lipgloss.Style
	RemovedBg   lipgloss.Style
	HunkHeader  lipgloss.Style

	// Stats
	StatsAdded   lipgloss.Style
	StatsRemoved lipgloss.Style

	// Selection
	Selected   lipgloss.Style
	Unselected lipgloss.Style
	Cursor     lipgloss.Style

	// Help
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style

	// Worktree list
	WorktreeCurrent lipgloss.Style
	WorktreePath    lipgloss.Style
	WorktreeBranch  lipgloss.Style
}

// DefaultStyles returns the default color scheme
func DefaultStyles() Styles {
	return Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c9d1d9")).
			Background(lipgloss.Color("#161b22")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")).
			Background(lipgloss.Color("#0d1117")).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c9d1d9")).
			Background(lipgloss.Color("#21262d")).
			Padding(0, 1),

		FileHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c9d1d9")).
			Background(lipgloss.Color("#21262d")).
			Padding(0, 1),

		DiffPane: lipgloss.NewStyle().
			Padding(0, 1),

		PopupOverlay: lipgloss.NewStyle().
			Background(lipgloss.Color("#0d1117")),

		Popup: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#58a6ff")).
			Padding(1, 2),

		LineNumber: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e7681")).
			Width(4).
			Align(lipgloss.Right).
			MarginRight(1),

		LineAdded: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3fb950")),

		LineRemoved: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f85149")),

		LineContext: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c9d1d9")),

		AddedBg: lipgloss.NewStyle().
			Background(lipgloss.Color("#0f2919")),

		RemovedBg: lipgloss.NewStyle().
			Background(lipgloss.Color("#2a1112")),

		HunkHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")).
			Italic(true),

		StatsAdded: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3fb950")).
			Bold(true),

		StatsRemoved: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f85149")).
			Bold(true),

		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3fb950")).
			SetString("[x]"),

		Unselected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")).
			SetString("[ ]"),

		Cursor: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e3b341")),

		HelpKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#58a6ff")).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")),

		WorktreeCurrent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3fb950")).
			Bold(true),

		WorktreePath: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")),

		WorktreeBranch: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#58a6ff")),
	}
}
