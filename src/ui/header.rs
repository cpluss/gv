//! Header rendering
//!
//! Displays branch info, commit stats, and current file indicator.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    text::{Line, Span},
    widgets::Widget,
};

use super::Styles;

/// Header widget showing branch and stats info
pub struct Header<'a> {
    /// Current branch name
    pub branch: &'a str,
    /// Main/base branch name
    pub main_branch: &'a str,
    /// Number of selected commits
    pub selected_commits: usize,
    /// Total number of commits
    pub total_commits: usize,
    /// Lines added
    pub added: usize,
    /// Lines removed
    pub removed: usize,
    /// Current file being viewed
    pub current_file: Option<&'a str>,
    /// Styles
    pub styles: &'a Styles,
}

impl Widget for Header<'_> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        if area.height == 0 {
            return;
        }

        // Clear the header area
        for x in area.x..area.x + area.width {
            buf[(x, area.y)]
                .set_char(' ')
                .set_style(self.styles.header);
        }

        let mut spans = Vec::new();

        // Branch info: current → main
        spans.push(Span::styled(
            format!(" {} ", self.branch),
            self.styles.header,
        ));
        spans.push(Span::styled("→ ", self.styles.footer));
        spans.push(Span::styled(
            format!("{} ", self.main_branch),
            self.styles.header,
        ));

        // Separator
        spans.push(Span::styled(" │ ", self.styles.footer));

        // Commit count
        if self.total_commits > 0 {
            spans.push(Span::styled(
                format!("[{}/{} commits] ", self.selected_commits, self.total_commits),
                self.styles.header,
            ));
        }

        // Stats
        if self.added > 0 || self.removed > 0 {
            spans.push(Span::styled(
                format!("+{}", self.added),
                self.styles.stats_added,
            ));
            spans.push(Span::styled(" ", self.styles.header));
            spans.push(Span::styled(
                format!("-{}", self.removed),
                self.styles.stats_removed,
            ));
        }

        // Current file (right-aligned)
        if let Some(file) = self.current_file {
            let file_info = format!(" {} ", file);
            let file_width = file_info.len() as u16;

            // Calculate position for right alignment
            let left_content_width: u16 = spans.iter()
                .map(|s| s.content.len() as u16)
                .sum();

            if left_content_width + file_width < area.width {
                let padding = area.width - left_content_width - file_width;
                spans.push(Span::styled(
                    " ".repeat(padding as usize),
                    self.styles.header,
                ));
                spans.push(Span::styled(file_info, self.styles.header));
            }
        }

        let line = Line::from(spans);
        buf.set_line(area.x, area.y, &line, area.width);
    }
}

/// Render the header bar
pub fn render_header(
    buf: &mut Buffer,
    area: Rect,
    branch: &str,
    main_branch: &str,
    selected_commits: usize,
    total_commits: usize,
    added: usize,
    removed: usize,
    current_file: Option<&str>,
    styles: &Styles,
) {
    let header = Header {
        branch,
        main_branch,
        selected_commits,
        total_commits,
        added,
        removed,
        current_file,
        styles,
    };
    header.render(area, buf);
}
