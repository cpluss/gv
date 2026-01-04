//! Diff content rendering
//!
//! Renders the main diff view in side-by-side, unified, or full-file side-by-side modes.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    style::Style,
    text::{Line, Span},
    widgets::Widget,
};
use unicode_width::{UnicodeWidthChar, UnicodeWidthStr};

use crate::git::{FileDiff, Hunk, LineType};
use crate::syntax::{Highlighter, Token};
use super::Styles;

/// Diff display mode
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DiffMode {
    /// Side-by-side view showing old and new columns
    SideBySide,
    /// Unified view showing all changes in one column
    Unified,
    /// Full-file side-by-side view with highlighted changes
    SideBySideFull,
}

/// Diff content widget
pub struct DiffContent<'a> {
    /// List of file diffs to display
    pub diffs: &'a [&'a FileDiff],
    /// Scroll offset (in lines)
    pub scroll: usize,
    /// Current diff mode
    pub mode: DiffMode,
    /// Syntax highlighter
    pub highlighter: &'a mut Highlighter,
    /// Styles
    pub styles: &'a Styles,
}

const TAB_WIDTH: usize = 4;

impl Widget for DiffContent<'_> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        match self.mode {
            DiffMode::Unified => render_unified(self, area, buf),
            DiffMode::SideBySide => render_side_by_side(self, area, buf),
            DiffMode::SideBySideFull => render_side_by_side_full(self, area, buf),
        }
    }
}

/// Render unified diff view
fn render_unified(content: DiffContent<'_>, area: Rect, buf: &mut Buffer) {
    let mut current_line: usize = 0;
    let visible_start = content.scroll;
    let visible_end = content.scroll + area.height as usize;

    for diff in content.diffs.iter().copied() {
        let mut line_index = 0;
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
                        line_index,
                        content.highlighter,
                        content.styles,
                    );
                }
                current_line += 1;
                line_index += 1;

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

    for diff in content.diffs.iter().copied() {
        let mut line_index = 0;
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
            let pairs = pair_lines_with_index(&hunk.lines, line_index);

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

            line_index += hunk.lines.len();
        }
    }
}

