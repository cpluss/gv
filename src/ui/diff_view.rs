//! Diff content rendering
//!
//! Renders the main diff view in either side-by-side or unified mode.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    text::{Line, Span},
    widgets::Widget,
};
use unicode_width::UnicodeWidthStr;

use crate::git::{FileDiff, Hunk, LineType};
use crate::syntax::{Highlighter, HighlightedLine, Token};
use super::Styles;

/// Diff display mode
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DiffMode {
    /// Side-by-side view showing old and new columns
    SideBySide,
    /// Unified view showing all changes in one column
    Unified,
}

/// Diff content widget
pub struct DiffContent<'a> {
    /// List of file diffs to display
    pub diffs: &'a [FileDiff],
    /// Scroll offset (in lines)
    pub scroll: usize,
    /// Current diff mode
    pub mode: DiffMode,
    /// Syntax highlighter
    pub highlighter: &'a mut Highlighter,
    /// Styles
    pub styles: &'a Styles,
    /// Whether the content pane is focused
    pub focused: bool,
}

impl Widget for DiffContent<'_> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        match self.mode {
            DiffMode::Unified => render_unified(self, area, buf),
            DiffMode::SideBySide => render_side_by_side(self, area, buf),
        }
    }
}

/// Render unified diff view
fn render_unified(content: DiffContent<'_>, area: Rect, buf: &mut Buffer) {
    let mut current_line: usize = 0;
    let visible_start = content.scroll;
    let visible_end = content.scroll + area.height as usize;

    for diff in content.diffs.iter() {
        // File header
        if current_line >= visible_start && current_line < visible_end {
            let y = area.y + (current_line - visible_start) as u16;
            render_file_header(buf, area.x, y, area.width, diff, content.styles);
        }
        current_line += 1;

        if diff.collapsed || diff.is_binary {
            continue;
        }

        for hunk in &diff.hunks {
            // Hunk header
            if current_line >= visible_start && current_line < visible_end {
                let y = area.y + (current_line - visible_start) as u16;
                render_hunk_header(buf, area.x, y, area.width, hunk, content.styles);
            }
            current_line += 1;

            // Lines
            for line in &hunk.lines {
                if current_line >= visible_start && current_line < visible_end {
                    let y = area.y + (current_line - visible_start) as u16;
                    render_unified_line(
                        buf,
                        area.x,
                        y,
                        area.width,
                        line,
                        &diff.path,
                        content.highlighter,
                        content.styles,
                    );
                }
                current_line += 1;

                if current_line >= visible_end {
                    return;
                }
            }
        }
    }
}

/// Render side-by-side diff view
fn render_side_by_side(content: DiffContent<'_>, area: Rect, buf: &mut Buffer) {
    let mut current_line: usize = 0;
    let visible_start = content.scroll;
    let visible_end = content.scroll + area.height as usize;

    // Calculate column widths
    let half_width = area.width / 2;
    let line_num_width: u16 = 6;

    for diff in content.diffs.iter() {
        // File header (spans both columns)
        if current_line >= visible_start && current_line < visible_end {
            let y = area.y + (current_line - visible_start) as u16;
            render_file_header(buf, area.x, y, area.width, diff, content.styles);
        }
        current_line += 1;

        if diff.collapsed || diff.is_binary {
            continue;
        }

        for hunk in &diff.hunks {
            // Hunk header
            if current_line >= visible_start && current_line < visible_end {
                let y = area.y + (current_line - visible_start) as u16;
                render_hunk_header(buf, area.x, y, area.width, hunk, content.styles);
            }
            current_line += 1;

            // Process lines into pairs for side-by-side display
            let pairs = pair_lines(&hunk.lines);

            for (old_line, new_line) in pairs {
                if current_line >= visible_start && current_line < visible_end {
                    let y = area.y + (current_line - visible_start) as u16;

                    // Left column (old)
                    render_side_column(
                        buf,
                        area.x,
                        y,
                        half_width,
                        line_num_width,
                        old_line,
                        &diff.path,
                        content.highlighter,
                        content.styles,
                        true, // is_old
                    );

                    // Right column (new)
                    render_side_column(
                        buf,
                        area.x + half_width,
                        y,
                        half_width,
                        line_num_width,
                        new_line,
                        &diff.path,
                        content.highlighter,
                        content.styles,
                        false, // is_old
                    );
                }
                current_line += 1;

                if current_line >= visible_end {
                    return;
                }
            }
        }
    }
}

