//! Main application struct and event loop
//!
//! Contains the App struct with all application state,
//! and the main event loop for handling input and rendering.

use std::collections::HashMap;
use std::io;
use std::path::PathBuf;

use anyhow::Result;
use crossterm::{
    event::{self, Event, KeyCode, KeyEvent, KeyModifiers, MouseButton, MouseEvent, MouseEventKind},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout, Rect},
    Terminal,
};

use crate::git::{self, Commit, FileDiff, Worktree};
use crate::syntax::Highlighter;
use crate::ui::{
    DiffMode, FocusArea, Styles, TreeNode,
    build_file_tree, flatten_tree,
    render_diff_content, render_footer, render_header, render_sidebar,
    render_commit_popup, render_worktree_popup, render_help_popup,
    diff_view::calculate_total_lines,
    sidebar::SIDEBAR_WIDTH,
};

/// View mode for the application
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ViewMode {
    /// Main diff view
    Diff,
    /// Commit filter popup
    CommitFilter,
    /// Worktree switcher popup
    WorktreeSwitcher,
    /// Worktree list view
    WorktreeList,
    /// Help overlay
    Help,
}

/// Hidden file patterns (lock files, etc.)
const HIDDEN_PATTERNS: &[&str] = &[
    "go.sum",
    "package-lock.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "Cargo.lock",
    "Gemfile.lock",
    "poetry.lock",
    "composer.lock",
];

const MOUSE_SCROLL_LINES: i32 = 5;

/// Main application state
pub struct App {
    // Window dimensions
    width: u16,
    height: u16,

    // Repository
    repo_path: PathBuf,
    main_branch: String,

    // Worktrees
    worktrees: Vec<Worktree>,
    current_worktree: usize,

    // Commits
    commits: Vec<Commit>,

    // Diffs
    diffs: Vec<FileDiff>,
    visible_diffs: Vec<usize>, // Indices into diffs

    // File tree
    file_tree: Vec<TreeNode>,
    expanded_folders: HashMap<String, bool>,

    // View state
    view_mode: ViewMode,
    diff_mode: DiffMode,
    focus: FocusArea,

    // Scroll positions
    content_scroll: usize,
    sidebar_scroll: usize,
    file_cursor: usize,
    popup_cursor: usize,

    // Options
    show_hidden: bool,
    context_lines: u32,

    // Filter input (for worktree switcher)
    filter_input: String,

    // Number prefix for vim-style jumps
    number_prefix: Option<usize>,

    // Styling and highlighting
    styles: Styles,
    highlighter: Highlighter,

    // Loading state
    loading: bool,
    error: Option<String>,
}

impl App {
    /// Create a new App instance
    pub fn new(repo_path: PathBuf, base_branch: Option<String>) -> Result<Self> {
        // Discover the main branch
        let main_branch = base_branch
            .unwrap_or_else(|| git::get_main_branch(&repo_path).unwrap_or_else(|_| "main".to_string()));

        let mut app = Self {
            width: 0,
            height: 0,
            repo_path,
            main_branch,
            worktrees: Vec::new(),
            current_worktree: 0,
            commits: Vec::new(),
            diffs: Vec::new(),
            visible_diffs: Vec::new(),
            file_tree: Vec::new(),
            expanded_folders: HashMap::new(),
            view_mode: ViewMode::Diff,
            diff_mode: DiffMode::SideBySide,
            focus: FocusArea::Content,
            content_scroll: 0,
            sidebar_scroll: 0,
            file_cursor: 0,
            popup_cursor: 0,
            show_hidden: false,
            context_lines: 3,
            filter_input: String::new(),
            number_prefix: None,
            styles: Styles::new(),
            highlighter: Highlighter::new(),
            loading: true,
            error: None,
        };

        // Load initial data
        app.load_data()?;

        Ok(app)
    }

    /// Load/reload data from the repository
    fn load_data(&mut self) -> Result<()> {
        self.loading = true;
        self.error = None;

        // Load worktrees
        self.worktrees = git::list_worktrees(&self.repo_path).unwrap_or_default();
        git::find_current_worktree(&mut self.worktrees, &self.repo_path);

        // Find current worktree index
        self.current_worktree = self.worktrees
            .iter()
            .position(|w| w.is_current)
            .unwrap_or(0);

        // Load commits
        self.commits = git::list_commits(&self.repo_path, &self.main_branch).unwrap_or_default();

        // Load diffs
        self.reload_diffs()?;

        self.loading = false;
        Ok(())
    }

