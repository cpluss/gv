package ui

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/selund/gv/internal/git"
	"github.com/selund/gv/internal/syntax"
)

// ViewMode represents the current view state
type ViewMode int

const (
	ViewDiff ViewMode = iota
	ViewCommitFilter
	ViewWorktreeSwitcher
	ViewWorktreeList
	ViewHelp
)

// DiffMode represents unified vs side-by-side
type DiffMode int

const (
	DiffSideBySide DiffMode = iota
	DiffUnified
)

// Model is the root application model
type Model struct {
	// Window size
	width  int
	height int

	// State
	worktrees       []git.Worktree
	currentWorktree int
	commits         []git.Commit
	diffs           []git.FileDiff
	mainBranch      string
	repoPath        string

	// View state
	viewMode ViewMode
	diffMode DiffMode
	scroll   int
	cursor   int // For popups

	// Filter state
	filterInput string

	// Components
	styles      Styles
	highlighter *syntax.Highlighter

	// Error state
	err error
}

// InitModel creates a new model from the current directory
func InitModel() (Model, error) {
	m := Model{
		styles:      DefaultStyles(),
		highlighter: syntax.NewHighlighter(),
		viewMode:    ViewDiff,
		diffMode:    DiffSideBySide,
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return m, fmt.Errorf("getting current directory: %w", err)
	}

	// Find git root
	repoPath, err := findGitRoot(cwd)
	if err != nil {
		return m, fmt.Errorf("finding git root: %w", err)
	}
	m.repoPath = repoPath

	// Discover worktrees
	worktrees, err := git.ListWorktrees(repoPath)
	if err != nil {
		return m, fmt.Errorf("listing worktrees: %w", err)
	}
	m.worktrees = worktrees
	m.currentWorktree = git.FindCurrentWorktree(worktrees, cwd)

	// Get main branch
	m.mainBranch = git.GetMainBranch(repoPath)

	// Load initial data
	if err := m.loadData(); err != nil {
		m.err = err
	}

	return m, nil
}

// loadData loads commits and diffs for the current worktree
func (m *Model) loadData() error {
	if len(m.worktrees) == 0 {
		return nil
	}

	wt := m.worktrees[m.currentWorktree]

	// Load commits
	commits, err := git.ListCommits(wt.Path, m.mainBranch)
	if err != nil {
		// Not an error if we're on the main branch
		commits = nil
	}
	m.commits = commits

	// Load diffs
	selectedHashes := git.SelectedHashes(commits)
	diffs, err := git.ComputeDiff(wt.Path, m.mainBranch, selectedHashes)
	if err != nil {
		return err
	}
	m.diffs = diffs

	return nil
}

func findGitRoot(path string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			return path, nil
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("not in a git repository")
		}
		path = parent
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		if m.viewMode == ViewHelp {
			m.viewMode = ViewDiff
		} else {
			m.viewMode = ViewHelp
		}
		return m, nil
	}

	// Mode-specific handling
	switch m.viewMode {
	case ViewDiff:
		return m.handleDiffKey(msg)
	case ViewCommitFilter:
		return m.handleCommitFilterKey(msg)
	case ViewWorktreeSwitcher:
		return m.handleWorktreeSwitcherKey(msg)
	case ViewWorktreeList:
		return m.handleWorktreeListKey(msg)
	case ViewHelp:
		m.viewMode = ViewDiff
		return m, nil
	}

	return m, nil
}

