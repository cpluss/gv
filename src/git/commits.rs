//! Git commit listing and filtering
//!
//! Lists commits between the base branch and HEAD,
//! and detects uncommitted changes.

use std::collections::HashSet;
use std::path::Path;
use anyhow::{Context, Result};
use git2::{Repository, Oid, StatusOptions};

/// Represents a git commit
#[derive(Debug, Clone)]
pub struct Commit {
    /// Abbreviated commit hash (7 characters)
    pub hash: String,
    /// Full commit hash
    pub full_hash: String,
    /// Commit subject (first line of message)
    pub subject: String,
    /// Whether this commit is selected for display
    pub selected: bool,
    /// Virtual entry for uncommitted changes
    pub is_uncommitted: bool,
}

/// List commits between base branch and HEAD
///
/// Returns commits that are reachable from HEAD but not from the base branch.
/// Also includes a virtual "uncommitted" entry if there are working directory changes.
pub fn list_commits(repo_path: &Path, base_branch: &str) -> Result<Vec<Commit>> {
    let repo = Repository::discover(repo_path)
        .context("Failed to discover git repository")?;

    let mut commits = Vec::new();

    // Add uncommitted changes entry if applicable
    if has_uncommitted_changes(repo_path)? {
        commits.push(Commit {
            hash: "-------".to_string(),
            full_hash: String::new(),
            subject: "(uncommitted changes)".to_string(),
            selected: true,
            is_uncommitted: true,
        });
    }

    // Get the base branch commit
    let base_oid = match repo.revparse_single(base_branch) {
        Ok(obj) => obj.id(),
        Err(_) => {
            // Base branch doesn't exist, return just uncommitted
            return Ok(commits);
        }
    };

    // Get HEAD commit
    let head_oid = match repo.head() {
        Ok(head) => match head.target() {
            Some(oid) => oid,
            None => return Ok(commits),
        },
        Err(_) => return Ok(commits),
    };

    // Build set of commits reachable from base
    let base_commits = build_commit_set(&repo, base_oid)?;

    // Walk from HEAD and collect commits not in base
    let mut revwalk = repo.revwalk()?;
    revwalk.push(head_oid)?;
    revwalk.set_sorting(git2::Sort::TOPOLOGICAL)?;

    for oid_result in revwalk {
        let oid = oid_result?;

        // Stop if we hit a commit that's in the base
        if base_commits.contains(&oid) {
            break;
        }

        let commit = repo.find_commit(oid)?;
        let hash = oid.to_string();

        commits.push(Commit {
            hash: hash[..7].to_string(),
            full_hash: hash,
            subject: commit.summary().unwrap_or("").to_string(),
            selected: true,
            is_uncommitted: false,
        });
    }

    Ok(commits)
}

/// Check if there are uncommitted changes in the working directory
pub fn has_uncommitted_changes(repo_path: &Path) -> Result<bool> {
    let repo = Repository::discover(repo_path)
        .context("Failed to discover git repository")?;

    let mut opts = StatusOptions::new();
    opts.include_untracked(true);
    opts.include_ignored(false);

    let statuses = repo.statuses(Some(&mut opts))?;

    // Check if there are any changes
    Ok(!statuses.is_empty())
}

/// Build a set of all commits reachable from a given OID
fn build_commit_set(repo: &Repository, start: Oid) -> Result<HashSet<Oid>> {
    let mut set = HashSet::new();
    let mut revwalk = repo.revwalk()?;
    revwalk.push(start)?;

    // Limit to prevent infinite traversal on large repos
    const MAX_COMMITS: usize = 10000;

    for (i, oid_result) in revwalk.enumerate() {
        if i >= MAX_COMMITS {
            break;
        }
        if let Ok(oid) = oid_result {
            set.insert(oid);
        }
    }

    Ok(set)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_commit_struct() {
        let commit = Commit {
            hash: "abc1234".to_string(),
            full_hash: "abc1234567890".to_string(),
            subject: "Test commit".to_string(),
            selected: true,
            is_uncommitted: false,
        };

        assert_eq!(commit.hash, "abc1234");
        assert!(!commit.is_uncommitted);
    }
}