/// Render full-file side-by-side diff view
fn render_side_by_side_full(content: DiffContent<'_>, area: Rect, buf: &mut Buffer) {
    let mut current_line: usize = 0;
    let visible_start = content.scroll;
    let visible_end = content.scroll + area.height as usize;

    // Calculate column widths
    let half_width = area.width / 2;
    let line_num_width: u16 = 6;

    for diff in content.diffs.iter().copied() {
        // File header (spans both columns)
        if current_line >= visible_start && current_line < visible_end {
            let y = area.y + (current_line - visible_start) as u16;
            render_file_header(buf, area.x, y, area.width, diff, content.styles);
        }
        current_line += 1;

        if diff.collapsed || diff.is_binary {
            continue;
        }

        let has_full_content = diff.old_content.is_some() || diff.new_content.is_some();
        let old_lines = diff.old_content.as_ref().map(|lines| lines.as_slice()).unwrap_or(&[]);
        let new_lines = diff.new_content.as_ref().map(|lines| lines.as_slice()).unwrap_or(&[]);
        let old_filename = diff.old_path.as_deref().unwrap_or(&diff.path);
        let new_filename = diff.path.as_str();
        let old_cache_key = format!("{}::full::old", old_filename);
        let new_cache_key = format!("{}::full::new", new_filename);

        let mut old_idx = 0usize;
        let mut new_idx = 0usize;

        for hunk in &diff.hunks {
            if has_full_content {
                let old_target = hunk.old_start.saturating_sub(1) as usize;
                let new_target = hunk.new_start.saturating_sub(1) as usize;

                while old_idx < old_target || new_idx < new_target {
                    let old_line = old_lines.get(old_idx).map(|s| s.as_str());
                    let new_line = new_lines.get(new_idx).map(|s| s.as_str());

                    if current_line >= visible_start && current_line < visible_end {
                        let y = area.y + (current_line - visible_start) as u16;
                        render_full_column(
                            buf,
                            area.x,
                            y,
                            half_width,
                            line_num_width,
                            old_line.map(|_| old_idx + 1),
                            old_line,
                            &old_cache_key,
                            old_filename,
                            old_idx,
                            content.highlighter,
                            content.styles.gutter_context,
                            content.styles.line_context,
                            content.styles,
                        );
                        render_full_column(
                            buf,
                            area.x + half_width,
                            y,
                            half_width,
                            line_num_width,
                            new_line.map(|_| new_idx + 1),
                            new_line,
                            &new_cache_key,
                            new_filename,
                            new_idx,
                            content.highlighter,
                            content.styles.gutter_context,
                            content.styles.line_context,
                            content.styles,
                        );
                    }
                    current_line += 1;

                    if old_idx < old_target {
                        old_idx += 1;
                    }
                    if new_idx < new_target {
                        new_idx += 1;
                    }

                    if current_line >= visible_end {
                        return;
                    }
                }
            }

            for line in &hunk.lines {
                if current_line >= visible_start && current_line < visible_end {
                    let y = area.y + (current_line - visible_start) as u16;

                    match line.line_type {
                        LineType::Context => {
                            let old_line = old_lines
                                .get(old_idx)
                                .map(|s| s.as_str())
                                .unwrap_or(line.content.as_str());
                            let new_line = new_lines
                                .get(new_idx)
                                .map(|s| s.as_str())
                                .unwrap_or(line.content.as_str());
                            let old_lineno = line.old_lineno.map(|n| n as usize).unwrap_or(old_idx + 1);
                            let new_lineno = line.new_lineno.map(|n| n as usize).unwrap_or(new_idx + 1);

                            render_full_column(
                                buf,
                                area.x,
                                y,
                                half_width,
                                line_num_width,
                                Some(old_lineno),
                                Some(old_line),
                                &old_cache_key,
                                old_filename,
                                old_idx,
                                content.highlighter,
                                content.styles.gutter_context,
                                content.styles.line_context,
                                content.styles,
                            );
                            render_full_column(
                                buf,
                                area.x + half_width,
                                y,
                                half_width,
                                line_num_width,
                                Some(new_lineno),
                                Some(new_line),
                                &new_cache_key,
                                new_filename,
                                new_idx,
                                content.highlighter,
                                content.styles.gutter_context,
                                content.styles.line_context,
                                content.styles,
                            );
                        }
                        LineType::Removed => {
                            let old_line = old_lines
                                .get(old_idx)
                                .map(|s| s.as_str())
                                .unwrap_or(line.content.as_str());
                            let old_lineno = line.old_lineno.map(|n| n as usize).unwrap_or(old_idx + 1);
                            render_full_column(
                                buf,
                                area.x,
                                y,
                                half_width,
                                line_num_width,
                                Some(old_lineno),
                                Some(old_line),
                                &old_cache_key,
                                old_filename,
                                old_idx,
                                content.highlighter,
                                content.styles.gutter_removed,
                                content.styles.line_removed,
                                content.styles,
                            );
                            render_full_column(
                                buf,
                                area.x + half_width,
                                y,
                                half_width,
                                line_num_width,
                                None,
                                None,
                                &new_cache_key,
                                new_filename,
                                new_idx,
                                content.highlighter,
                                content.styles.gutter_context,
                                content.styles.line_context,
                                content.styles,
                            );
                        }
                        LineType::Added => {
                            let new_line = new_lines
                                .get(new_idx)
                                .map(|s| s.as_str())
                                .unwrap_or(line.content.as_str());
                            let new_lineno = line.new_lineno.map(|n| n as usize).unwrap_or(new_idx + 1);
                            render_full_column(
                                buf,
                                area.x,
                                y,
                                half_width,
                                line_num_width,
                                None,
                                None,
                                &old_cache_key,
                                old_filename,
                                old_idx,
                                content.highlighter,
                                content.styles.gutter_context,
                                content.styles.line_context,
                                content.styles,
                            );
                            render_full_column(
                                buf,
                                area.x + half_width,
                                y,
                                half_width,
                                line_num_width,
                                Some(new_lineno),
                                Some(new_line),
                                &new_cache_key,
                                new_filename,
                                new_idx,
                                content.highlighter,
                                content.styles.gutter_added,
                                content.styles.line_added,
                                content.styles,
                            );
                        }
                        LineType::Header => {}
                    }
                }

                // Always increment indices based on line type, regardless of visibility
                match line.line_type {
                    LineType::Context => {
                        old_idx += 1;
                        new_idx += 1;
                    }
                    LineType::Removed => {
                        old_idx += 1;
                    }
                    LineType::Added => {
                        new_idx += 1;
                    }
                    LineType::Header => {}
                }

                current_line += 1;
                if current_line >= visible_end {
                    return;
                }
            }
        }

        if has_full_content {
            while old_idx < old_lines.len() || new_idx < new_lines.len() {
                let old_line = old_lines.get(old_idx).map(|s| s.as_str());
                let new_line = new_lines.get(new_idx).map(|s| s.as_str());

                if current_line >= visible_start && current_line < visible_end {
                    let y = area.y + (current_line - visible_start) as u16;
                    render_full_column(
                        buf,
                        area.x,
                        y,
                        half_width,
                        line_num_width,
                        old_line.map(|_| old_idx + 1),
                        old_line,
                        &old_cache_key,
                        old_filename,
                        old_idx,
                        content.highlighter,
                        content.styles.gutter_context,
                        content.styles.line_context,
                        content.styles,
                    );
                    render_full_column(
                        buf,
                        area.x + half_width,
                        y,
                        half_width,
                        line_num_width,
                        new_line.map(|_| new_idx + 1),
                        new_line,
                        &new_cache_key,
                        new_filename,
                        new_idx,
                        content.highlighter,
                        content.styles.gutter_context,
                        content.styles.line_context,
                        content.styles,
                    );
                }

                current_line += 1;
                if old_idx < old_lines.len() {
                    old_idx += 1;
                }
                if new_idx < new_lines.len() {
                    new_idx += 1;
                }

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

    for line in lines {
        match line.line_type {
            LineType::Removed => {
                // Removed lines appear on left only
                pairs.push((Some(line), None));
            }
            LineType::Added => {
                // Added lines appear on right only
                pairs.push((None, Some(line)));
            }
            LineType::Context => {
                // Context lines appear on both sides
                pairs.push((Some(line), Some(line)));
            }
            LineType::Header => {}
        }
    }

    pairs
}

#[derive(Clone, Copy)]
struct IndexedLine<'a> {
    line: &'a crate::git::DiffLine,
    index: usize,
}

/// Pair old and new lines for side-by-side display, preserving line indices
fn pair_lines_with_index(lines: &[crate::git::DiffLine], start_index: usize) -> Vec<(Option<IndexedLine<'_>>, Option<IndexedLine<'_>>)> {
    let mut pairs = Vec::new();

    for (offset, line) in lines.iter().enumerate() {
        let indexed = IndexedLine {
            line,
            index: start_index + offset,
        };

        match line.line_type {
            LineType::Removed => {
                pairs.push((Some(indexed), None));
            }
            LineType::Added => {
                pairs.push((None, Some(indexed)));
            }
            LineType::Context => {
                pairs.push((Some(indexed), Some(indexed)));
            }
            LineType::Header => {}
        }
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

    let display_path = if let Some(old_path) = &diff.old_path {
        format!("{} → {}", old_path, diff.path)
    } else {
        diff.path.clone()
    };

    let path = if display_path.len() > path_width && path_width > 3 {
        format!("...{}", &display_path[display_path.len() - path_width + 3..])
    } else {
        display_path
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
    let header = if hunk.header.is_empty() {
        format!(
            "@@ -{},{} +{},{} @@",
            hunk.old_start, hunk.old_count, hunk.new_start, hunk.new_count
        )
    } else {
        hunk.header.clone()
    };

    buf.set_line(x, y, &Line::styled(header, styles.hunk_header), width);
}

/// Render a unified diff line
fn render_unified_line(
    buf: &mut Buffer,
    x: u16,
    y: u16,
    width: u16,
    line: &crate::git::DiffLine,
    filename: &str,
    line_index: usize,
    highlighter: &mut Highlighter,
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
        LineType::Added => ("│ ", styles.gutter_added, styles.line_added),
        LineType::Removed => ("│ ", styles.gutter_removed, styles.line_removed),
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

    if line.line_type == LineType::Header {
        let content = truncate_str(&line.content, content_width as usize);
        buf.set_line(content_x, y, &Line::styled(content, styles.hunk_header), content_width);
        return;
    }

    for i in content_x..(content_x + content_width) {
        buf[(i, y)].set_char(' ').set_style(line_style);
    }

    let spans = highlight_spans(
        filename,
        filename,
        line_index,
        &line.content,
        highlighter,
        line_style,
    );

    let content_line = Line::from(spans);
    buf.set_line(content_x, y, &content_line, content_width);
}

/// Render one side of a side-by-side column
fn render_side_column(
    buf: &mut Buffer,
    x: u16,
    y: u16,
    width: u16,
    line_num_width: u16,
    line: Option<IndexedLine<'_>>,
    filename: &str,
    highlighter: &mut Highlighter,
    styles: &Styles,
    is_old: bool,
) {
    let gutter_width: u16 = 2;

    match line {
        Some(indexed) => {
            let l = indexed.line;
            // Line number
            let lineno = if is_old { l.old_lineno } else { l.new_lineno };
            let lineno_str = match lineno {
                Some(n) if n > 0 => format!("{:>5} ", n),
                _ => "      ".to_string(),
            };
            buf.set_line(x, y, &Line::styled(&lineno_str, styles.line_number), line_num_width);

            // Gutter
            let (gutter_char, gutter_style, line_style) = match l.line_type {
                LineType::Added => ("│ ", styles.gutter_added, styles.line_added),
                LineType::Removed => ("│ ", styles.gutter_removed, styles.line_removed),
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

            if l.line_type == LineType::Header {
                let content = truncate_str(&l.content, content_width as usize);
                buf.set_line(content_x, y, &Line::styled(content, styles.hunk_header), content_width);
                return;
            }

            for i in content_x..(content_x + content_width) {
                buf[(i, y)].set_char(' ').set_style(line_style);
            }

            let spans = highlight_spans(
                filename,
                filename,
                indexed.index,
                &l.content,
                highlighter,
                line_style,
            );
            let content_line = Line::from(spans);
            buf.set_line(content_x, y, &content_line, content_width);
        }
        None => {
            // Empty line (no corresponding line on this side)
            for i in x..x + width {
                buf[(i, y)].set_char(' ').set_style(styles.line_context);
            }
        }
    }
}

/// Render one column in full-file mode
fn render_full_column(
    buf: &mut Buffer,
    x: u16,
    y: u16,
    width: u16,
    line_num_width: u16,
    lineno: Option<usize>,
    content: Option<&str>,
    cache_key: &str,
    filename: &str,
    line_index: usize,
    highlighter: &mut Highlighter,
    gutter_style: Style,
    line_style: Style,
    styles: &Styles,
) {
    let gutter_width: u16 = 2;

    if let Some(content) = content {
        let lineno_str = match lineno {
            Some(n) if n > 0 => format!("{:>5} ", n),
            _ => "      ".to_string(),
        };
        buf.set_line(x, y, &Line::styled(&lineno_str, styles.line_number), line_num_width);
        buf.set_line(
            x + line_num_width,
            y,
            &Line::styled("│ ", gutter_style),
            gutter_width,
        );

        let content_x = x + line_num_width + gutter_width;
        let content_width = width.saturating_sub(line_num_width + gutter_width);

        for i in content_x..(content_x + content_width) {
            buf[(i, y)].set_char(' ').set_style(line_style);
        }

        let spans = highlight_spans(
            cache_key,
            filename,
            line_index,
            content,
            highlighter,
            line_style,
        );
        let content_line = Line::from(spans);
        buf.set_line(content_x, y, &content_line, content_width);
    } else {
        for i in x..x + width {
            buf[(i, y)].set_char(' ').set_style(styles.line_context);
        }
    }
}

fn highlight_spans(
    cache_key: &str,
    filename: &str,
    line_index: usize,
    content: &str,
    highlighter: &mut Highlighter,
    base_style: Style,
) -> Vec<Span<'static>> {
    let tokens = highlighter.get_line(cache_key, filename, line_index, content);
    if tokens.is_empty() {
        let expanded = expand_tabs(content, TAB_WIDTH);
        return vec![Span::styled(expanded, base_style)];
    }

    let expanded_tokens = expand_tabs_tokens(&tokens, TAB_WIDTH);
    expanded_tokens
        .into_iter()
        .map(|token| Span::styled(token.text, base_style.patch(token.style)))
        .collect()
}

fn expand_tabs_tokens(tokens: &[Token], tab_width: usize) -> Vec<Token> {
    let mut expanded = Vec::new();
    let mut col = 0usize;

    for token in tokens {
        let mut text = String::new();
        for ch in token.text.chars() {
            if ch == '\t' {
                let spaces = tab_width.saturating_sub(col % tab_width).max(1);
                text.extend(std::iter::repeat(' ').take(spaces));
                col += spaces;
            } else {
                text.push(ch);
                col += UnicodeWidthChar::width(ch).unwrap_or(0);
            }
        }

        if !text.is_empty() {
            expanded.push(Token {
                text,
                style: token.style,
            });
        }
    }

    expanded
}

fn expand_tabs(content: &str, tab_width: usize) -> String {
    let mut expanded = String::new();
    let mut col = 0usize;

    for ch in content.chars() {
        if ch == '\t' {
            let spaces = tab_width.saturating_sub(col % tab_width).max(1);
            expanded.extend(std::iter::repeat(' ').take(spaces));
            col += spaces;
        } else {
            expanded.push(ch);
            col += UnicodeWidthChar::width(ch).unwrap_or(0);
        }
    }

    expanded
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
pub fn calculate_total_lines(diffs: &[&FileDiff], mode: DiffMode) -> usize {
    diffs.iter().map(|diff| file_line_count(*diff, mode)).sum()
}

pub fn file_line_count(diff: &FileDiff, mode: DiffMode) -> usize {
    let mut total = 1; // File header

    if diff.collapsed || diff.is_binary {
        return total;
    }

    match mode {
        DiffMode::SideBySide | DiffMode::Unified => {
            for hunk in &diff.hunks {
                total += 1; // Hunk header
                let pairs = pair_lines(&hunk.lines);
                total += pairs.len();
            }
        }
        DiffMode::SideBySideFull => {
            total += full_line_count(diff);
        }
    }

    total
}

fn full_line_count(diff: &FileDiff) -> usize {
    let old_len = diff.old_content.as_ref().map(|lines| lines.len()).unwrap_or(0);
    let new_len = diff.new_content.as_ref().map(|lines| lines.len()).unwrap_or(0);

    if diff.old_content.is_none() && diff.new_content.is_none() {
        return diff.hunks.iter().map(|h| pair_lines(&h.lines).len()).sum();
    }

    if old_len >= new_len {
        old_len.saturating_add(diff.added)
    } else {
        new_len.saturating_add(diff.removed)
    }
}

/// Render the diff content
pub fn render_diff_content(
    buf: &mut Buffer,
    area: Rect,
    diffs: &[&FileDiff],
    scroll: usize,
    mode: DiffMode,
    highlighter: &mut Highlighter,
    styles: &Styles,
) {
    let content = DiffContent {
        diffs,
        scroll,
        mode,
        highlighter,
        styles,
    };
    content.render(area, buf);
}
