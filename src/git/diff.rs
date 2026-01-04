//! Git diff computation
//!
//! Computes diffs between commits or the working directory,
//! parsing the output into structured data for display.

use std::path::Path;
use anyhow::{Context, Result};
use git2::{Diff, DiffOptions, Repository, DiffFormat, DiffDelta, DiffHunk, DiffLine as Git2DiffLine};

/// Type of a diff line
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LineType {
    /// Line exists in both old and new (context line)
    Context,
    /// Line was added
    Added,
    /// Line was removed
    Removed,
    /// Hunk header (@@...@@)
    Header,
}

/// A single line in a diff
#[derive(Debug, Clone)]
pub struct DiffLine {
    /// The type of this line
    pub line_type: LineType,
    /// The content of the line (without +/- prefix)
    pub content: String,
    /// Line number in the old file (if applicable)
    pub old_lineno: Option<u32>,
    /// Line number in the new file (if applicable)
    pub new_lineno: Option<u32>,
}

/// A hunk (section) of a diff
#[derive(Debug, Clone)]
pub struct Hunk {
    /// Starting line in old file
    pub old_start: u32,
    /// Number of lines in old file
    pub old_count: u32,
    /// Starting line in new file
    pub new_start: u32,
    /// Number of lines in new file
    pub new_count: u32,
    /// The header text (@@...@@)
    pub header: String,
    /// Lines in this hunk
    pub lines: Vec<DiffLine>,
}

/// Diff for a single file
#[derive(Debug, Clone)]
pub struct FileDiff {
    /// Path to the file (new path if renamed)
    pub path: String,
    /// Old path (if renamed/moved)
    pub old_path: Option<String>,
    /// Lines added
    pub added: usize,
    /// Lines removed
    pub removed: usize,
    /// Hunks in this file
    pub hunks: Vec<Hunk>,
    /// Whether the file is collapsed in the UI
    pub collapsed: bool,
    /// Whether this is a binary file
    pub is_binary: bool,
}

/// Compute diff between base branch and HEAD (or working directory)
///
/// # Arguments
/// * `repo_path` - Path to the repository
/// * `base_branch` - The base branch to diff against (e.g., "origin/main")
/// * `include_uncommitted` - Whether to include uncommitted changes
/// * `selected_commits` - Specific commit hashes to include (empty = all)
/// * `context_lines` - Number of context lines around changes
pub fn compute_diff(
    repo_path: &Path,
    base_branch: &str,
    include_uncommitted: bool,
    selected_commits: &[String],
    context_lines: u32,
) -> Result<Vec<FileDiff>> {
    let repo = Repository::discover(repo_path)
        .context("Failed to discover git repository")?;

    let mut opts = DiffOptions::new();
    opts.context_lines(context_lines);
    opts.ignore_whitespace_change(false);

    // Determine what to diff
    let diff = if include_uncommitted && selected_commits.is_empty() {
        // Diff HEAD against working directory
        let head_tree = repo.head()?.peel_to_tree()?;
        repo.diff_tree_to_workdir_with_index(Some(&head_tree), Some(&mut opts))?
    } else if include_uncommitted {
        // Diff base branch against working directory
        let base_obj = repo.revparse_single(base_branch)?;
        let base_tree = base_obj.peel_to_tree()?;
        repo.diff_tree_to_workdir_with_index(Some(&base_tree), Some(&mut opts))?
    } else if !selected_commits.is_empty() {
        // Diff base branch against HEAD
        let base_obj = repo.revparse_single(base_branch)?;
        let base_tree = base_obj.peel_to_tree()?;
        let head_tree = repo.head()?.peel_to_tree()?;
        repo.diff_tree_to_tree(Some(&base_tree), Some(&head_tree), Some(&mut opts))?
    } else {
        // No changes to show
        return Ok(Vec::new());
    };

    parse_diff(&diff)
}

/// Parse a git2 Diff into our FileDiff structures
fn parse_diff(diff: &Diff) -> Result<Vec<FileDiff>> {
    let mut files: Vec<FileDiff> = Vec::new();
    let mut current_file: Option<FileDiff> = None;
    let mut current_hunk: Option<Hunk> = None;

    diff.print(DiffFormat::Patch, |delta, hunk, line| {
        // Handle file changes
        if let Some(new_file) = delta.new_file().path() {
            let new_path = new_file.to_string_lossy().to_string();

            // Check if we need to start a new file
            let should_start_new = current_file.as_ref()
                .map_or(true, |f| f.path != new_path);

            if should_start_new {
                // Save previous hunk and file
                if let Some(h) = current_hunk.take() {
                    if let Some(ref mut f) = current_file {
                        f.hunks.push(h);
                    }
                }
                if let Some(f) = current_file.take() {
                    files.push(f);
                }

                // Start new file
                let old_path = delta.old_file().path()
                    .map(|p| p.to_string_lossy().to_string())
                    .filter(|p| p != &new_path);

                current_file = Some(FileDiff {
                    path: new_path,
                    old_path,
                    added: 0,
                    removed: 0,
                    hunks: Vec::new(),
                    collapsed: false,
                    is_binary: delta.flags().is_binary(),
                });
            }
        }

        // Handle hunks
        if let Some(h) = hunk {
            // Save previous hunk
            if let Some(prev_hunk) = current_hunk.take() {
                if let Some(ref mut f) = current_file {
                    f.hunks.push(prev_hunk);
                }
            }

            // Start new hunk
            let header = String::from_utf8_lossy(h.header()).to_string();
            current_hunk = Some(Hunk {
                old_start: h.old_start(),
                old_count: h.old_lines(),
                new_start: h.new_start(),
                new_count: h.new_lines(),
                header: header.trim().to_string(),
                lines: Vec::new(),
            });
        }

        // Handle lines
        let origin = line.origin();
        let (line_type, update_stats) = match origin {
            '+' => (LineType::Added, true),
            '-' => (LineType::Removed, true),
            ' ' => (LineType::Context, false),
            _ => return true, // Skip other line types
        };

        let content = String::from_utf8_lossy(line.content()).to_string();
        let diff_line = DiffLine {
            line_type,
            content: content.trim_end_matches('\n').to_string(),
            old_lineno: line.old_lineno(),
            new_lineno: line.new_lineno(),
        };

        if let Some(ref mut h) = current_hunk {
            h.lines.push(diff_line);
        }

        // Update stats
        if update_stats {
            if let Some(ref mut f) = current_file {
                match line_type {
                    LineType::Added => f.added += 1,
                    LineType::Removed => f.removed += 1,
                    _ => {}
                }
            }
        }

        true
    })?;

    // Save final hunk and file
    if let Some(h) = current_hunk {
        if let Some(ref mut f) = current_file {
            f.hunks.push(h);
        }
    }
    if let Some(f) = current_file {
        files.push(f);
    }

    Ok(files)
}

/// Compute aggregate stats for a list of diffs
pub fn compute_stats(diffs: &[FileDiff]) -> (usize, usize) {
    let added: usize = diffs.iter().map(|d| d.added).sum();
    let removed: usize = diffs.iter().map(|d| d.removed).sum();
    (added, removed)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_line_type() {
        assert_eq!(LineType::Added, LineType::Added);
        assert_ne!(LineType::Added, LineType::Removed);
    }
}
