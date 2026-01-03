package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// Config holds CLI/config file options
type Config struct {
	BaseBranch string
}

// dataLoadedMsg is sent when async data loading completes
type dataLoadedMsg struct {
	commits []git.Commit
	diffs   []git.FileDiff
	err     error
}

// FocusArea represents which pane has focus
type FocusArea int

const (
	FocusSidebar FocusArea = iota
	FocusContent
)

const sidebarWidth = 35

// hiddenPatterns are file patterns hidden by default
var hiddenPatterns = []string{
	"go.sum",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"Cargo.lock",
	"Gemfile.lock",
	"poetry.lock",
	"composer.lock",
	".pnp.cjs",
	".pnp.loader.mjs",
}

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
	loading         bool // True while loading data asynchronously

	// View state
	viewMode     ViewMode
	diffMode     DiffMode
	scroll       int
	cursor       int // For popups
	fileCursor   int // For file sidebar
	focus        FocusArea
	showHidden   bool   // Show hidden/noisy files
	numBuffer    string // Buffer for number prefixes like "10G"
	contextLines int    // Context lines for diff (0, 1, or 3)

	// Filter state
	filterInput string

	// Components
	styles      Styles
	highlighter *syntax.Highlighter

	// Error state
	err error
}

// InitModel creates a new model from the current directory with default config
func InitModel() (Model, error) {
	return InitModelWithConfig(Config{})
}

// InitModelWithConfig creates a new model with the given configuration.
// This returns immediately with a loading state; data is loaded async in Init().
func InitModelWithConfig(cfg Config) (Model, error) {
	m := Model{
		styles:       DefaultStyles(),
		highlighter:  syntax.NewHighlighter(),
		viewMode:     ViewDiff,
		diffMode:     DiffSideBySide,
		contextLines: 3, // Default context lines
		loading:      true,
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return m, fmt.Errorf("getting current directory: %w", err)
	}

	// Find git root (fast - just walks up looking for .git)
	repoPath, err := findGitRoot(cwd)
	if err != nil {
		return m, fmt.Errorf("finding git root: %w", err)
	}
	m.repoPath = repoPath

	// Discover worktrees (fast - single git command)
	worktrees, err := git.ListWorktrees(repoPath)
	if err != nil {
		return m, fmt.Errorf("listing worktrees: %w", err)
	}
	m.worktrees = worktrees
	m.currentWorktree = git.FindCurrentWorktree(worktrees, cwd)

	// Get main branch - use config override if provided (fast)
	if cfg.BaseBranch != "" {
		m.mainBranch = cfg.BaseBranch
	} else {
		m.mainBranch = git.GetMainBranch(repoPath)
	}

	// Data loading happens asynchronously in Init()
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

	// Load diffs with current context setting
	diffs, err := git.ComputeDiffWithContext(wt.Path, m.mainBranch, commits, m.contextLines)
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
	// Load data asynchronously
	return m.loadDataCmd()
}