/// Pair old and new lines for side-by-side display
fn pair_lines(lines: &[crate::git::DiffLine]) -> Vec<(Option<&crate::git::DiffLine>, Option<&crate::git::DiffLine>)> {
    let mut pairs = Vec::new();
    let mut old_lines: Vec<&crate::git::DiffLine> = Vec::new();
    let mut new_lines: Vec<&crate::git::DiffLine> = Vec::new();

    for line in lines {
        match line.line_type {
            LineType::Removed => old_lines.push(line),
            LineType::Added => new_lines.push(line),
            LineType::Context => {
                // Flush any pending removed/added pairs
                let max_len = old_lines.len().max(new_lines.len());
                for i in 0..max_len {
                    pairs.push((old_lines.get(i).copied(), new_lines.get(i).copied()));
                }
                old_lines.clear();
                new_lines.clear();

                // Add context line to both sides
                pairs.push((Some(line), Some(line)));
            }
            LineType::Header => {}
        }
    }

    // Flush remaining
    let max_len = old_lines.len().max(new_lines.len());
    for i in 0..max_len {
        pairs.push((old_lines.get(i).copied(), new_lines.get(i).copied()));
    }

    pairs
}

/// Render a file header
fn render_file_header(buf: &mut Buffer, x: u16, y: u16, width: u16, diff: &FileDiff, styles: &Styles) {
    // Fill background
    for i in x..x + width {
        buf[(i, y)].set_char(' ').set_style(styles.file_header);
    }

    let stats = format!(" +{} -{} ", diff.added, diff.removed);
    let path_width = (width as usize).saturating_sub(stats.len() + 2);

    let path = if diff.path.len() > path_width && path_width > 3 {
        format!("...{}", &diff.path[diff.path.len() - path_width + 3..])
    } else {
        diff.path.clone()
    };

    let mut spans = vec![
        Span::styled(format!(" {} ", path), styles.file_header),
    ];

    // Add stats on the right
    let current_len = path.len() + 2;
    if current_len + stats.len() < width as usize {
        let padding = width as usize - current_len - stats.len();
        spans.push(Span::styled(" ".repeat(padding), styles.file_header));
        spans.push(Span::styled(format!("+{}", diff.added), styles.stats_added));
        spans.push(Span::styled(" ", styles.file_header));
        spans.push(Span::styled(format!("-{}", diff.removed), styles.stats_removed));
        spans.push(Span::styled(" ", styles.file_header));
    }

    let line = Line::from(spans);
    buf.set_line(x, y, &line, width);
}

/// Render a hunk header
fn render_hunk_header(buf: &mut Buffer, x: u16, y: u16, width: u16, hunk: &Hunk, styles: &Styles) {
    let header = format!(
        "@@ -{},{} +{},{} @@",
        hunk.old_start, hunk.old_count, hunk.new_start, hunk.new_count
    );

    buf.set_line(x, y, &Line::styled(header, styles.hunk_header), width);
}

/// Render a unified diff line
fn render_unified_line(
    buf: &mut Buffer,
    x: u16,
    y: u16,
    width: u16,
    line: &crate::git::DiffLine,
    _filename: &str,
    _highlighter: &mut Highlighter,
    styles: &Styles,
) {
    let line_num_width: u16 = 6;
    let gutter_width: u16 = 2;

    // Line number
    let lineno = line.new_lineno.or(line.old_lineno).unwrap_or(0);
    let lineno_str = if lineno > 0 {
        format!("{:>5} ", lineno)
    } else {
        "      ".to_string()
    };
    buf.set_line(x, y, &Line::styled(&lineno_str, styles.line_number), line_num_width);

    // Gutter indicator
    let (gutter_char, gutter_style, line_style) = match line.line_type {
        LineType::Added => ("│+", styles.gutter_added, styles.line_added),
        LineType::Removed => ("│-", styles.gutter_removed, styles.line_removed),
        LineType::Context => ("│ ", styles.gutter_context, styles.line_context),
        LineType::Header => ("  ", styles.line_context, styles.hunk_header),
    };
    buf.set_line(
        x + line_num_width,
        y,
        &Line::styled(gutter_char, gutter_style),
        gutter_width,
    );

    // Content
    let content_x = x + line_num_width + gutter_width;
    let content_width = width.saturating_sub(line_num_width + gutter_width);

    // Truncate content if needed
    let content = truncate_str(&line.content, content_width as usize);
    buf.set_line(content_x, y, &Line::styled(content, line_style), content_width);

    // Fill background
    for i in content_x..(content_x + content_width) {
        buf[(i, y)].set_style(line_style);
    }
}