func (m Model) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.scroll++
	case "k", "up":
		if m.scroll > 0 {
			m.scroll--
		}
	case "ctrl+d":
		m.scroll += m.height / 2
	case "ctrl+u":
		if m.scroll > m.height/2 {
			m.scroll -= m.height / 2
		} else {
			m.scroll = 0
		}
	case "g":
		m.scroll = 0
	case "G":
		// Scroll to end - will be clamped in View
		m.scroll = 99999
	case "u":
		if m.diffMode == DiffSideBySide {
			m.diffMode = DiffUnified
		} else {
			m.diffMode = DiffSideBySide
		}
	case "c":
		m.viewMode = ViewCommitFilter
		m.cursor = 0
	case "w":
		m.viewMode = ViewWorktreeSwitcher
		m.cursor = m.currentWorktree
		m.filterInput = ""
	case "W":
		m.viewMode = ViewWorktreeList
		m.cursor = m.currentWorktree
	case "n":
		m.nextFile()
	case "N":
		m.prevFile()
	case "enter":
		m.toggleCurrentFile()
	case "z":
		m.toggleAllFiles()
	}
	return m, nil
}

func (m Model) handleCommitFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.commits)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case " ":
		if m.cursor < len(m.commits) {
			m.commits[m.cursor].Selected = !m.commits[m.cursor].Selected
			m.recomputeDiff()
		}
	case "a":
		for i := range m.commits {
			m.commits[i].Selected = true
		}
		m.recomputeDiff()
	case "n":
		for i := range m.commits {
			m.commits[i].Selected = false
		}
		m.recomputeDiff()
	case "enter", "esc":
		m.viewMode = ViewDiff
	}
	return m, nil
}

func (m Model) handleWorktreeSwitcherKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.worktrees)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		m.currentWorktree = m.cursor
		m.loadData()
		m.viewMode = ViewDiff
		m.scroll = 0
	case "esc":
		m.viewMode = ViewDiff
	}
	return m, nil
}

func (m Model) handleWorktreeListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.worktrees)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		m.currentWorktree = m.cursor
		m.loadData()
		m.viewMode = ViewDiff
		m.scroll = 0
	case "esc":
		m.viewMode = ViewDiff
	}
	return m, nil
}

func (m *Model) recomputeDiff() {
	wt := m.worktrees[m.currentWorktree]
	selectedHashes := git.SelectedHashes(m.commits)
	diffs, err := git.ComputeDiff(wt.Path, m.mainBranch, selectedHashes)
	if err != nil {
		m.err = err
		return
	}
	m.diffs = diffs
}

func (m *Model) nextFile() {
	// Find next file header position and scroll to it
	// This is simplified - full implementation would track file positions
	m.scroll += 20
}

func (m *Model) prevFile() {
	if m.scroll > 20 {
		m.scroll -= 20
	} else {
		m.scroll = 0
	}
}

func (m *Model) toggleCurrentFile() {
	// Find which file we're on and toggle its collapsed state
	// Simplified implementation
	for i := range m.diffs {
		m.diffs[i].Collapsed = !m.diffs[i].Collapsed
		return
	}
}

func (m *Model) toggleAllFiles() {
	// Toggle all files collapsed/expanded
	allCollapsed := true
	for _, d := range m.diffs {
		if !d.Collapsed {
			allCollapsed = false
			break
		}
	}
	for i := range m.diffs {
		m.diffs[i].Collapsed = !allCollapsed
	}
}

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.viewMode {
	case ViewHelp:
		return m.renderHelp()
	case ViewCommitFilter:
		return m.renderWithOverlay(m.renderCommitFilter())
	case ViewWorktreeSwitcher:
		return m.renderWithOverlay(m.renderWorktreeSwitcher())
	case ViewWorktreeList:
		return m.renderWithOverlay(m.renderWorktreeList())
	default:
		return m.renderDiff()
	}
}

func (m Model) renderWithOverlay(popup string) string {
	bg := m.renderDiff()
	// For now, just show popup at top - proper overlay would dim background
	return lipgloss.JoinVertical(lipgloss.Left, popup, bg)
}