// loadDataCmd returns a command that loads commits and diffs asynchronously
func (m Model) loadDataCmd() tea.Cmd {
	return func() tea.Msg {
		if len(m.worktrees) == 0 {
			return dataLoadedMsg{}
		}

		wt := m.worktrees[m.currentWorktree]

		// Load commits
		commits, err := git.ListCommits(wt.Path, m.mainBranch)
		if err != nil {
			// Not an error if we're on the main branch
			commits = nil
		}

		// Load diffs with current context setting
		diffs, err := git.ComputeDiffWithContext(wt.Path, m.mainBranch, commits, m.contextLines)
		if err != nil {
			return dataLoadedMsg{err: err}
		}

		return dataLoadedMsg{
			commits: commits,
			diffs:   diffs,
		}
	}
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dataLoadedMsg:
		m.loading = false
		m.commits = msg.commits
		m.diffs = msg.diffs
		m.err = msg.err
		return m, nil
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
	key := msg.String()

	// Collect digits into number buffer
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		m.numBuffer += key
		return m, nil
	}

	// Get numeric prefix if any, then clear buffer
	numPrefix := 0
	if m.numBuffer != "" {
		numPrefix, _ = strconv.Atoi(m.numBuffer)
		m.numBuffer = ""
	}

	switch key {
	case "tab":
		// Toggle focus between sidebar and content
		if m.focus == FocusSidebar {
			m.focus = FocusContent
		} else {
			m.focus = FocusSidebar
		}
	case "j", "down":
		count := 1
		if numPrefix > 0 {
			count = numPrefix
		}
		if m.focus == FocusSidebar {
			visible := m.visibleDiffs()
			m.fileCursor += count
			if m.fileCursor >= len(visible) {
				m.fileCursor = len(visible) - 1
			}
			if m.fileCursor < 0 {
				m.fileCursor = 0
			}
		} else {
			m.scroll += count
		}
	case "k", "up":
		count := 1
		if numPrefix > 0 {
			count = numPrefix
		}
		if m.focus == FocusSidebar {
			m.fileCursor -= count
			if m.fileCursor < 0 {
				m.fileCursor = 0
			}
		} else {
			m.scroll -= count
			if m.scroll < 0 {
				m.scroll = 0
			}
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
		if m.focus == FocusSidebar {
			m.fileCursor = 0
		} else {
			m.scroll = 0
		}
	case "G":
		if m.focus == FocusSidebar {
			visible := m.visibleDiffs()
			if numPrefix > 0 && numPrefix <= len(visible) {
				m.fileCursor = numPrefix - 1 // 1-indexed to 0-indexed
			} else if len(visible) > 0 {
				m.fileCursor = len(visible) - 1
			}
		} else {
			if numPrefix > 0 {
				// Go to specific line
				m.scroll = numPrefix - 1 // 1-indexed to 0-indexed
			} else {
				// Scroll to end - will be clamped in View
				m.scroll = 99999
			}
		}
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
	case " ":
		// Space toggles collapse for file under cursor in sidebar
		if m.focus == FocusSidebar {
			visible := m.visibleDiffs()
			if m.fileCursor < len(visible) {
				// Find the actual index in m.diffs for this visible file
				targetPath := visible[m.fileCursor].Path
				for i := range m.diffs {
					if m.diffs[i].Path == targetPath {
						m.diffs[i].Collapsed = !m.diffs[i].Collapsed
						break
					}
				}
			}
		}
	case "enter":
		visible := m.visibleDiffs()
		if m.focus == FocusSidebar && m.fileCursor < len(visible) {
			// Jump to this file in content view
			m.scrollToFile(m.fileCursor)
			m.focus = FocusContent
		} else {
			m.toggleCurrentFile()
		}
	case "z":
		m.toggleAllFiles()
	case "h":
		m.showHidden = !m.showHidden
		// Clamp file cursor to visible range
		visible := m.visibleDiffs()
		if m.fileCursor >= len(visible) {
			m.fileCursor = len(visible) - 1
			if m.fileCursor < 0 {
				m.fileCursor = 0
			}
		}
	case "x":
		// Toggle context lines: 3 -> 1 -> 0 -> 3
		switch m.contextLines {
		case 3:
			m.contextLines = 1
		case 1:
			m.contextLines = 0
		default:
			m.contextLines = 3
		}
		m.recomputeDiff()
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
		m.loading = true
		m.viewMode = ViewDiff
		m.scroll = 0
		return m, m.loadDataCmd()
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
		m.loading = true
		m.viewMode = ViewDiff
		m.scroll = 0
		return m, m.loadDataCmd()
	case "esc":
		m.viewMode = ViewDiff
	}
	return m, nil
}

func (m *Model) recomputeDiff() {
	wt := m.worktrees[m.currentWorktree]
	diffs, err := git.ComputeDiffWithContext(wt.Path, m.mainBranch, m.commits, m.contextLines)
	if err != nil {
		m.err = err
		return
	}
	m.diffs = diffs
}

// isHiddenFile checks if a file matches hidden patterns
func isHiddenFile(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range hiddenPatterns {
		if base == pattern {
			return true
		}
	}
	return false
}

