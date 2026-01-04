//! File tree structure
//!
//! Builds a tree of files and folders from a list of file paths,
//! supporting collapsible folders and path disambiguation.

use std::collections::HashMap;
use crate::git::FileDiff;

/// Lock files that are considered hidden
const HIDDEN_PATTERNS: &[&str] = &[
    "go.sum",
    "package-lock.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "Cargo.lock",
    "Gemfile.lock",
    "poetry.lock",
    "composer.lock",
];

/// Check if a file path is considered hidden (dotfile or lock file)
pub fn is_hidden_file(path: &str) -> bool {
    // Check for dotfiles/dotfolders (any path component starting with ".")
    if path.split('/').any(|part| part.starts_with('.')) {
        return true;
    }

    // Check against specific hidden patterns
    let filename = path.split('/').last().unwrap_or(path);
    HIDDEN_PATTERNS.iter().any(|p| filename == *p)
}

/// A node in the file tree
#[derive(Debug, Clone)]
pub struct TreeNode {
    /// Display name for this node
    pub name: String,
    /// Full path to this file/folder
    pub path: String,
    /// Whether this is a folder (vs a file)
    pub is_folder: bool,
    /// Depth in the tree (for indentation)
    pub depth: usize,
    /// Aggregated lines added (for folders, sum of children)
    pub added: usize,
    /// Aggregated lines removed
    pub removed: usize,
    /// Index into the diffs array (for files only)
    pub diff_index: Option<usize>,
    /// Whether this folder is expanded
    pub expanded: bool,
    /// Whether this is a hidden file (dotfile or lock file)
    pub is_hidden: bool,
}

/// Build a file tree from a list of diffs
pub fn build_file_tree(diffs: &[FileDiff], expanded_folders: &HashMap<String, bool>) -> Vec<TreeNode> {
    if diffs.is_empty() {
        return Vec::new();
    }

    // Create folder nodes and file nodes
    let mut folders: HashMap<String, (usize, usize)> = HashMap::new(); // path -> (added, removed)
    let mut all_nodes: Vec<TreeNode> = Vec::new();

    // First pass: collect all folders and their stats
    for (i, diff) in diffs.iter().enumerate() {
        let parts: Vec<&str> = diff.path.split('/').collect();

        // Add folder entries
        let mut current_path = String::new();
        for (_depth, part) in parts.iter().take(parts.len() - 1).enumerate() {
            if !current_path.is_empty() {
                current_path.push('/');
            }
            current_path.push_str(part);

            let entry = folders.entry(current_path.clone()).or_insert((0, 0));
            entry.0 += diff.added;
            entry.1 += diff.removed;
        }

        // Add file entry
        all_nodes.push(TreeNode {
            name: parts.last().unwrap_or(&"").to_string(),
            path: diff.path.clone(),
            is_folder: false,
            depth: parts.len() - 1,
            added: diff.added,
            removed: diff.removed,
            diff_index: Some(i),
            expanded: false,
            is_hidden: is_hidden_file(&diff.path),
        });
    }

    // Convert folders to nodes
    let mut folder_nodes: Vec<TreeNode> = folders
        .into_iter()
        .map(|(path, (added, removed))| {
            let depth = path.matches('/').count();
            let name = path.split('/').last().unwrap_or(&path).to_string();
            let expanded = expanded_folders.get(&path).copied().unwrap_or(true);

            TreeNode {
                name,
                path: path.clone(),
                is_folder: true,
                depth,
                added,
                removed,
                diff_index: None,
                expanded,
                is_hidden: is_hidden_file(&path),
            }
        })
        .collect();

    // Combine and sort
    folder_nodes.extend(all_nodes);
    folder_nodes.sort_by(|a, b| a.path.cmp(&b.path));

    folder_nodes
}

/// Flatten the tree for display, respecting collapsed folders
pub fn flatten_tree(nodes: &[TreeNode]) -> Vec<&TreeNode> {
    let mut result = Vec::new();
    let mut collapsed_prefixes: Vec<String> = Vec::new();

    for node in nodes {
        // Check if this node is under a collapsed folder
        let is_hidden = collapsed_prefixes.iter().any(|prefix| {
            node.path.starts_with(prefix) && node.path != *prefix
        });

        if is_hidden {
            continue;
        }

        result.push(node);

        // If this is a collapsed folder, add it to the prefix list
        if node.is_folder && !node.expanded {
            collapsed_prefixes.push(format!("{}/", node.path));
        }
    }

    result
}

/// Get display names for files, disambiguating duplicates
#[allow(dead_code)]
pub fn get_display_names(diffs: &[FileDiff]) -> HashMap<String, String> {
    let mut names: HashMap<String, String> = HashMap::new();
    let mut basename_counts: HashMap<String, Vec<String>> = HashMap::new();

    // Group files by basename
    for diff in diffs {
        let basename = diff.path.split('/').last().unwrap_or(&diff.path).to_string();
        basename_counts
            .entry(basename)
            .or_default()
            .push(diff.path.clone());
    }

    // Assign display names
    for diff in diffs {
        let basename = diff.path.split('/').last().unwrap_or(&diff.path).to_string();

        if let Some(paths) = basename_counts.get(&basename) {
            if paths.len() > 1 {
                // Find unique suffix
                let display = find_unique_suffix(&diff.path, paths);
                names.insert(diff.path.clone(), display);
            } else {
                names.insert(diff.path.clone(), basename);
            }
        }
    }

    names
}

/// Find the shortest unique suffix for a path among a set of paths
#[allow(dead_code)]
fn find_unique_suffix(path: &str, all_paths: &[String]) -> String {
    let parts: Vec<&str> = path.split('/').collect();

    for i in (0..parts.len()).rev() {
        let suffix: String = parts[i..].join("/");
        let matching = all_paths
            .iter()
            .filter(|p| p.ends_with(&suffix))
            .count();

        if matching == 1 {
            return suffix;
        }
    }

    path.to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_get_display_names() {
        let diffs = vec![
            FileDiff {
                path: "src/components/Button.tsx".to_string(),
                old_path: None,
                old_content: None,
                new_content: None,
                added: 10,
                removed: 5,
                hunks: vec![],
                collapsed: false,
                is_binary: false,
            },
            FileDiff {
                path: "src/pages/Button.tsx".to_string(),
                old_path: None,
                old_content: None,
                new_content: None,
                added: 3,
                removed: 1,
                hunks: vec![],
                collapsed: false,
                is_binary: false,
            },
        ];

        let names = get_display_names(&diffs);
        assert_eq!(names.get("src/components/Button.tsx"), Some(&"components/Button.tsx".to_string()));
        assert_eq!(names.get("src/pages/Button.tsx"), Some(&"pages/Button.tsx".to_string()));
    }
}