func (m Model) renderDiff() string {
	var header, footer, content string

	// Header
	branchName := ""
	if len(m.worktrees) > 0 {
		branchName = m.worktrees[m.currentWorktree].Branch
	}
	added, removed := git.ComputeStats(m.diffs)
	headerText := fmt.Sprintf("gv: %s → %s", branchName, m.mainBranch)
	statsText := fmt.Sprintf("[%d commits] ", len(m.commits))
	statsText += m.styles.StatsAdded.Render(fmt.Sprintf("+%d", added)) + " "
	statsText += m.styles.StatsRemoved.Render(fmt.Sprintf("-%d", removed))
	header = m.styles.Header.Width(m.width).Render(headerText + "  " + statsText)

	// Footer
	footerText := "j/k: scroll  c: commits  w: worktrees  u: unified  ?: help  q: quit"
	footer = m.styles.Footer.Width(m.width).Render(footerText)

	// Content
	contentHeight := m.height - 2 // Account for header and footer
	content = m.renderDiffContent(contentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m Model) renderDiffContent(height int) string {
	if len(m.diffs) == 0 {
		if m.err != nil {
			return fmt.Sprintf("Error: %v", m.err)
		}
		return "No changes (branch is same as " + m.mainBranch + ")"
	}

	var lines []string

	for _, diff := range m.diffs {
		// File header
		statsText := m.styles.StatsAdded.Render(fmt.Sprintf("+%d", diff.Added)) + " " +
			m.styles.StatsRemoved.Render(fmt.Sprintf("-%d", diff.Removed))
		path := diff.Path
		if diff.OldPath != "" {
			path = diff.OldPath + " → " + diff.Path
		}
		fileHeader := m.styles.FileHeader.Width(m.width).Render(path + "  " + statsText)
		lines = append(lines, fileHeader)

		if diff.Collapsed {
			lines = append(lines, "  (collapsed)")
			continue
		}

		if diff.IsBinary {
			lines = append(lines, "  Binary file")
			continue
		}

		// Render hunks
		for _, hunk := range diff.Hunks {
			lines = append(lines, m.renderHunk(hunk, diff.Path)...)
		}
	}

	// Apply scroll
	if m.scroll >= len(lines) {
		m.scroll = len(lines) - 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}

	start := m.scroll
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		start = len(lines)
	}

	visibleLines := lines[start:end]
	return lipgloss.JoinVertical(lipgloss.Left, visibleLines...)
}

func (m Model) renderHunk(hunk git.Hunk, filename string) []string {
	var lines []string

	if m.diffMode == DiffSideBySide {
		lines = m.renderHunkSideBySide(hunk, filename)
	} else {
		lines = m.renderHunkUnified(hunk, filename)
	}

	return lines
}

func (m Model) renderHunkUnified(hunk git.Hunk, filename string) []string {
	var lines []string

	for _, line := range hunk.Lines {
		var prefix string
		var style lipgloss.Style

		switch line.Type {
		case git.LineAdded:
			prefix = "+"
			style = m.styles.LineAdded
		case git.LineRemoved:
			prefix = "-"
			style = m.styles.LineRemoved
		default:
			prefix = " "
			style = m.styles.LineContext
		}

		lineText := prefix + line.Content
		lines = append(lines, style.Render(lineText))
	}

	return lines
}