/// Render one side of a side-by-side column
fn render_side_column(
    buf: &mut Buffer,
    x: u16,
    y: u16,
    width: u16,
    line_num_width: u16,
    line: Option<&crate::git::DiffLine>,
    _filename: &str,
    _highlighter: &mut Highlighter,
    styles: &Styles,
    is_old: bool,
) {
    let gutter_width: u16 = 2;

    match line {
        Some(l) => {
            // Line number
            let lineno = if is_old { l.old_lineno } else { l.new_lineno };
            let lineno_str = match lineno {
                Some(n) if n > 0 => format!("{:>5} ", n),
                _ => "      ".to_string(),
            };
            buf.set_line(x, y, &Line::styled(&lineno_str, styles.line_number), line_num_width);

            // Gutter
            let (gutter_char, gutter_style, line_style) = match l.line_type {
                LineType::Added => ("│+", styles.gutter_added, styles.line_added),
                LineType::Removed => ("│-", styles.gutter_removed, styles.line_removed),
                LineType::Context => ("│ ", styles.gutter_context, styles.line_context),
                LineType::Header => ("  ", styles.line_context, styles.hunk_header),
            };
            buf.set_line(
                x + line_num_width,
                y,
                &Line::styled(gutter_char, gutter_style),
                gutter_width,
            );

            // Content
            let content_x = x + line_num_width + gutter_width;
            let content_width = width.saturating_sub(line_num_width + gutter_width);
            let content = truncate_str(&l.content, content_width as usize);
            buf.set_line(content_x, y, &Line::styled(content, line_style), content_width);

            // Fill background
            for i in content_x..(content_x + content_width) {
                buf[(i, y)].set_style(line_style);
            }
        }
        None => {
            // Empty line (no corresponding line on this side)
            for i in x..x + width {
                buf[(i, y)].set_char(' ').set_style(styles.line_context);
            }
        }
    }
}

/// Truncate a string to fit width
fn truncate_str(s: &str, max_width: usize) -> String {
    if s.width() <= max_width {
        s.to_string()
    } else {
        let mut result = String::new();
        let mut width = 0;
        for c in s.chars() {
            let cw = unicode_width::UnicodeWidthChar::width(c).unwrap_or(0);
            if width + cw > max_width {
                break;
            }
            result.push(c);
            width += cw;
        }
        result
    }
}

/// Calculate total number of lines in the diff view
pub fn calculate_total_lines(diffs: &[FileDiff]) -> usize {
    let mut total = 0;

    for diff in diffs {
        total += 1; // File header

        if !diff.collapsed && !diff.is_binary {
            for hunk in &diff.hunks {
                total += 1; // Hunk header

                // Count lines (in side-by-side mode, pairs are rendered on single lines)
                let pairs = pair_lines(&hunk.lines);
                total += pairs.len();
            }
        }
    }

    total
}

/// Render the diff content
pub fn render_diff_content(
    buf: &mut Buffer,
    area: Rect,
    diffs: &[FileDiff],
    scroll: usize,
    mode: DiffMode,
    highlighter: &mut Highlighter,
    focused: bool,
    styles: &Styles,
) {
    let content = DiffContent {
        diffs,
        scroll,
        mode,
        highlighter,
        styles,
        focused,
    };
    content.render(area, buf);
}
