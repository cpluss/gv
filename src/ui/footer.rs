//! Footer rendering
//!
//! Displays keybinding hints and focus state.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    text::{Line, Span},
    widgets::Widget,
};

use super::Styles;

/// Focus area indicator
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FocusArea {
    Sidebar,
    Content,
}

/// Footer widget showing keybinding hints
pub struct Footer<'a> {
    /// Current focus area
    pub focus: FocusArea,
    /// Whether hidden files are shown
    pub show_hidden: bool,
    /// Current context lines setting
    pub context_lines: u32,
    /// Styles
    pub styles: &'a Styles,
}

impl Widget for Footer<'_> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        if area.height == 0 {
            return;
        }

        // Clear the footer area
        for x in area.x..area.x + area.width {
            buf[(x, area.y)]
                .set_char(' ')
                .set_style(self.styles.footer);
        }

        let mut spans = Vec::new();
        spans.push(Span::styled(" ", self.styles.footer));

        // Keybinding hints
        let hints = [
            ("j/k", "scroll"),
            ("n/N", "file"),
            ("u", "view"),
            ("c", "commits"),
            ("w", "worktree"),
            ("h", if self.show_hidden { "hide" } else { "show" }),
            ("x", &format!("ctx:{}", self.context_lines)),
            ("?", "help"),
            ("q", "quit"),
        ];

        for (i, (key, desc)) in hints.iter().enumerate() {
            if i > 0 {
                spans.push(Span::styled(" │ ", self.styles.footer));
            }
            spans.push(Span::styled(*key, self.styles.footer_key));
            spans.push(Span::styled(format!(" {}", desc), self.styles.footer));
        }

        // Focus indicator (right-aligned)
        let focus_text = match self.focus {
            FocusArea::Sidebar => " [SIDEBAR] ",
            FocusArea::Content => " [CONTENT] ",
        };

        let left_width: u16 = spans.iter().map(|s| s.content.len() as u16).sum();
        let focus_width = focus_text.len() as u16;

        if left_width + focus_width < area.width {
            let padding = area.width - left_width - focus_width;
            spans.push(Span::styled(" ".repeat(padding as usize), self.styles.footer));
            spans.push(Span::styled(focus_text, self.styles.footer_key));
        }

        let line = Line::from(spans);
        buf.set_line(area.x, area.y, &line, area.width);
    }
}

/// Render the footer bar
pub fn render_footer(
    buf: &mut Buffer,
    area: Rect,
    focus: FocusArea,
    show_hidden: bool,
    context_lines: u32,
    styles: &Styles,
) {
    let footer = Footer {
        focus,
        show_hidden,
        context_lines,
        styles,
    };
    footer.render(area, buf);
}