func (m Model) renderHunkSideBySide(hunk git.Hunk, filename string) []string {
	var lines []string

	halfWidth := (m.width - 3) / 2 // -3 for separator

	// Split lines into old (left) and new (right)
	var oldLines, newLines []git.DiffLine

	for _, line := range hunk.Lines {
		switch line.Type {
		case git.LineRemoved:
			oldLines = append(oldLines, line)
		case git.LineAdded:
			newLines = append(newLines, line)
		case git.LineContext:
			// Flush any pending adds/removes
			for len(oldLines) > 0 || len(newLines) > 0 {
				var left, right string
				if len(oldLines) > 0 {
					left = m.styles.LineRemoved.Render(truncate(oldLines[0].Content, halfWidth))
					oldLines = oldLines[1:]
				}
				if len(newLines) > 0 {
					right = m.styles.LineAdded.Render(truncate(newLines[0].Content, halfWidth))
					newLines = newLines[1:]
				}
				lines = append(lines, fmt.Sprintf("%-*s │ %s", halfWidth, left, right))
			}
			// Add context line on both sides
			content := truncate(line.Content, halfWidth)
			lines = append(lines, fmt.Sprintf("%-*s │ %s", halfWidth, content, content))
		}
	}

	// Flush remaining
	for len(oldLines) > 0 || len(newLines) > 0 {
		var left, right string
		if len(oldLines) > 0 {
			left = m.styles.LineRemoved.Render(truncate(oldLines[0].Content, halfWidth))
			oldLines = oldLines[1:]
		}
		if len(newLines) > 0 {
			right = m.styles.LineAdded.Render(truncate(newLines[0].Content, halfWidth))
			newLines = newLines[1:]
		}
		lines = append(lines, fmt.Sprintf("%-*s │ %s", halfWidth, left, right))
	}

	return lines
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func (m Model) renderCommitFilter() string {
	var lines []string
	lines = append(lines, "Commits (space: toggle, a: all, n: none)")
	lines = append(lines, "")

	selected := 0
	for i, commit := range m.commits {
		checkbox := m.styles.Unselected.String()
		if commit.Selected {
			checkbox = m.styles.Selected.String()
			selected++
		}

		line := fmt.Sprintf("%s %s %s", checkbox, commit.Hash.String()[:7], commit.Subject)
		if i == m.cursor {
			line = m.styles.Cursor.Render("> " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Showing: %d of %d commits", selected, len(m.commits)))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.styles.Popup.Render(content)
}

func (m Model) renderWorktreeSwitcher() string {
	var lines []string
	lines = append(lines, "Switch Worktree")
	lines = append(lines, "")

	for i, wt := range m.worktrees {
		branch := m.styles.WorktreeBranch.Render(wt.Branch)
		path := m.styles.WorktreePath.Render(wt.Path)
		line := fmt.Sprintf("%s  %s", branch, path)

		if i == m.cursor {
			line = m.styles.Cursor.Render("> " + line)
		} else {
			line = "  " + line
		}
		if i == m.currentWorktree {
			line += " (current)"
		}
		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.styles.Popup.Render(content)
}

func (m Model) renderWorktreeList() string {
	var lines []string
	lines = append(lines, "Worktrees")
	lines = append(lines, "")

	for i, wt := range m.worktrees {
		branch := m.styles.WorktreeBranch.Render(wt.Branch)
		path := m.styles.WorktreePath.Render(wt.Path)
		line := fmt.Sprintf("%s  %s", branch, path)

		if i == m.cursor {
			line = m.styles.Cursor.Render("> " + line)
		} else {
			line = "  " + line
		}
		if i == m.currentWorktree {
			line = m.styles.WorktreeCurrent.Render(line)
		}
		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.styles.Popup.Render(content)
}

func (m Model) renderHelp() string {
	help := []struct{ key, desc string }{
		{"j/k", "Scroll"},
		{"ctrl+d/u", "Page down/up"},
		{"g/G", "Top/bottom"},
		{"n/N", "Next/prev file"},
		{"enter", "Collapse/expand file"},
		{"z", "Collapse/expand all"},
		{"u", "Toggle unified/side-by-side"},
		{"c", "Commit filter"},
		{"w", "Worktree switcher"},
		{"W", "Worktree list"},
		{"?", "This help"},
		{"q", "Quit"},
	}

	var lines []string
	lines = append(lines, "Help")
	lines = append(lines, "")

	for _, h := range help {
		key := m.styles.HelpKey.Width(12).Render(h.key)
		desc := m.styles.HelpDesc.Render(h.desc)
		lines = append(lines, key+desc)
	}

	lines = append(lines, "")
	lines = append(lines, "Press any key to close")

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.styles.Popup.Render(content)
}
