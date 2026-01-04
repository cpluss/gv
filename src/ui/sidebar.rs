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

/// Default sidebar width
pub const DEFAULT_SIDEBAR_WIDTH: u16 = 35;
/// Minimum sidebar width
pub const MIN_SIDEBAR_WIDTH: u16 = 20;
/// Maximum sidebar width
pub const MAX_SIDEBAR_WIDTH: u16 = 80;
/// Sidebar resize increment
pub const SIDEBAR_RESIZE_STEP: u16 = 5;
/// Maximum visual indentation depth (to prevent deep files from being invisible)
const MAX_VISUAL_INDENT: usize = 6;

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
            let style = match (is_cursor, node.is_hidden) {
                (true, true) => self.styles.sidebar_hidden_cursor,
                (true, false) => self.styles.sidebar_cursor,
                (false, true) => self.styles.sidebar_hidden,
                (false, false) => self.styles.sidebar_normal,
            };

            // Build the line
            let mut spans = Vec::new();

            // Indentation (capped to prevent deep files from being invisible)
            let visual_depth = node.depth.min(MAX_VISUAL_INDENT);
            let indent = "  ".repeat(visual_depth);
            spans.push(Span::styled(indent, style));

            // Depth indicator for very deep items
            if node.depth > MAX_VISUAL_INDENT {
                spans.push(Span::styled(
                    format!("{}·", node.depth - MAX_VISUAL_INDENT),
                    self.styles.line_number,
                ));
            }

            // Folder icon or file indicator
            if node.is_folder {
                let icon = if node.expanded { "▼ " } else { "▶ " };
                spans.push(Span::styled(icon, self.styles.folder_icon));
            } else {
                spans.push(Span::styled("  ", style));
            }

            // Name - calculate available space accounting for capped indent and depth indicator
            let indent_width = visual_depth * 2;
            let depth_indicator_width = if node.depth > MAX_VISUAL_INDENT {
                format!("{}·", node.depth - MAX_VISUAL_INDENT).len()
            } else {
                0
            };
            let max_name_width = (inner.width as usize)
                .saturating_sub(indent_width + depth_indicator_width + 12);
            let name = smart_truncate(&node.name, max_name_width);
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

/// Smart truncate: shows beginning...end for better context
///
/// For "very_long_filename.tsx" with max 12:
/// - Old: "very_long..." (loses extension info)
/// - New: "very...e.tsx" (preserves extension)
fn smart_truncate(s: &str, max_width: usize) -> String {
    if s.len() <= max_width {
        return s.to_string();
    }

    if max_width < 5 {
        // Too small for smart truncation
        return s.chars().take(max_width).collect();
    }

    // For filenames, try to preserve the extension
    let ellipsis = "…"; // Single character ellipsis
    let available = max_width - 1; // Space minus ellipsis

    // Split into prefix and suffix
    // Allocate more to the beginning (where the unique part usually is)
    let prefix_len = (available * 2) / 3;
    let suffix_len = available - prefix_len;

    let prefix: String = s.chars().take(prefix_len).collect();
    let suffix: String = s.chars().rev().take(suffix_len).collect::<String>().chars().rev().collect();

    format!("{}{}{}", prefix, ellipsis, suffix)
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
