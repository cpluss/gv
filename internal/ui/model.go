package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
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

// TreeNode represents a folder or file in the file tree
type TreeNode struct {
	Name       string
	Path       string      // Full path (empty for folders)
	IsFolder   bool
	Children   []*TreeNode
	FileIdx    int         // Index in visible diffs (for files only, -1 for folders)
	Expanded   bool        // For folders: is it expanded?
	Added      int         // Aggregated stats for folders
	Removed    int
}

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

	// Folder tree state
	expandedFolders map[string]bool // Track which folders are expanded

	// Components
	styles      Styles
	highlighter *syntax.Highlighter
	spinner     spinner.Model

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
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := Model{
		styles:          DefaultStyles(),
		highlighter:     syntax.NewHighlighter(),
		spinner:         s,
		viewMode:        ViewDiff,
		diffMode:        DiffSideBySide,
		contextLines:    3, // Default context lines
		loading:         true,
		expandedFolders: make(map[string]bool),
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
	// Start spinner and load data asynchronously
	return tea.Batch(m.spinner.Tick, m.loadDataCmd())
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
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case tea.MouseMsg:
		return m.handleMouse(msg)
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
			maxScroll := m.getMaxScroll()
			if m.scroll > maxScroll {
				m.scroll = maxScroll
			}
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
		maxScroll := m.getMaxScroll()
		if m.scroll > maxScroll {
			m.scroll = maxScroll
		}
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
				// Scroll to end - calculate actual max scroll
				m.scroll = m.getMaxScroll()
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

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Only handle mouse in diff view
	if m.viewMode != ViewDiff {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if msg.X < sidebarWidth {
			// Scroll in sidebar
			if m.fileCursor > 0 {
				m.fileCursor--
			}
		} else {
			// Scroll in content
			if m.scroll > 0 {
				m.scroll -= 3
				if m.scroll < 0 {
					m.scroll = 0
				}
			}
		}
	case tea.MouseButtonWheelDown:
		if msg.X < sidebarWidth {
			// Scroll in sidebar
			visible := m.visibleDiffs()
			if m.fileCursor < len(visible)-1 {
				m.fileCursor++
			}
		} else {
			// Scroll in content
			m.scroll += 3
			maxScroll := m.getMaxScroll()
			if m.scroll > maxScroll {
				m.scroll = maxScroll
			}
		}
	case tea.MouseButtonLeft:
		// Determine click location
		// Sidebar layout: Y=0 app header, Y=1 "Files", Y=2 separator, Y=3+ tree items
		if msg.X < sidebarWidth && msg.Y >= 3 {
			itemIdx := msg.Y - 3 // Subtract app header (1) + sidebar header (2)

			// Build tree and flatten to find clicked item
			visible := m.visibleDiffs()
			tree := buildFileTree(visible, m.expandedFolders)
			var treeItems []treeItem
			flattenTree(tree, 0, &treeItems)

			if itemIdx >= 0 && itemIdx < len(treeItems) {
				if msg.Action == tea.MouseActionRelease {
					item := treeItems[itemIdx]
					m.focus = FocusSidebar

					if item.node.IsFolder {
						// Toggle folder expand/collapse
						folderPath := item.node.Path
						if m.expandedFolders[folderPath] {
							m.expandedFolders[folderPath] = false
						} else {
							m.expandedFolders[folderPath] = true
						}
					} else {
						// Select file and scroll to it
						m.fileCursor = item.node.FileIdx
						m.scrollToFile(item.node.FileIdx)
					}
				}
			}
		} else if msg.X >= sidebarWidth && msg.Action == tea.MouseActionRelease {
			// Click in content area
			m.focus = FocusContent
		}
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
		return m, tea.Batch(m.spinner.Tick, m.loadDataCmd())
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
		return m, tea.Batch(m.spinner.Tick, m.loadDataCmd())
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

// getMaxScroll calculates the maximum scroll position based on content
func (m Model) getMaxScroll() int {
	visible := m.visibleDiffs()
	totalLines := 0
	for _, diff := range visible {
		totalLines++ // File header
		if diff.Collapsed {
			totalLines++ // "(collapsed)"
		} else if diff.IsBinary {
			totalLines++ // "Binary file"
		} else {
			for _, hunk := range diff.Hunks {
				totalLines += len(hunk.Lines)
			}
		}
	}
	// Account for visible height
	contentHeight := m.height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}
	maxScroll := totalLines - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
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

// getCurrentFileAtScroll returns the file index that is visible at the current scroll position
func (m Model) getCurrentFileAtScroll() int {
	visible := m.visibleDiffs()
	if len(visible) == 0 {
		return -1
	}

	line := 0
	for i, diff := range visible {
		fileStart := line
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

		// Check if scroll position is within this file's range
		if m.scroll >= fileStart && m.scroll < line {
			return i
		}
	}

	// If scroll is beyond all content, return last file
	if len(visible) > 0 {
		return len(visible) - 1
	}
	return -1
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

	loadingText := m.spinner.View() + " Loading..."
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

	// Count commits and uncommitted separately
	commitCount := 0
	selectedCommits := 0
	hasUncommitted := false
	uncommittedSelected := false
	for _, c := range m.commits {
		if c.IsUncommitted {
			hasUncommitted = true
			uncommittedSelected = c.Selected
		} else {
			commitCount++
			if c.Selected {
				selectedCommits++
			}
		}
	}

	var commitText string
	if commitCount > 0 {
		if selectedCommits == commitCount {
			commitText = fmt.Sprintf("[%d commits", commitCount)
		} else {
			commitText = fmt.Sprintf("[%d/%d commits", selectedCommits, commitCount)
		}
		if hasUncommitted && uncommittedSelected {
			commitText += " + uncommitted"
		}
		commitText += "] "
	} else if hasUncommitted && uncommittedSelected {
		commitText = "[uncommitted] "
	} else {
		commitText = ""
	}

	statsText := commitText
	statsText += m.styles.StatsAdded.Render(fmt.Sprintf("+%d", added)) + " "
	statsText += m.styles.StatsRemoved.Render(fmt.Sprintf("-%d", removed))

	// Add current file indicator based on scroll position
	currentFileIdx := m.getCurrentFileAtScroll()
	var currentFileText string
	visible := m.visibleDiffs()
	if currentFileIdx >= 0 && currentFileIdx < len(visible) {
		currentFile := visible[currentFileIdx]
		fileName := filepath.Base(currentFile.Path)
		fileStats := m.styles.StatsAdded.Render(fmt.Sprintf("+%d", currentFile.Added)) + " " +
			m.styles.StatsRemoved.Render(fmt.Sprintf("-%d", currentFile.Removed))
		currentFileText = fmt.Sprintf("  │  %s %s", fileName, fileStats)
	}

	header = m.styles.Header.Width(m.width).Render(headerText + "  " + statsText + currentFileText)

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
	if contentHeight < 1 {
		contentHeight = 1
	}
	contentWidth := m.width - sidebarWidth - 1
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Render sidebar
	sidebar := m.renderFileSidebar(contentHeight)

	// Render diff content
	content := m.renderDiffContent(contentHeight, contentWidth)

	// Join sidebar and content horizontally
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// buildFileTree creates a tree structure from file paths
func buildFileTree(diffs []git.FileDiff, expandedFolders map[string]bool) *TreeNode {
	root := &TreeNode{
		Name:     "",
		IsFolder: true,
		Expanded: true,
		FileIdx:  -1,
		Children: make([]*TreeNode, 0),
	}

	for i, diff := range diffs {
		parts := strings.Split(diff.Path, string(filepath.Separator))
		current := root

		// Navigate/create folder path
		for j := 0; j < len(parts)-1; j++ {
			folderName := parts[j]
			folderPath := strings.Join(parts[:j+1], string(filepath.Separator))

			// Find or create folder
			var found *TreeNode
			for _, child := range current.Children {
				if child.IsFolder && child.Name == folderName {
					found = child
					break
				}
			}

			if found == nil {
				// Check if folder should be expanded (default to true for visibility)
				expanded := true
				if val, ok := expandedFolders[folderPath]; ok {
					expanded = val
				}
				found = &TreeNode{
					Name:     folderName,
					Path:     folderPath,
					IsFolder: true,
					Expanded: expanded,
					FileIdx:  -1,
					Children: make([]*TreeNode, 0),
				}
				current.Children = append(current.Children, found)
			}

			// Aggregate stats
			found.Added += diff.Added
			found.Removed += diff.Removed
			current = found
		}

		// Add file node
		fileName := parts[len(parts)-1]
		fileNode := &TreeNode{
			Name:     fileName,
			Path:     diff.Path,
			IsFolder: false,
			FileIdx:  i,
			Added:    diff.Added,
			Removed:  diff.Removed,
		}
		current.Children = append(current.Children, fileNode)
	}

	return root
}

// flattenTree returns a flat list of visible tree items with their indentation level and file index
type treeItem struct {
	node   *TreeNode
	indent int
}

func flattenTree(node *TreeNode, indent int, items *[]treeItem) {
	for _, child := range node.Children {
		*items = append(*items, treeItem{node: child, indent: indent})
		if child.IsFolder && child.Expanded {
			flattenTree(child, indent+1, items)
		}
	}
}

// getDisplayNames returns display names for files, adding path context for duplicates
func getDisplayNames(diffs []git.FileDiff) map[string]string {
	result := make(map[string]string)

	// Group files by basename
	byBasename := make(map[string][]string)
	for _, d := range diffs {
		base := filepath.Base(d.Path)
		byBasename[base] = append(byBasename[base], d.Path)
	}

	// For each file, determine the display name
	for _, d := range diffs {
		base := filepath.Base(d.Path)
		paths := byBasename[base]

		if len(paths) == 1 {
			// No duplicates, just use basename
			result[d.Path] = base
		} else {
			// Find shortest unique suffix for disambiguation
			result[d.Path] = getShortestUniquePath(d.Path, paths)
		}
	}

	return result
}

// getShortestUniquePath finds the shortest path suffix that uniquely identifies this file
func getShortestUniquePath(path string, allPaths []string) string {
	parts := strings.Split(path, string(filepath.Separator))

	// Start from just the filename and add parent dirs until unique
	for i := len(parts) - 1; i >= 0; i-- {
		suffix := filepath.Join(parts[i:]...)
		isUnique := true
		for _, other := range allPaths {
			if other == path {
				continue
			}
			if strings.HasSuffix(other, suffix) || strings.HasSuffix(other, string(filepath.Separator)+suffix) {
				isUnique = false
				break
			}
		}
		if isUnique {
			return suffix
		}
	}
	// Fallback to full path
	return path
}

func (m Model) renderFileSidebar(height int) string {
	// Ensure height is positive
	if height < 1 {
		height = 1
	}

	var lines []string

	visible := m.visibleDiffs()
	hiddenCount := len(m.diffs) - len(visible)

	// Build file tree
	tree := buildFileTree(visible, m.expandedFolders)

	// Flatten tree to visible items
	var treeItems []treeItem
	flattenTree(tree, 0, &treeItems)

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

	// Track file index for cursor matching
	fileCount := 0
	for _, item := range treeItems {
		node := item.node
		indent := strings.Repeat("  ", item.indent)

		var line string
		var statsStyled string
		maxNameLen := sidebarWidth - 8 - len(indent)

		if node.IsFolder {
			// Folder with expand/collapse indicator
			indicator := "▼"
			if !node.Expanded {
				indicator = "▶"
			}
			name := node.Name
			if len(name) > maxNameLen {
				name = name[:maxNameLen-3] + "..."
			}
			// Folder styling with dimmed color
			folderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
			line = indent + indicator + " " + folderStyle.Render(name+"/")
			// Aggregate stats for folder
			statsStyled = m.styles.StatsAdded.Render(fmt.Sprintf("+%d", node.Added)) + " " +
				m.styles.StatsRemoved.Render(fmt.Sprintf("-%d", node.Removed))
		} else {
			// File with collapse indicator for diff content
			diff := visible[node.FileIdx]
			indicator := "▼"
			if diff.Collapsed {
				indicator = "▶"
			}
			name := node.Name
			if len(name) > maxNameLen {
				name = name[:maxNameLen-3] + "..."
			}

			// Highlight current file
			if node.FileIdx == m.fileCursor {
				if m.focus == FocusSidebar {
					line = m.styles.Cursor.Render(indent + "> " + indicator + " " + name)
				} else {
					line = indent + "> " + indicator + " " + name
				}
			} else {
				line = indent + "  " + indicator + " " + name
			}

			statsStyled = m.styles.StatsAdded.Render(fmt.Sprintf("+%d", node.Added)) + " " +
				m.styles.StatsRemoved.Render(fmt.Sprintf("-%d", node.Removed))
			fileCount++
		}

		// Pad line and add stats
		stats := fmt.Sprintf("+%d -%d", node.Added, node.Removed)
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
		// Count actual commits (non-uncommitted)
		commitCount := 0
		for _, c := range m.commits {
			if !c.IsUncommitted && c.Selected {
				commitCount++
			}
		}
		if commitCount > 0 {
			return fmt.Sprintf("%d commit(s) selected, but no file changes\n(press 'c' to view commits)", commitCount)
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

	// Apply scroll with proper bounds checking
	scroll := m.scroll
	if scroll < 0 {
		scroll = 0
	}
	if len(lines) > 0 && scroll >= len(lines) {
		scroll = len(lines) - 1
	}

	// Ensure height is positive
	if height <= 0 {
		height = 1
	}

	start := scroll
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	if start >= len(lines) {
		start = 0
		if len(lines) > 0 {
			start = len(lines) - 1
		}
		end = len(lines)
	}
	if start >= end {
		return ""
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

	// Collect all line contents for batch highlighting
	var contents []string
	for _, line := range hunk.Lines {
		contents = append(contents, line.Content)
	}

	// Get syntax highlighting for all lines at once
	highlighted := m.highlighter.HighlightLines(filename, contents)

	for i, line := range hunk.Lines {
		var prefix string
		var prefixStyle lipgloss.Style
		var bgStyle lipgloss.Style
		var lineNum string

		switch line.Type {
		case git.LineAdded:
			prefix = "+"
			prefixStyle = m.styles.StatsAdded // Green prefix
			bgStyle = m.styles.AddedBg        // Subtle green background
			lineNum = fmt.Sprintf("    %4d ", line.NewNum)
		case git.LineRemoved:
			prefix = "-"
			prefixStyle = m.styles.StatsRemoved // Red prefix
			bgStyle = m.styles.RemovedBg        // Subtle red background
			lineNum = fmt.Sprintf("%4d     ", line.OldNum)
		default:
			prefix = " "
			prefixStyle = m.styles.LineContext
			bgStyle = lipgloss.NewStyle()
			lineNum = fmt.Sprintf("%4d %4d ", line.OldNum, line.NewNum)
		}

		lineNumStyled := m.styles.LineNumber.Render(lineNum)
		prefixStyled := prefixStyle.Render(prefix)

		// Render syntax-highlighted content
		var contentParts []string
		if i < len(highlighted) {
			for _, token := range highlighted[i].Tokens {
				tokenStyle := lipgloss.NewStyle()
				if token.Style.Color != "" {
					tokenStyle = tokenStyle.Foreground(lipgloss.Color(token.Style.Color))
				}
				if token.Style.Bold {
					tokenStyle = tokenStyle.Bold(true)
				}
				if token.Style.Italic {
					tokenStyle = tokenStyle.Italic(true)
				}
				contentParts = append(contentParts, tokenStyle.Render(token.Text))
			}
		}

		var content string
		if len(contentParts) > 0 {
			content = strings.Join(contentParts, "")
		} else {
			content = line.Content
		}

		// Apply background to entire line content (prefix + content)
		lineContent := prefixStyled + content
		if line.Type != git.LineContext {
			lineContent = bgStyle.Render(lineContent)
		}

		lines = append(lines, lineNumStyled+lineContent)
	}

	return lines
}

func (m Model) renderHunkSideBySideWithWidth(hunk git.Hunk, filename string, width int) []string {
	var lines []string

	lineNumWidth := 5 // "1234 " format
	halfWidth := (width - 3 - lineNumWidth*2) / 2 // -3 for separator, -10 for line numbers

	// Collect all line contents for syntax highlighting
	var allContents []string
	var contentIndices []int // Maps hunk line index to allContents index
	for _, line := range hunk.Lines {
		contentIndices = append(contentIndices, len(allContents))
		allContents = append(allContents, line.Content)
	}

	// Get syntax highlighting for all lines
	highlighted := m.highlighter.HighlightLines(filename, allContents)

	// Helper to render syntax-highlighted content
	renderSyntaxContent := func(contentIdx int, content string) string {
		if contentIdx < 0 || contentIdx >= len(highlighted) {
			return content
		}
		var parts []string
		for _, token := range highlighted[contentIdx].Tokens {
			tokenStyle := lipgloss.NewStyle()
			if token.Style.Color != "" {
				tokenStyle = tokenStyle.Foreground(lipgloss.Color(token.Style.Color))
			}
			if token.Style.Bold {
				tokenStyle = tokenStyle.Bold(true)
			}
			if token.Style.Italic {
				tokenStyle = tokenStyle.Italic(true)
			}
			parts = append(parts, tokenStyle.Render(token.Text))
		}
		if len(parts) > 0 {
			return strings.Join(parts, "")
		}
		return content
	}

	// Helper to render a side-by-side line with proper alignment
	renderLine := func(leftNum int, leftContent string, leftContentIdx int, isLeftRemoved bool,
		rightNum int, rightContent string, rightContentIdx int, isRightAdded bool) string {
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

		// Render syntax-highlighted content
		var leftRendered, rightRendered string
		truncatedLeft := truncate(leftContent, halfWidth-1) // -1 for gutter
		truncatedRight := truncate(rightContent, halfWidth-1)

		if isLeftRemoved {
			// Red gutter + syntax highlighted content with red background
			gutter := m.styles.StatsRemoved.Render("-")
			syntaxContent := renderSyntaxContent(leftContentIdx, truncatedLeft)
			leftRendered = m.styles.RemovedBg.Width(halfWidth).Render(gutter + syntaxContent)
		} else if leftContent != "" {
			// Context line
			leftRendered = lipgloss.NewStyle().Width(halfWidth).Render(" " + renderSyntaxContent(leftContentIdx, truncatedLeft))
		} else {
			leftRendered = lipgloss.NewStyle().Width(halfWidth).Render("")
		}

		if isRightAdded {
			// Green gutter + syntax highlighted content with green background
			gutter := m.styles.StatsAdded.Render("+")
			syntaxContent := renderSyntaxContent(rightContentIdx, truncatedRight)
			rightRendered = m.styles.AddedBg.Width(halfWidth).Render(gutter + syntaxContent)
		} else if rightContent != "" {
			// Context line
			rightRendered = lipgloss.NewStyle().Width(halfWidth).Render(" " + renderSyntaxContent(rightContentIdx, truncatedRight))
		} else {
			rightRendered = lipgloss.NewStyle().Width(halfWidth).Render("")
		}

		return leftNumStyled + leftRendered + " │ " + rightNumStyled + rightRendered
	}

	// Split lines into old (left) and new (right) with their indices
	type indexedLine struct {
		line git.DiffLine
		idx  int
	}
	var oldLines, newLines []indexedLine

	for i, line := range hunk.Lines {
		switch line.Type {
		case git.LineRemoved:
			oldLines = append(oldLines, indexedLine{line, contentIndices[i]})
		case git.LineAdded:
			newLines = append(newLines, indexedLine{line, contentIndices[i]})
		case git.LineContext:
			// Flush any pending adds/removes
			for len(oldLines) > 0 || len(newLines) > 0 {
				var leftContent, rightContent string
				var leftNum, rightNum int
				var leftIdx, rightIdx int = -1, -1
				var isLeftRemoved, isRightAdded bool

				if len(oldLines) > 0 {
					leftContent = oldLines[0].line.Content
					leftNum = oldLines[0].line.OldNum
					leftIdx = oldLines[0].idx
					isLeftRemoved = true
					oldLines = oldLines[1:]
				}

				if len(newLines) > 0 {
					rightContent = newLines[0].line.Content
					rightNum = newLines[0].line.NewNum
					rightIdx = newLines[0].idx
					isRightAdded = true
					newLines = newLines[1:]
				}

				lines = append(lines, renderLine(leftNum, leftContent, leftIdx, isLeftRemoved,
					rightNum, rightContent, rightIdx, isRightAdded))
			}
			// Add context line on both sides
			lines = append(lines, renderLine(line.OldNum, line.Content, contentIndices[i], false,
				line.NewNum, line.Content, contentIndices[i], false))
		}
	}

	// Flush remaining
	for len(oldLines) > 0 || len(newLines) > 0 {
		var leftContent, rightContent string
		var leftNum, rightNum int
		var leftIdx, rightIdx int = -1, -1
		var isLeftRemoved, isRightAdded bool

		if len(oldLines) > 0 {
			leftContent = oldLines[0].line.Content
			leftNum = oldLines[0].line.OldNum
			leftIdx = oldLines[0].idx
			isLeftRemoved = true
			oldLines = oldLines[1:]
		}

		if len(newLines) > 0 {
			rightContent = newLines[0].line.Content
			rightNum = newLines[0].line.NewNum
			rightIdx = newLines[0].idx
			isRightAdded = true
			newLines = newLines[1:]
		}

		lines = append(lines, renderLine(leftNum, leftContent, leftIdx, isLeftRemoved,
			rightNum, rightContent, rightIdx, isRightAdded))
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
