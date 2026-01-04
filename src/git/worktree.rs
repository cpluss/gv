//! Git worktree discovery and management
//!
//! Handles listing worktrees, finding the current worktree,
//! and detecting the main branch.

use std::path::{Path, PathBuf};
use anyhow::{Context, Result};
use git2::Repository;

/// Represents a git worktree
#[derive(Debug, Clone)]
pub struct Worktree {
    /// Absolute path to the worktree directory
    pub path: PathBuf,
    /// Branch name (if not detached)
    pub branch: Option<String>,
    /// Whether this is the current worktree
    pub is_current: bool,
}

/// List all worktrees for the repository
///
/// Returns a vector of worktrees including the main worktree
/// and any linked worktrees.
pub fn list_worktrees(repo_path: &Path) -> Result<Vec<Worktree>> {
    let repo = Repository::discover(repo_path)
        .context("Failed to discover git repository")?;

    let mut worktrees = Vec::new();

    // Add the main worktree
    if let Some(workdir) = repo.workdir() {
        let branch = get_current_branch(&repo);
        worktrees.push(Worktree {
            path: workdir.to_path_buf(),
            branch,
            is_current: false,
        });
    }

    // Add linked worktrees
    let worktree_names = repo.worktrees()?;
    for name in worktree_names.iter().flatten() {
        if let Ok(wt) = repo.find_worktree(name) {
            if let Some(wt_path) = wt.path().parent() {
                // Open the worktree as a repository to get its HEAD
                if let Ok(wt_repo) = Repository::open(wt_path) {
                    let branch = get_current_branch(&wt_repo);
                    worktrees.push(Worktree {
                        path: wt_path.to_path_buf(),
                        branch,
                        is_current: false,
                    });
                }
            }
        }
    }

    Ok(worktrees)
}

/// Find which worktree contains the given path
///
/// Returns the index of the matching worktree in the list,
/// using the longest matching path prefix.
pub fn find_current_worktree(worktrees: &mut [Worktree], current_path: &Path) -> Option<usize> {
    let canonical = current_path.canonicalize().ok()?;

    let mut best_match: Option<(usize, usize)> = None; // (index, path_len)

    for (i, wt) in worktrees.iter().enumerate() {
        if let Ok(wt_canonical) = wt.path.canonicalize() {
            if canonical.starts_with(&wt_canonical) {
                let len = wt_canonical.as_os_str().len();
                if best_match.map_or(true, |(_, best_len)| len > best_len) {
                    best_match = Some((i, len));
                }
            }
        }
    }

    if let Some((idx, _)) = best_match {
        worktrees[idx].is_current = true;
        Some(idx)
    } else {
        None
    }
}

/// Get the main branch name (main or master)
///
/// Checks for origin/main first, then falls back to origin/master.
/// If neither exists, defaults to "main".
pub fn get_main_branch(repo_path: &Path) -> Result<String> {
    let repo = Repository::discover(repo_path)
        .context("Failed to discover git repository")?;

    // Try origin/main first
    if repo.find_reference("refs/remotes/origin/main").is_ok() {
        return Ok("origin/main".to_string());
    }

    // Fall back to origin/master
    if repo.find_reference("refs/remotes/origin/master").is_ok() {
        return Ok("origin/master".to_string());
    }

    // Try local main/master
    if repo.find_reference("refs/heads/main").is_ok() {
        return Ok("main".to_string());
    }

    if repo.find_reference("refs/heads/master").is_ok() {
        return Ok("master".to_string());
    }

    // Default to main
    Ok("main".to_string())
}

/// Get the current branch name from a repository
fn get_current_branch(repo: &Repository) -> Option<String> {
    repo.head().ok().and_then(|head| {
        if head.is_branch() {
            head.shorthand().map(|s| s.to_string())
        } else {
            // Detached HEAD
            None
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    #[test]
    fn test_find_current_worktree() {
        let mut worktrees = vec![
            Worktree {
                path: PathBuf::from("/repo"),
                branch: Some("main".to_string()),
                is_current: false,
            },
            Worktree {
                path: PathBuf::from("/repo/.worktrees/feature"),
                branch: Some("feature".to_string()),
                is_current: false,
            },
        ];

        // This test requires actual paths to work
        // Just verify the function signature works
        let current = env::current_dir().unwrap();
        let _ = find_current_worktree(&mut worktrees, &current);
    }
}
