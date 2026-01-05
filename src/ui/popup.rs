//! Popup overlays
//!
//! Commit filter, worktree switcher, and help overlay.

use ratatui::{
    buffer::Buffer,
    layout::Rect,
    text::{Line, Span},
    widgets::{Block, Borders, Clear, Widget},
};

use crate::git::{Commit, Worktree};
use super::Styles;

/// Render a centered popup overlay
fn render_centered_popup(buf: &mut Buffer, area: Rect, width: u16, height: u16, title: &str, styles: &Styles) -> Rect {
    // Calculate centered position
    let popup_x = area.x + (area.width.saturating_sub(width)) / 2;
    let popup_y = area.y + (area.height.saturating_sub(height)) / 2;

    let popup_area = Rect::new(
        popup_x,
        popup_y,
        width.min(area.width),
        height.min(area.height),
    );

    // Clear the popup area
    Clear.render(popup_area, buf);

    // Draw border
    let block = Block::default()
        .borders(Borders::ALL)
        .border_style(styles.border_focus)
        .title(Span::styled(format!(" {} ", title), styles.popup_title))
        .style(styles.popup);

    let inner = block.inner(popup_area);
    block.render(popup_area, buf);

    inner
}

/// Render commit filter popup
pub fn render_commit_popup(
    buf: &mut Buffer,
    area: Rect,
    commits: &[Commit],
    cursor: usize,
    styles: &Styles,
) {
    let width = 60.min(area.width - 4);
    let height = (commits.len() as u16 + 4).min(area.height - 4);

    let inner = render_centered_popup(buf, area, width, height, "Select Commits", styles);

    // Instructions
    let instructions = "Space: toggle  a: all  n: none  Enter: apply  Esc: cancel";
    buf.set_line(
        inner.x,
        inner.y,
        &Line::styled(instructions, styles.footer),
        inner.width,
    );

    // Separator
    buf.set_line(
        inner.x,
        inner.y + 1,
        &Line::styled("─".repeat(inner.width as usize), styles.border),
        inner.width,
    );

    // Commits list
    for (i, commit) in commits.iter().enumerate() {
        let y = inner.y + 2 + i as u16;
        if y >= inner.y + inner.height {
            break;
        }

        let is_cursor = i == cursor;
        let style = if is_cursor {
            styles.sidebar_cursor
        } else {
            styles.sidebar_normal
        };

        let checkbox = if commit.selected { "[x]" } else { "[ ]" };
        let hash = if commit.is_uncommitted {
            "-------".to_string()
        } else {
            commit.hash.clone()
        };

        let subject = truncate(&commit.subject, (inner.width as usize).saturating_sub(15));

        let line = Line::from(vec![
            Span::styled(format!(" {} ", checkbox), style),
            Span::styled(format!("{} ", hash), styles.worktree_branch),
            Span::styled(subject, style),
        ]);

        buf.set_line(inner.x, y, &line, inner.width);

        if is_cursor {
            for x in inner.x..inner.x + inner.width {
                buf[(x, y)].set_style(style);
            }
        }
    }
}

/// Render worktree switcher popup
pub fn render_worktree_popup(
    buf: &mut Buffer,
    area: Rect,
    worktrees: &[Worktree],
    cursor: usize,
    filter: &str,
    styles: &Styles,
) {
    let width = 70.min(area.width - 4);
    let height = (worktrees.len() as u16 + 5).min(area.height - 4);

    let inner = render_centered_popup(buf, area, width, height, "Switch Worktree", styles);

    // Filter input
    let filter_line = format!("> {}", filter);
    buf.set_line(inner.x, inner.y, &Line::styled(&filter_line, styles.popup_title), inner.width);

    // Separator
    buf.set_line(
        inner.x,
        inner.y + 1,
        &Line::styled("─".repeat(inner.width as usize), styles.border),
        inner.width,
    );

    // Worktrees list
    let filtered: Vec<_> = worktrees
        .iter()
        .filter(|wt| {
            filter.is_empty()
                || wt.path.to_string_lossy().to_lowercase().contains(&filter.to_lowercase())
                || wt.branch.as_ref().map_or(false, |b| b.to_lowercase().contains(&filter.to_lowercase()))
        })
        .collect();

    for (i, wt) in filtered.iter().enumerate() {
        let y = inner.y + 2 + i as u16;
        if y >= inner.y + inner.height {
            break;
        }

        let is_cursor = i == cursor;
        let style = if is_cursor {
            styles.sidebar_cursor
        } else {
            styles.sidebar_normal
        };

        let branch = wt.branch.as_deref().unwrap_or("(detached)");
        let path = wt.path.to_string_lossy();
        let path_display = truncate(&path, (inner.width as usize).saturating_sub(branch.len() + 10));

        let mut spans = vec![Span::styled(" ", style)];

        if wt.is_current {
            spans.push(Span::styled("* ", styles.worktree_current));
        } else {
            spans.push(Span::styled("  ", style));
        }

        spans.push(Span::styled(format!("{:<20} ", branch), styles.worktree_branch));
        spans.push(Span::styled(path_display, styles.worktree_path));

        let line = Line::from(spans);
        buf.set_line(inner.x, y, &line, inner.width);

        if is_cursor {
            for x in inner.x..inner.x + inner.width {
                buf[(x, y)].set_style(style);
            }
        }
    }
}

/// Render help overlay
pub fn render_help_popup(buf: &mut Buffer, area: Rect, styles: &Styles) {
    let width = 50.min(area.width - 4);
    let height = 24.min(area.height - 4);

    let inner = render_centered_popup(buf, area, width, height, "Help", styles);

    let help_items = [
        ("Navigation", ""),
        ("j/k", "Scroll down/up"),
        ("Ctrl+d/u", "Page down/up"),
        ("g/G", "Go to top/bottom"),
        ("n/N", "Next/previous file"),
        ("Enter", "Jump to file (sidebar)"),
        ("Tab", "Switch focus"),
        ("", ""),
        ("View", ""),
        ("u", "Cycle view (split/unified/full)"),
        ("x", "Cycle context lines"),
        ("[/]", "Resize sidebar (or drag border)"),
        ("/", "Search files"),
        ("Space", "Collapse/expand file"),
        ("z", "Collapse/expand all"),
        ("h", "Toggle hidden files"),
        ("", ""),
        ("Filters", ""),
        ("c", "Commit filter"),
        ("w", "Worktree switcher"),
        ("W", "Worktree list"),
        ("", ""),
        ("?", "Toggle this help"),
        ("q", "Quit"),
    ];

    for (i, (key, desc)) in help_items.iter().enumerate() {
        let y = inner.y + i as u16;
        if y >= inner.y + inner.height {
            break;
        }

        if key.is_empty() && desc.is_empty() {
            continue;
        }

        if desc.is_empty() {
            // Section header
            buf.set_line(
                inner.x,
                y,
                &Line::styled(format!(" {}", key), styles.popup_title),
                inner.width,
            );
        } else {
            // Key/desc pair
            let line = Line::from(vec![
                Span::styled(format!("  {:>12} ", key), styles.help_key),
                Span::styled(*desc, styles.help_desc),
            ]);
            buf.set_line(inner.x, y, &line, inner.width);
        }
    }
}

/// Truncate a string
fn truncate(s: &str, max: usize) -> String {
    if s.len() <= max {
        s.to_string()
    } else if max > 3 {
        format!("{}...", &s[..max - 3])
    } else {
        s[..max].to_string()
    }
}