    /// Reload diffs based on current commit selection
    fn reload_diffs(&mut self) -> Result<()> {
        let include_uncommitted = self.commits
            .iter()
            .any(|c| c.is_uncommitted && c.selected);

        let selected_hashes: Vec<String> = self.commits
            .iter()
            .filter(|c| c.selected && !c.is_uncommitted)
            .map(|c| c.full_hash.clone())
            .collect();

        self.diffs = git::compute_diff(
            &self.repo_path,
            &self.main_branch,
            include_uncommitted,
            &selected_hashes,
            self.context_lines,
        ).unwrap_or_default();

        // Rebuild file tree
        self.file_tree = build_file_tree(&self.diffs, &self.expanded_folders);
        self.set_sidebar_cursor(self.file_cursor);

        // Update visible diffs
        self.update_visible_diffs();

        // Clear highlight cache when diffs change
        self.highlighter.clear_cache();
        self.prime_highlight_cache();

        Ok(())
    }

    fn prime_highlight_cache(&mut self) {
        for diff in &self.diffs {
            if diff.is_binary {
                continue;
            }

            let mut lines = Vec::new();
            for hunk in &diff.hunks {
                for line in &hunk.lines {
                    lines.push(line.content.as_str());
                }
            }

            if !lines.is_empty() {
                let _ = self.highlighter.highlight_lines(&diff.path, &lines);
            }
        }
    }

    /// Update the list of visible diff indices (respecting hidden filter)
    fn update_visible_diffs(&mut self) {
        self.visible_diffs = self.diffs
            .iter()
            .enumerate()
            .filter(|(_, d)| self.show_hidden || !is_hidden_file(&d.path))
            .map(|(i, _)| i)
            .collect();
    }

    /// Get the current branch name
    fn current_branch(&self) -> &str {
        self.worktrees
            .get(self.current_worktree)
            .and_then(|w| w.branch.as_deref())
            .unwrap_or("HEAD")
    }

    /// Run the application
    pub fn run(&mut self) -> Result<()> {
        // Setup terminal
        enable_raw_mode()?;
        let mut stdout = io::stdout();
        execute!(stdout, EnterAlternateScreen, crossterm::event::EnableMouseCapture)?;
        let backend = CrosstermBackend::new(stdout);
        let mut terminal = Terminal::new(backend)?;

        // Main loop
        loop {
            // Draw
            terminal.draw(|frame| {
                self.width = frame.area().width;
                self.height = frame.area().height;
                self.render(frame);
            })?;

            // Handle events
            if event::poll(std::time::Duration::from_millis(100))? {
                match event::read()? {
                    Event::Key(key) => {
                        if self.handle_key(key) {
                            break;
                        }
                    }
                    Event::Mouse(mouse) => {
                        self.handle_mouse(mouse);
                    }
                    Event::Resize(w, h) => {
                        self.width = w;
                        self.height = h;
                    }
                    _ => {}
                }
            }
        }

        // Restore terminal
        disable_raw_mode()?;
        execute!(
            terminal.backend_mut(),
            LeaveAlternateScreen,
            crossterm::event::DisableMouseCapture
        )?;

        Ok(())
    }

    /// Render the application
    fn render(&mut self, frame: &mut ratatui::Frame) {
        let area = frame.area();

        match self.view_mode {
            ViewMode::Diff => self.render_diff_view(frame, area),
            ViewMode::CommitFilter => {
                self.render_diff_view(frame, area);
                render_commit_popup(frame.buffer_mut(), area, &self.commits, self.popup_cursor, &self.styles);
            }
            ViewMode::WorktreeSwitcher => {
                self.render_diff_view(frame, area);
                render_worktree_popup(frame.buffer_mut(), area, &self.worktrees, self.popup_cursor, &self.filter_input, &self.styles);
            }
            ViewMode::WorktreeList => {
                self.render_worktree_list(frame, area);
            }
            ViewMode::Help => {
                self.render_diff_view(frame, area);
                render_help_popup(frame.buffer_mut(), area, &self.styles);
            }
        }
    }

