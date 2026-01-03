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
	LineNumber    lipgloss.Style
	LineAdded     lipgloss.Style
	LineRemoved   lipgloss.Style
	LineContext   lipgloss.Style
	AddedBg       lipgloss.Style
	RemovedBg     lipgloss.Style
	HunkHeader    lipgloss.Style

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
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("236")).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("238")).
			Padding(0, 1),

		FileHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("81")).
			Background(lipgloss.Color("237")).
			Padding(0, 1).
			MarginTop(1),

		DiffPane: lipgloss.NewStyle().
			Padding(0, 1),

		PopupOverlay: lipgloss.NewStyle().
			Background(lipgloss.Color("235")),

		Popup: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),

		LineNumber: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(4).
			Align(lipgloss.Right).
			MarginRight(1),

		LineAdded: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")),

		LineRemoved: lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")),

		LineContext: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),

		AddedBg: lipgloss.NewStyle().
			Background(lipgloss.Color("22")),

		RemovedBg: lipgloss.NewStyle().
			Background(lipgloss.Color("52")),

		HunkHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Italic(true),

		StatsAdded: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true),

		StatsRemoved: lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Bold(true),

		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			SetString("[x]"),

		Unselected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			SetString("[ ]"),

		Cursor: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),

		HelpKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("81")).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),

		WorktreeCurrent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true),

		WorktreePath: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),

		WorktreeBranch: lipgloss.NewStyle().
			Foreground(lipgloss.Color("81")),
	}
}
