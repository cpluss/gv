//! UI module
//!
//! Contains all terminal UI components:
//! - Styles for consistent theming
//! - Diff view rendering
//! - File sidebar
//! - Header and footer
//! - Popups and overlays

mod styles;
pub mod diff_view;
pub mod sidebar;
mod header;
pub mod footer;
mod popup;
mod file_tree;

pub use styles::Styles;
pub use diff_view::{render_diff_content, DiffMode};
pub use sidebar::render_sidebar;
pub use header::render_header;
pub use footer::{render_footer, FocusArea};
pub use popup::{render_commit_popup, render_worktree_popup, render_help_popup};
pub use file_tree::{TreeNode, build_file_tree, flatten_tree};