// visibleDiffs returns diffs filtered by showHidden setting
func (m Model) visibleDiffs() []git.FileDiff {
	if m.showHidden {
		return m.diffs
	}
	var visible []git.FileDiff
	for _, d := range m.diffs {
		if !isHiddenFile(d.Path) {
			visible = append(visible, d)
		}
	}
	return visible
}

func (m *Model) nextFile() {
	visible := m.visibleDiffs()
	if m.fileCursor < len(visible)-1 {
		m.fileCursor++
		m.scrollToFile(m.fileCursor)
	}
}

func (m *Model) prevFile() {
	if m.fileCursor > 0 {
		m.fileCursor--
		m.scrollToFile(m.fileCursor)
	}
}

func (m *Model) scrollToFile(fileIdx int) {
	visible := m.visibleDiffs()
	// Calculate scroll position for given file
	line := 0
	for i, diff := range visible {
		if i == fileIdx {
			m.scroll = line
			return
		}
		line++ // File header
		if !diff.Collapsed && !diff.IsBinary {
			for _, hunk := range diff.Hunks {
				line += len(hunk.Lines)
			}
		} else if diff.IsBinary {
			line++ // "Binary file" line
		} else {
			line++ // "(collapsed)" line
		}
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

	if m.loading {
		return m.renderLoading()
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

func (m Model) renderLoading() string {
	branchName := ""
	if len(m.worktrees) > 0 {
		branchName = m.worktrees[m.currentWorktree].Branch
	}
	headerText := fmt.Sprintf("gv: %s → %s", branchName, m.mainBranch)
	header := m.styles.Header.Width(m.width).Render(headerText)

	loadingText := "Loading..."
	content := lipgloss.Place(m.width, m.height-2, lipgloss.Center, lipgloss.Center, loadingText)

	footer := m.styles.Footer.Width(m.width).Render("q: quit")

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m Model) renderWithOverlay(popup string) string {
	bg := m.renderDiff()
	bgLines := strings.Split(bg, "\n")
	popupLines := strings.Split(popup, "\n")

	popupHeight := len(popupLines)
	popupWidth := lipgloss.Width(popup)

	// Calculate top-left position to center the popup
	startY := (m.height - popupHeight) / 2
	startX := (m.width - popupWidth) / 2

	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Overlay popup onto background
	for i, popupLine := range popupLines {
		bgY := startY + i
		if bgY >= len(bgLines) {
			break
		}

		// Build new line: left padding + popup line + right remainder
		bgLine := bgLines[bgY]
		bgRunes := []rune(bgLine)

		// Pad background line if needed
		for len(bgRunes) < startX {
			bgRunes = append(bgRunes, ' ')
		}

		// Create new line with popup overlaid
		newLine := string(bgRunes[:startX]) + popupLine
		bgLines[bgY] = newLine
	}

	return strings.Join(bgLines, "\n")
}

func (m Model) renderDiff() string {
	var header, footer string

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
	focusHint := "Tab: switch pane"
	if m.focus == FocusSidebar {
		focusHint = "[Sidebar] " + focusHint
	} else {
		focusHint = "[Content] " + focusHint
	}
	footerText := focusHint + "  j/k: scroll  c: commits  w: worktrees  u: unified  ?: help  q: quit"
	footer = m.styles.Footer.Width(m.width).Render(footerText)

	// Content area with sidebar
	contentHeight := m.height - 2 // Account for header and footer
	contentWidth := m.width - sidebarWidth - 1

	// Render sidebar
	sidebar := m.renderFileSidebar(contentHeight)

	// Render diff content
	content := m.renderDiffContent(contentHeight, contentWidth)

	// Join sidebar and content horizontally
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) renderFileSidebar(height int) string {
	var lines []string

	visible := m.visibleDiffs()
	hiddenCount := len(m.diffs) - len(visible)

	// Sidebar header
	title := "Files"
	if hiddenCount > 0 && !m.showHidden {
		title = fmt.Sprintf("Files (%d hidden)", hiddenCount)
	}
	if m.focus == FocusSidebar {
		title = m.styles.Cursor.Render(title)
	}
	lines = append(lines, title)
	lines = append(lines, strings.Repeat("─", sidebarWidth-2))

	for i, diff := range visible {
		// Collapse indicator
		indicator := "▼"
		if diff.Collapsed {
			indicator = "▶"
		}

		// File name (basename only for brevity)
		name := filepath.Base(diff.Path)
		if len(name) > sidebarWidth-8 {
			name = name[:sidebarWidth-11] + "..."
		}

		// Stats
		stats := fmt.Sprintf("+%d -%d", diff.Added, diff.Removed)

		line := fmt.Sprintf("%s %s", indicator, name)

		// Highlight current file
		if i == m.fileCursor {
			if m.focus == FocusSidebar {
				line = m.styles.Cursor.Render("> " + line)
			} else {
				line = "> " + line
			}
		} else {
			line = "  " + line
		}

		// Add stats with color
		statsStyled := m.styles.StatsAdded.Render(fmt.Sprintf("+%d", diff.Added)) + " " +
			m.styles.StatsRemoved.Render(fmt.Sprintf("-%d", diff.Removed))
		// Pad line and add stats
		padding := sidebarWidth - lipgloss.Width(line) - lipgloss.Width(stats) - 1
		if padding > 0 {
			line += strings.Repeat(" ", padding) + statsStyled
		}

		lines = append(lines, line)
	}

	// Fill remaining height
	for len(lines) < height {
		lines = append(lines, "")
	}

	// Truncate if too many files
	if len(lines) > height {
		lines = lines[:height]
	}

	// Apply sidebar style
	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true).
		BorderForeground(lipgloss.Color("238"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return sidebarStyle.Render(content)
}

func (m Model) renderDiffContent(height int, width int) string {
	visible := m.visibleDiffs()
	if len(visible) == 0 {
		if m.err != nil {
			return fmt.Sprintf("Error: %v", m.err)
		}
		if len(m.diffs) > 0 {
			return fmt.Sprintf("All %d files hidden (press 'h' to show)", len(m.diffs))
		}
		return "No changes (branch is same as " + m.mainBranch + ")"
	}

	var lines []string

	for _, diff := range visible {
		// File header
		statsText := m.styles.StatsAdded.Render(fmt.Sprintf("+%d", diff.Added)) + " " +
			m.styles.StatsRemoved.Render(fmt.Sprintf("-%d", diff.Removed))
		path := diff.Path
		if diff.OldPath != "" {
			path = diff.OldPath + " → " + diff.Path
		}
		fileHeader := m.styles.FileHeader.Width(width).Render(path + "  " + statsText)
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
			lines = append(lines, m.renderHunkWithWidth(hunk, diff.Path, width)...)
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
	return m.renderHunkWithWidth(hunk, filename, m.width)
}

func (m Model) renderHunkWithWidth(hunk git.Hunk, filename string, width int) []string {
	var lines []string

	if m.diffMode == DiffSideBySide {
		lines = m.renderHunkSideBySideWithWidth(hunk, filename, width)
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
		var lineNum string

		switch line.Type {
		case git.LineAdded:
			prefix = "+"
			style = m.styles.LineAdded
			lineNum = fmt.Sprintf("    %4d ", line.NewNum)
		case git.LineRemoved:
			prefix = "-"
			style = m.styles.LineRemoved
			lineNum = fmt.Sprintf("%4d     ", line.OldNum)
		default:
			prefix = " "
			style = m.styles.LineContext
			lineNum = fmt.Sprintf("%4d %4d ", line.OldNum, line.NewNum)
		}

		lineNumStyled := m.styles.LineNumber.Render(lineNum)
		lineText := prefix + line.Content
		lines = append(lines, lineNumStyled+style.Render(lineText))
	}

	return lines
}

func (m Model) renderHunkSideBySideWithWidth(hunk git.Hunk, filename string, width int) []string {
	var lines []string

	lineNumWidth := 5 // "1234 " format
	halfWidth := (width - 3 - lineNumWidth*2) / 2 // -3 for separator, -10 for line numbers

	// Helper to render a side-by-side line with proper alignment
	renderLine := func(leftNum int, leftContent string, leftStyle lipgloss.Style, rightNum int, rightContent string, rightStyle lipgloss.Style) string {
		// Format line numbers
		var leftNumStr, rightNumStr string
		if leftNum > 0 {
			leftNumStr = fmt.Sprintf("%4d ", leftNum)
		} else {
			leftNumStr = "     "
		}
		if rightNum > 0 {
			rightNumStr = fmt.Sprintf("%4d ", rightNum)
		} else {
			rightNumStr = "     "
		}

		leftNumStyled := m.styles.LineNumber.Render(leftNumStr)
		rightNumStyled := m.styles.LineNumber.Render(rightNumStr)

		// Pad and style each side separately using lipgloss Width for ANSI-aware padding
		left := leftStyle.Width(halfWidth).Render(truncate(leftContent, halfWidth))
		right := rightStyle.Width(halfWidth).Render(truncate(rightContent, halfWidth))
		return leftNumStyled + left + " │ " + rightNumStyled + right
	}

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
				var leftContent, rightContent string
				var leftStyle, rightStyle lipgloss.Style
				var leftNum, rightNum int

				if len(oldLines) > 0 {
					leftContent = oldLines[0].Content
					leftNum = oldLines[0].OldNum
					leftStyle = m.styles.LineRemoved
					oldLines = oldLines[1:]
				} else {
					leftStyle = m.styles.LineContext
				}

				if len(newLines) > 0 {
					rightContent = newLines[0].Content
					rightNum = newLines[0].NewNum
					rightStyle = m.styles.LineAdded
					newLines = newLines[1:]
				} else {
					rightStyle = m.styles.LineContext
				}

				lines = append(lines, renderLine(leftNum, leftContent, leftStyle, rightNum, rightContent, rightStyle))
			}
			// Add context line on both sides
			lines = append(lines, renderLine(line.OldNum, line.Content, m.styles.LineContext, line.NewNum, line.Content, m.styles.LineContext))
		}
	}

	// Flush remaining
	for len(oldLines) > 0 || len(newLines) > 0 {
		var leftContent, rightContent string
		var leftStyle, rightStyle lipgloss.Style
		var leftNum, rightNum int

		if len(oldLines) > 0 {
			leftContent = oldLines[0].Content
			leftNum = oldLines[0].OldNum
			leftStyle = m.styles.LineRemoved
			oldLines = oldLines[1:]
		} else {
			leftStyle = m.styles.LineContext
		}

		if len(newLines) > 0 {
			rightContent = newLines[0].Content
			rightNum = newLines[0].NewNum
			rightStyle = m.styles.LineAdded
			newLines = newLines[1:]
		} else {
			rightStyle = m.styles.LineContext
		}

		lines = append(lines, renderLine(leftNum, leftContent, leftStyle, rightNum, rightContent, rightStyle))
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

		var line string
		if commit.IsUncommitted {
			line = fmt.Sprintf("%s %s", checkbox, commit.Subject)
		} else {
			line = fmt.Sprintf("%s %s %s", checkbox, commit.Hash.String()[:7], commit.Subject)
		}

		if i == m.cursor {
			line = m.styles.Cursor.Render("> " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Showing: %d of %d", selected, len(m.commits)))

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
		{"Tab", "Switch sidebar/content focus"},
		{"j/k", "Navigate (supports number prefix like 10j)"},
		{"ctrl+d/u", "Page down/up"},
		{"g/G", "Top/bottom (supports number prefix like 50G)"},
		{"n/N", "Next/prev file"},
		{"space", "Fold/unfold file (in sidebar)"},
		{"enter", "Jump to file (in sidebar)"},
		{"z", "Collapse/expand all"},
		{"h", "Toggle hidden files (lock files, etc.)"},
		{"x", "Toggle context lines (3/1/0)"},
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