    /// Render the main diff view
    fn render_diff_view(&mut self, frame: &mut ratatui::Frame, area: Rect) {
        // Layout: header (1) + content + footer (1)
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(1),
                Constraint::Min(0),
                Constraint::Length(1),
            ])
            .split(area);

        let header_area = chunks[0];
        let content_area = chunks[1];
        let footer_area = chunks[2];

        // Split content into sidebar + diff
        let content_chunks = Layout::default()
            .direction(Direction::Horizontal)
            .constraints([
                Constraint::Length(SIDEBAR_WIDTH),
                Constraint::Min(0),
            ])
            .split(content_area);

        let sidebar_area = content_chunks[0];
        let diff_area = content_chunks[1];

        // Calculate stats
        let (added, removed) = git::compute_stats(&self.diffs);
        let selected_count = self.commits.iter().filter(|c| c.selected).count();
        let total_count = self.commits.len();

        // Get current file at scroll position
        let current_file = self.get_current_file();

        // Render header
        render_header(
            frame.buffer_mut(),
            header_area,
            self.current_branch(),
            &self.main_branch,
            selected_count,
            total_count,
            added,
            removed,
            current_file.as_deref(),
            &self.styles,
        );

        // Render sidebar
        let tree_nodes = flatten_tree(&self.file_tree);
        let tree_refs: Vec<&TreeNode> = tree_nodes.iter().cloned().collect();
        let hidden_count = if self.show_hidden {
            0
        } else {
            self.diffs.len() - self.visible_diffs.len()
        };

        render_sidebar(
            frame.buffer_mut(),
            sidebar_area,
            &tree_refs,
            self.file_cursor,
            self.sidebar_scroll,
            hidden_count,
            self.focus == FocusArea::Sidebar,
            &self.styles,
        );

        // Get visible diffs
        let visible: Vec<FileDiff> = self.visible_diffs
            .iter()
            .filter_map(|&i| self.diffs.get(i).cloned())
            .collect();

        // Render diff content
        render_diff_content(
            frame.buffer_mut(),
            diff_area,
            &visible,
            self.content_scroll,
            self.diff_mode,
            &mut self.highlighter,
            &self.styles,
        );

        // Render footer
        render_footer(
            frame.buffer_mut(),
            footer_area,
            self.focus,
            self.show_hidden,
            self.context_lines,
            &self.styles,
        );
    }

    /// Render worktree list view
    fn render_worktree_list(&mut self, frame: &mut ratatui::Frame, area: Rect) {
        // Similar to diff view but shows worktree list instead
        render_worktree_popup(frame.buffer_mut(), area, &self.worktrees, self.popup_cursor, &self.filter_input, &self.styles);
    }

    /// Get the file at the current scroll position
    fn get_current_file(&self) -> Option<String> {
        let visible: Vec<&FileDiff> = self.visible_diffs
            .iter()
            .filter_map(|&i| self.diffs.get(i))
            .collect();

        let mut line = 0;
        for diff in visible {
            let file_lines = Self::file_line_count(diff);

            if line + file_lines > self.content_scroll {
                return Some(diff.path.clone());
            }
            line += file_lines;
        }

        None
    }

    /// Handle keyboard input. Returns true if app should quit.
    fn handle_key(&mut self, key: KeyEvent) -> bool {
        match self.view_mode {
            ViewMode::Diff => self.handle_diff_key(key),
            ViewMode::CommitFilter => self.handle_commit_filter_key(key),
            ViewMode::WorktreeSwitcher => self.handle_worktree_switcher_key(key),
            ViewMode::WorktreeList => self.handle_worktree_list_key(key),
            ViewMode::Help => self.handle_help_key(key),
        }
    }

    /// Handle keys in diff view
    fn handle_diff_key(&mut self, key: KeyEvent) -> bool {
        // Check for number prefix
        if let KeyCode::Char(c) = key.code {
            if c.is_ascii_digit() {
                let digit = c.to_digit(10).unwrap() as usize;
                self.number_prefix = Some(self.number_prefix.unwrap_or(0) * 10 + digit);
                return false;
            }
        }

        let (count, had_prefix) = match self.number_prefix.take() {
            Some(value) => (value, true),
            None => (1, false),
        };

        match (key.code, key.modifiers) {
            // Quit
            (KeyCode::Char('q'), _) => return true,
            (KeyCode::Esc, _) => return true,

            // Navigation
            (KeyCode::Char('j') | KeyCode::Down, _) => {
                if self.focus == FocusArea::Sidebar {
                    self.move_sidebar_cursor(count as i32);
                } else {
                    self.scroll_content(count as i32);
                }
            }
            (KeyCode::Char('k') | KeyCode::Up, _) => {
                if self.focus == FocusArea::Sidebar {
                    self.move_sidebar_cursor(-(count as i32));
                } else {
                    self.scroll_content(-(count as i32));
                }
            }
            (KeyCode::Char('d'), KeyModifiers::CONTROL) => {
                let page = (self.height / 2) as i32;
                if self.focus == FocusArea::Sidebar {
                    self.scroll_sidebar(page * count as i32);
                } else {
                    self.scroll_content(page * count as i32);
                }
            }
            (KeyCode::Char('u'), KeyModifiers::CONTROL) => {
                let page = (self.height / 2) as i32;
                if self.focus == FocusArea::Sidebar {
                    self.scroll_sidebar(-page * count as i32);
                } else {
                    self.scroll_content(-page * count as i32);
                }
            }
            (KeyCode::Char('g'), _) => {
                if self.focus == FocusArea::Sidebar {
                    self.set_sidebar_cursor(0);
                } else {
                    self.content_scroll = 0;
                }
            }
            (KeyCode::Char('G'), _) => {
                if self.focus == FocusArea::Sidebar {
                    let total = self.sidebar_len();
                    if total > 0 {
                        let target = if had_prefix {
                            count.saturating_sub(1)
                        } else {
                            total.saturating_sub(1)
                        };
                        self.set_sidebar_cursor(target.min(total.saturating_sub(1)));
                    }
                } else if had_prefix {
                    let target = count.saturating_sub(1).min(self.max_scroll());
                    self.content_scroll = target;
                } else {
                    self.content_scroll = self.max_scroll();
                }
            }
            (KeyCode::Char('n'), _) => {
                for _ in 0..count {
                    self.next_file();
                }
            }
            (KeyCode::Char('N'), _) => {
                for _ in 0..count {
                    self.prev_file();
                }
            }

            // Focus
            (KeyCode::Tab, _) => {
                self.focus = match self.focus {
                    FocusArea::Content => FocusArea::Sidebar,
                    FocusArea::Sidebar => FocusArea::Content,
                };
            }

            // View toggles
            (KeyCode::Char('u'), KeyModifiers::NONE) => {
                self.diff_mode = match self.diff_mode {
                    DiffMode::SideBySide => DiffMode::Unified,
                    DiffMode::Unified => DiffMode::SideBySide,
                };
            }
            (KeyCode::Char('x'), _) => {
                self.context_lines = match self.context_lines {
                    3 => 1,
                    1 => 0,
                    _ => 3,
                };
                let _ = self.reload_diffs();
            }
            (KeyCode::Char('h'), KeyModifiers::NONE) => {
                self.show_hidden = !self.show_hidden;
                self.update_visible_diffs();
            }
            (KeyCode::Char(' '), _) => {
                if self.focus == FocusArea::Sidebar {
                    self.toggle_sidebar_node();
                } else {
                    self.toggle_current_file();
                }
            }
            (KeyCode::Enter, _) => {
                if self.focus == FocusArea::Sidebar {
                    self.jump_to_sidebar_selection();
                }
            }
            (KeyCode::Char('z'), _) => {
                self.toggle_all_files();
            }

            // Popups
            (KeyCode::Char('c'), _) => {
                self.view_mode = ViewMode::CommitFilter;
                self.popup_cursor = 0;
            }
            (KeyCode::Char('w'), KeyModifiers::NONE) => {
                self.view_mode = ViewMode::WorktreeSwitcher;
                self.popup_cursor = 0;
                self.filter_input.clear();
            }
            (KeyCode::Char('W'), _) => {
                self.view_mode = ViewMode::WorktreeList;
                self.popup_cursor = self.current_worktree;
            }
            (KeyCode::Char('?'), _) => {
                self.view_mode = ViewMode::Help;
            }

            _ => {}
        }

        false
    }

    /// Handle keys in commit filter popup
    fn handle_commit_filter_key(&mut self, key: KeyEvent) -> bool {
        match key.code {
            KeyCode::Esc => {
                self.view_mode = ViewMode::Diff;
            }
            KeyCode::Enter => {
                self.view_mode = ViewMode::Diff;
                let _ = self.reload_diffs();
            }
            KeyCode::Char('j') | KeyCode::Down => {
                if self.popup_cursor < self.commits.len().saturating_sub(1) {
                    self.popup_cursor += 1;
                }
            }
            KeyCode::Char('k') | KeyCode::Up => {
                self.popup_cursor = self.popup_cursor.saturating_sub(1);
            }
            KeyCode::Char(' ') => {
                if let Some(commit) = self.commits.get_mut(self.popup_cursor) {
                    commit.selected = !commit.selected;
                }
            }
            KeyCode::Char('a') => {
                for commit in &mut self.commits {
                    commit.selected = true;
                }
            }
            KeyCode::Char('n') => {
                for commit in &mut self.commits {
                    commit.selected = false;
                }
            }
            _ => {}
        }
        false
    }

    /// Handle keys in worktree switcher popup
    fn handle_worktree_switcher_key(&mut self, key: KeyEvent) -> bool {
        match key.code {
            KeyCode::Esc => {
                self.view_mode = ViewMode::Diff;
                self.filter_input.clear();
            }
            KeyCode::Enter => {
                // Switch to selected worktree
                let filtered: Vec<_> = self.worktrees
                    .iter()
                    .enumerate()
                    .filter(|(_, wt)| {
                        self.filter_input.is_empty()
                            || wt.path.to_string_lossy().to_lowercase().contains(&self.filter_input.to_lowercase())
                            || wt.branch.as_ref().map_or(false, |b| b.to_lowercase().contains(&self.filter_input.to_lowercase()))
                    })
                    .collect();

                if let Some((idx, wt)) = filtered.get(self.popup_cursor) {
                    self.repo_path = wt.path.clone();
                    self.current_worktree = *idx;
                    let _ = self.load_data();
                }

                self.view_mode = ViewMode::Diff;
                self.filter_input.clear();
            }
            KeyCode::Char('j') | KeyCode::Down => {
                self.popup_cursor += 1;
            }
            KeyCode::Char('k') | KeyCode::Up => {
                self.popup_cursor = self.popup_cursor.saturating_sub(1);
            }
            KeyCode::Char(c) => {
                self.filter_input.push(c);
                self.popup_cursor = 0;
            }
            KeyCode::Backspace => {
                self.filter_input.pop();
                self.popup_cursor = 0;
            }
            _ => {}
        }
        false
    }

    /// Handle keys in worktree list view
    fn handle_worktree_list_key(&mut self, key: KeyEvent) -> bool {
        match key.code {
            KeyCode::Esc | KeyCode::Char('q') => {
                self.view_mode = ViewMode::Diff;
            }
            KeyCode::Enter => {
                if let Some(wt) = self.worktrees.get(self.popup_cursor) {
                    self.repo_path = wt.path.clone();
                    self.current_worktree = self.popup_cursor;
                    let _ = self.load_data();
                }
                self.view_mode = ViewMode::Diff;
            }
            KeyCode::Char('j') | KeyCode::Down => {
                if self.popup_cursor < self.worktrees.len().saturating_sub(1) {
                    self.popup_cursor += 1;
                }
            }
            KeyCode::Char('k') | KeyCode::Up => {
                self.popup_cursor = self.popup_cursor.saturating_sub(1);
            }
            _ => {}
        }
        false
    }

    /// Handle keys in help overlay
    fn handle_help_key(&mut self, key: KeyEvent) -> bool {
        match key.code {
            KeyCode::Esc | KeyCode::Char('q') | KeyCode::Char('?') => {
                self.view_mode = ViewMode::Diff;
            }
            _ => {}
        }
        false
    }

    /// Handle mouse input
    fn handle_mouse(&mut self, mouse: MouseEvent) {
        match mouse.kind {
            MouseEventKind::ScrollDown => {
                if mouse.column < SIDEBAR_WIDTH {
                    self.scroll_sidebar(MOUSE_SCROLL_LINES);
                } else {
                    self.scroll_content(MOUSE_SCROLL_LINES);
                }
            }
            MouseEventKind::ScrollUp => {
                if mouse.column < SIDEBAR_WIDTH {
                    self.scroll_sidebar(-MOUSE_SCROLL_LINES);
                } else {
                    self.scroll_content(-MOUSE_SCROLL_LINES);
                }
            }
            MouseEventKind::Down(MouseButton::Left) => {
                // Check if click is in sidebar
                if mouse.column < SIDEBAR_WIDTH {
                    self.focus = FocusArea::Sidebar;
                    self.handle_sidebar_click(mouse.row);
                } else {
                    self.focus = FocusArea::Content;
                }
            }
            _ => {}
        }
    }

    /// Scroll content by delta lines
    fn scroll_content(&mut self, delta: i32) {
        let new_scroll = if delta >= 0 {
            self.content_scroll.saturating_add(delta as usize)
        } else {
            self.content_scroll.saturating_sub((-delta) as usize)
        };

        self.content_scroll = new_scroll.min(self.max_scroll());
    }

    /// Get maximum scroll position
    fn max_scroll(&self) -> usize {
        let visible: Vec<&FileDiff> = self.visible_diffs
            .iter()
            .filter_map(|&i| self.diffs.get(i))
            .collect();

        let total_lines = calculate_total_lines(&visible.iter().cloned().cloned().collect::<Vec<_>>());
        total_lines.saturating_sub(self.height as usize - 2)
    }

    /// Navigate to next file
    fn next_file(&mut self) {
        // Find the next file boundary in scroll position
        let visible: Vec<&FileDiff> = self.visible_diffs
            .iter()
            .filter_map(|&i| self.diffs.get(i))
            .collect();

        let mut line = 0;
        for diff in visible {
            let file_lines = Self::file_line_count(diff);

            if line > self.content_scroll {
                self.content_scroll = line;
                return;
            }
            line += file_lines;
        }
    }

    /// Navigate to previous file
    fn prev_file(&mut self) {
        let visible: Vec<&FileDiff> = self.visible_diffs
            .iter()
            .filter_map(|&i| self.diffs.get(i))
            .collect();

        let mut positions: Vec<usize> = Vec::new();
        let mut line = 0;

        for diff in visible {
            positions.push(line);
            let file_lines = Self::file_line_count(diff);
            line += file_lines;
        }

        // Find the position before current scroll
        for &pos in positions.iter().rev() {
            if pos < self.content_scroll {
                self.content_scroll = pos;
                return;
            }
        }

        self.content_scroll = 0;
    }

    /// Toggle collapse on current file
    fn toggle_current_file(&mut self) {
        if let Some(current_file) = self.get_current_file() {
            if let Some(diff) = self.diffs.iter_mut().find(|d| d.path == current_file) {
                diff.collapsed = !diff.collapsed;
            }
        }
    }

    /// Toggle collapse on all files
    fn toggle_all_files(&mut self) {
        let all_collapsed = self.diffs.iter().all(|d| d.collapsed);
        for diff in &mut self.diffs {
            diff.collapsed = !all_collapsed;
        }
    }

    fn sidebar_len(&self) -> usize {
        flatten_tree(&self.file_tree).len()
    }

    fn sidebar_visible_height(&self) -> usize {
        let content_height = self.height.saturating_sub(2);
        content_height.saturating_sub(2) as usize
    }

    fn set_sidebar_cursor(&mut self, index: usize) {
        let total = self.sidebar_len();
        if total == 0 {
            self.file_cursor = 0;
            self.sidebar_scroll = 0;
            return;
        }

        self.file_cursor = index.min(total.saturating_sub(1));
        self.ensure_sidebar_cursor_visible(total);
    }

    fn move_sidebar_cursor(&mut self, delta: i32) {
        let total = self.sidebar_len();
        if total == 0 {
            self.file_cursor = 0;
            self.sidebar_scroll = 0;
            return;
        }

        let new_cursor = if delta >= 0 {
            self.file_cursor.saturating_add(delta as usize)
        } else {
            self.file_cursor.saturating_sub((-delta) as usize)
        };

        self.file_cursor = new_cursor.min(total.saturating_sub(1));
        self.ensure_sidebar_cursor_visible(total);
    }

    fn scroll_sidebar(&mut self, delta: i32) {
        let total = self.sidebar_len();
        let visible = self.sidebar_visible_height();
        if total <= visible || visible == 0 {
            self.sidebar_scroll = 0;
            return;
        }

        let max_scroll = total.saturating_sub(visible);
        let new_scroll = if delta >= 0 {
            self.sidebar_scroll.saturating_add(delta as usize)
        } else {
            self.sidebar_scroll.saturating_sub((-delta) as usize)
        };

        self.sidebar_scroll = new_scroll.min(max_scroll);
    }

    fn ensure_sidebar_cursor_visible(&mut self, total: usize) {
        let visible = self.sidebar_visible_height();
        if visible == 0 {
            return;
        }

        if self.file_cursor < self.sidebar_scroll {
            self.sidebar_scroll = self.file_cursor;
        } else if self.file_cursor >= self.sidebar_scroll + visible {
            self.sidebar_scroll = self.file_cursor + 1 - visible;
        }

        let max_scroll = total.saturating_sub(visible);
        self.sidebar_scroll = self.sidebar_scroll.min(max_scroll);
    }

    fn toggle_sidebar_node(&mut self) {
        let nodes = flatten_tree(&self.file_tree);
        let Some(node) = nodes.get(self.file_cursor) else {
            return;
        };

        if node.is_folder {
            let expanded = self.expanded_folders.entry(node.path.clone()).or_insert(true);
            *expanded = !*expanded;

            let path = node.path.clone();
            self.file_tree = build_file_tree(&self.diffs, &self.expanded_folders);
            self.restore_sidebar_cursor(&path);
        } else if let Some(index) = node.diff_index {
            if let Some(diff) = self.diffs.get_mut(index) {
                diff.collapsed = !diff.collapsed;
            }
        }
    }

    fn restore_sidebar_cursor(&mut self, path: &str) {
        let nodes = flatten_tree(&self.file_tree);
        if nodes.is_empty() {
            self.file_cursor = 0;
            self.sidebar_scroll = 0;
            return;
        }

        if let Some(index) = nodes.iter().position(|node| node.path == path) {
            self.file_cursor = index;
        } else {
            self.file_cursor = self.file_cursor.min(nodes.len().saturating_sub(1));
        }

        self.ensure_sidebar_cursor_visible(nodes.len());
    }

    fn jump_to_sidebar_selection(&mut self) {
        let nodes = flatten_tree(&self.file_tree);
        let Some(node) = nodes.get(self.file_cursor) else {
            return;
        };

        if node.is_folder {
            let expanded = self.expanded_folders.entry(node.path.clone()).or_insert(true);
            if !*expanded {
                *expanded = true;
                let path = node.path.clone();
                self.file_tree = build_file_tree(&self.diffs, &self.expanded_folders);
                self.restore_sidebar_cursor(&path);
            }
            return;
        }

        if let Some(index) = node.diff_index {
            self.scroll_to_diff_index(index);
        }
    }

    fn handle_sidebar_click(&mut self, row: u16) {
        let content_top = 1u16;
        let sidebar_top = content_top;
        let inner_top = sidebar_top.saturating_add(1);
        let inner_height = self.height.saturating_sub(2).saturating_sub(2);

        if row < inner_top || row >= inner_top.saturating_add(inner_height) {
            return;
        }

        let index = self.sidebar_scroll + (row - inner_top) as usize;
        let nodes = flatten_tree(&self.file_tree);
        if index >= nodes.len() {
            return;
        }

        let node = nodes[index];
        let node_path = node.path.clone();
        let node_is_folder = node.is_folder;
        let node_diff_index = node.diff_index;

        self.set_sidebar_cursor(index);
        if node_is_folder {
            let expanded = self.expanded_folders.entry(node_path.clone()).or_insert(true);
            *expanded = !*expanded;
            self.file_tree = build_file_tree(&self.diffs, &self.expanded_folders);
            self.restore_sidebar_cursor(&node_path);
        } else if let Some(diff_index) = node_diff_index {
            self.scroll_to_diff_index(diff_index);
        }
    }

    fn scroll_to_diff_index(&mut self, diff_index: usize) {
        let mut line = 0;
        for &idx in &self.visible_diffs {
            if let Some(diff) = self.diffs.get(idx) {
                if idx == diff_index {
                    self.content_scroll = line.min(self.max_scroll());
                    return;
                }
                line += Self::file_line_count(diff);
            }
        }
    }

    fn file_line_count(diff: &FileDiff) -> usize {
        if diff.collapsed || diff.is_binary {
            1
        } else {
            1 + diff.hunks.iter().map(|h| 1 + h.lines.len()).sum::<usize>()
        }
    }
}

/// Check if a file matches hidden patterns
fn is_hidden_file(path: &str) -> bool {
    // Check for dotfiles/dotfolders (any path component starting with ".")
    if path.split('/').any(|part| part.starts_with('.')) {
        return true;
    }

    // Check against specific hidden patterns
    let filename = path.split('/').last().unwrap_or(path);
    HIDDEN_PATTERNS.iter().any(|p| filename == *p)
}
