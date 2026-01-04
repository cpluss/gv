//! gv - Terminal UI diff viewer for git worktrees
//!
//! A read-only terminal application for reviewing code changes across
//! multiple git worktrees. Built with ratatui for efficient rendering.
//!
//! # Usage
//!
//! ```bash
//! gv                    # Run in current directory
//! gv /path/to/repo      # Run in specified repository
//! gv -b origin/develop  # Use custom base branch
//! ```

mod app;
mod git;
mod syntax;
mod ui;

use std::path::PathBuf;
use anyhow::Result;
use clap::Parser;

/// Terminal UI diff viewer for git worktrees
#[derive(Parser, Debug)]
#[command(name = "gv")]
#[command(author, version, about, long_about = None)]
struct Args {
    /// Path to the repository (defaults to current directory)
    #[arg(default_value = ".")]
    path: PathBuf,

    /// Base branch to diff against (defaults to origin/main or origin/master)
    #[arg(short, long)]
    base: Option<String>,
}

fn main() -> Result<()> {
    let args = Args::parse();

    // Resolve the repository path
    let repo_path = args.path.canonicalize()
        .unwrap_or_else(|_| args.path.clone());

    // Create and run the application
    let mut app = app::App::new(repo_path, args.base)?;
    app.run()?;

    Ok(())
}
