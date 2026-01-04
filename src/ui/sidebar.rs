//! Sidebar rendering
//!
//! Displays file tree with collapsible folders and stats.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    text::{Line, Span},
    widgets::{Block, Borders, Widget},
};

use super::{Styles, TreeNode};

/// Sidebar width constant
pub const SIDEBAR_WIDTH: u16 = 35;

/// Sidebar widget showing file tree
pub struct Sidebar<'a> {
    /// Flattened tree nodes to display
    pub nodes: &'a [&'a TreeNode],
    /// Current cursor position
    pub cursor: usize,
    /// Scroll offset
    pub scroll: usize,
    /// Number of hidden files
    pub hidden_count: usize,
    /// Whether the sidebar is focused
    pub focused: bool,
    /// Styles
    pub styles: &'a Styles,
}

impl Widget for Sidebar<'_> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        // Draw border
        let border_style = if self.focused {
            self.styles.border_focus
        } else {
            self.styles.border
        };

        let title = if self.hidden_count > 0 {
            format!(" Files ({} hidden) ", self.hidden_count)
        } else {
            " Files ".to_string()
        };

        let block = Block::default()
            .borders(Borders::ALL)
            .border_style(border_style)
            .title(Span::styled(title, self.styles.popup_title));

        let inner = block.inner(area);
        block.render(area, buf);

        // Render file list
        let visible_height = inner.height as usize;

        for (i, node) in self.nodes.iter().skip(self.scroll).take(visible_height).enumerate() {
            let y = inner.y + i as u16;
            if y >= inner.y + inner.height {
                break;
            }

            let is_cursor = i + self.scroll == self.cursor;
            let style = if is_cursor {
                self.styles.sidebar_cursor
            } else {
                self.styles.sidebar_normal
            };

            // Build the line
            let mut spans = Vec::new();

            // Indentation
            let indent = "  ".repeat(node.depth);
            spans.push(Span::styled(indent, style));

            // Folder icon or file indicator
            if node.is_folder {
                let icon = if node.expanded { "▼ " } else { "▶ " };
                spans.push(Span::styled(icon, self.styles.folder_icon));
            } else {
                spans.push(Span::styled("  ", style));
            }

            // Name
            let max_name_width = (inner.width as usize).saturating_sub(node.depth * 2 + 12);
            let name = truncate(&node.name, max_name_width);
            spans.push(Span::styled(name, style));

            // Stats
            let stats = format!(" +{} -{}", node.added, node.removed);
            let name_len: usize = spans.iter().map(|s| s.content.len()).sum();
            let available = (inner.width as usize).saturating_sub(name_len + stats.len());

            if available > 0 {
                spans.push(Span::styled(" ".repeat(available), style));
            }

            spans.push(Span::styled(
                format!("+{}", node.added),
                self.styles.stats_added,
            ));
            spans.push(Span::styled(" ", style));
            spans.push(Span::styled(
                format!("-{}", node.removed),
                self.styles.stats_removed,
            ));

            // Render the line
            let line = Line::from(spans);
            buf.set_line(inner.x, y, &line, inner.width);

            // Fill background for cursor line
            if is_cursor {
                for x in inner.x..inner.x + inner.width {
                    buf[(x, y)].set_style(style);
                }
            }
        }
    }
}

/// Truncate a string to a maximum width
fn truncate(s: &str, max_width: usize) -> String {
    if s.len() <= max_width {
        s.to_string()
    } else if max_width > 3 {
        format!("{}...", &s[..max_width - 3])
    } else {
        s[..max_width].to_string()
    }
}

/// Render the sidebar
pub fn render_sidebar(
    buf: &mut Buffer,
    area: Rect,
    nodes: &[&TreeNode],
    cursor: usize,
    scroll: usize,
    hidden_count: usize,
    focused: bool,
    styles: &Styles,
) {
    let sidebar = Sidebar {
        nodes,
        cursor,
        scroll,
        hidden_count,
        focused,
        styles,
    };
    sidebar.render(area, buf);
}
