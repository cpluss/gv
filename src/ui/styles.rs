//! UI styles
//!
//! Defines consistent styling for the entire application.
//! Uses a delta-like color palette for diffs.

use ratatui::style::{Color, Modifier, Style};

/// Color palette inspired by delta diff viewer
pub mod colors {
    use ratatui::style::Color;

    // Base colors
    pub const BG: Color = Color::Reset;
    pub const FG: Color = Color::White;
    pub const DIM: Color = Color::DarkGray;

    // Diff colors (delta-like palette)
    pub const ADDED_BG: Color = Color::Rgb(35, 60, 35);
    pub const ADDED_FG: Color = Color::Rgb(120, 200, 120);
    pub const REMOVED_BG: Color = Color::Rgb(60, 35, 35);
    pub const REMOVED_FG: Color = Color::Rgb(200, 120, 120);

    // Gutter colors
    pub const GUTTER_ADDED: Color = Color::Green;
    pub const GUTTER_REMOVED: Color = Color::Red;
    pub const GUTTER_CONTEXT: Color = Color::DarkGray;

    // Line numbers
    pub const LINE_NUMBER: Color = Color::DarkGray;
    pub const LINE_NUMBER_HIGHLIGHT: Color = Color::Yellow;

    // UI elements
    pub const HEADER_BG: Color = Color::Rgb(40, 44, 52);
    pub const HEADER_FG: Color = Color::White;
    pub const FOOTER_BG: Color = Color::Rgb(40, 44, 52);
    pub const FOOTER_FG: Color = Color::DarkGray;

    // Stats
    pub const STATS_ADDED: Color = Color::Green;
    pub const STATS_REMOVED: Color = Color::Red;

    // Selection
    pub const SELECTED_BG: Color = Color::Rgb(60, 60, 80);
    pub const CURSOR_BG: Color = Color::Rgb(80, 80, 100);

    // File headers
    pub const FILE_HEADER_BG: Color = Color::Rgb(50, 55, 65);
    pub const FILE_HEADER_FG: Color = Color::Cyan;

    // Hunk headers
    pub const HUNK_HEADER_FG: Color = Color::Magenta;

    // Borders
    pub const BORDER: Color = Color::DarkGray;
    pub const BORDER_FOCUS: Color = Color::Cyan;

    // Popup
    pub const POPUP_BG: Color = Color::Rgb(30, 34, 42);
    pub const POPUP_BORDER: Color = Color::Cyan;

    // Worktree
    pub const WORKTREE_CURRENT: Color = Color::Green;
    pub const WORKTREE_PATH: Color = Color::DarkGray;
    pub const WORKTREE_BRANCH: Color = Color::Cyan;
}

/// Collection of styles used throughout the UI
#[derive(Clone)]
pub struct Styles {
    // Header/Footer
    pub header: Style,
    pub footer: Style,
    pub footer_key: Style,

    // Diff content
    pub line_number: Style,
    pub line_number_highlight: Style,
    pub line_added: Style,
    pub line_removed: Style,
    pub line_context: Style,
    pub gutter_added: Style,
    pub gutter_removed: Style,
    pub gutter_context: Style,

    // File headers
    pub file_header: Style,
    pub hunk_header: Style,

    // Stats
    pub stats_added: Style,
    pub stats_removed: Style,

    // Sidebar
    pub sidebar_normal: Style,
    pub sidebar_selected: Style,
    pub sidebar_cursor: Style,
    pub folder_icon: Style,

    // Borders
    pub border: Style,
    pub border_focus: Style,

    // Popup
    pub popup: Style,
    pub popup_title: Style,
    pub popup_selected: Style,

    // Worktree
    pub worktree_current: Style,
    pub worktree_path: Style,
    pub worktree_branch: Style,

    // Help
    pub help_key: Style,
    pub help_desc: Style,
}

impl Default for Styles {
    fn default() -> Self {
        Self::new()
    }
}

impl Styles {
    /// Create a new Styles instance with default values
    pub fn new() -> Self {
        Self {
            // Header/Footer
            header: Style::default()
                .bg(colors::HEADER_BG)
                .fg(colors::HEADER_FG),
            footer: Style::default()
                .bg(colors::FOOTER_BG)
                .fg(colors::FOOTER_FG),
            footer_key: Style::default()
                .fg(colors::HEADER_FG)
                .add_modifier(Modifier::BOLD),

            // Diff content
            line_number: Style::default().fg(colors::LINE_NUMBER),
            line_number_highlight: Style::default()
                .fg(colors::LINE_NUMBER_HIGHLIGHT)
                .add_modifier(Modifier::BOLD),
            line_added: Style::default()
                .bg(colors::ADDED_BG)
                .fg(colors::ADDED_FG),
            line_removed: Style::default()
                .bg(colors::REMOVED_BG)
                .fg(colors::REMOVED_FG),
            line_context: Style::default().fg(colors::FG),
            gutter_added: Style::default().fg(colors::GUTTER_ADDED),
            gutter_removed: Style::default().fg(colors::GUTTER_REMOVED),
            gutter_context: Style::default().fg(colors::GUTTER_CONTEXT),

            // File headers
            file_header: Style::default()
                .bg(colors::FILE_HEADER_BG)
                .fg(colors::FILE_HEADER_FG)
                .add_modifier(Modifier::BOLD),
            hunk_header: Style::default()
                .fg(colors::HUNK_HEADER_FG)
                .add_modifier(Modifier::ITALIC),

            // Stats
            stats_added: Style::default()
                .fg(colors::STATS_ADDED)
                .add_modifier(Modifier::BOLD),
            stats_removed: Style::default()
                .fg(colors::STATS_REMOVED)
                .add_modifier(Modifier::BOLD),

            // Sidebar
            sidebar_normal: Style::default().fg(colors::FG),
            sidebar_selected: Style::default()
                .bg(colors::SELECTED_BG)
                .fg(colors::FG),
            sidebar_cursor: Style::default()
                .bg(colors::CURSOR_BG)
                .fg(colors::FG)
                .add_modifier(Modifier::BOLD),
            folder_icon: Style::default().fg(colors::DIM),

            // Borders
            border: Style::default().fg(colors::BORDER),
            border_focus: Style::default().fg(colors::BORDER_FOCUS),

            // Popup
            popup: Style::default().bg(colors::POPUP_BG).fg(colors::FG),
            popup_title: Style::default()
                .fg(colors::POPUP_BORDER)
                .add_modifier(Modifier::BOLD),
            popup_selected: Style::default()
                .bg(colors::SELECTED_BG)
                .fg(colors::FG),

            // Worktree
            worktree_current: Style::default()
                .fg(colors::WORKTREE_CURRENT)
                .add_modifier(Modifier::BOLD),
            worktree_path: Style::default().fg(colors::WORKTREE_PATH),
            worktree_branch: Style::default().fg(colors::WORKTREE_BRANCH),

            // Help
            help_key: Style::default()
                .fg(Color::Yellow)
                .add_modifier(Modifier::BOLD),
            help_desc: Style::default().fg(colors::DIM),
        }
    }
}
