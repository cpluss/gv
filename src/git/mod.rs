//! Git operations module
//!
//! Provides functionality for interacting with git repositories:
//! - Worktree discovery and management
//! - Diff computation with context lines
//! - Commit listing and filtering

mod worktree;
mod diff;
mod commits;

pub use worktree::{Worktree, list_worktrees, find_current_worktree, get_main_branch};
pub use diff::{FileDiff, Hunk, DiffLine, LineType, compute_diff, compute_stats};
pub use commits::{Commit, list_commits};
