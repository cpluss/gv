# gv

A fast terminal UI for reviewing diffs across git worktrees.

Built for **multi-agent vibe coding** when you're running multiple AI agents in parallel and need to review their work.

![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)
![Rust](https://img.shields.io/badge/rust-2024-orange.svg)

## Why gv?

When you're running multiple agents across worktrees, reviewing their output is painful:

- Switching between directories breaks your flow
- `git diff` output is noisy and hard to scan
- You want to see *specific commits*, not the whole branch
- You need it to be **fast** on large repos with massive diffs

**gv** solves this with a keyboard-driven TUI that lets you jump between worktrees, cherry-pick commits to review, and read syntax-highlighted diffs at speed.

## Features

**Worktree Navigation**
- Auto-detects all worktrees in your repo
- Switch instantly with fuzzy search (`w`)
- Always compares feature branch against main

**Selective Commit Review**
- View all commits, specific commits, or just uncommitted changes
- Toggle individual commits on/off (`c`)
- See exactly what each agent changed

**Fast Diff Browsing**
- Side-by-side or unified view (`u`)
- Syntax highlighting for 200+ languages
- Collapsible file tree with change stats
- Adjustable context lines (`x`)
- Hide lock files and dotfiles (`h`)

**Keyboard-Driven**
- Vim-style navigation (`j`/`k`, `g`/`G`, `Ctrl-d`/`Ctrl-u`)
- Jump between files (`n`/`N`)
- Everything accessible without a mouse

## Keybindings

| Key | Action |
|-----|--------|
| `j`/`k` | Scroll up/down |
| `n`/`N` | Next/previous file |
| `g`/`G` | Top/bottom |
| `Ctrl-d`/`Ctrl-u` | Page down/up |
| `u` | Toggle unified/side-by-side |
| `x` | Cycle context lines (3→1→0) |
| `h` | Toggle hidden files |
| `c` | Select commits to show |
| `w` | Switch worktree |
| `Space` | Collapse/expand file |
| `z` | Collapse/expand all |
| `?` | Help |
| `q` | Quit |

## Built with Rust

Performance matters when you're reviewing thousands of lines across multiple worktrees:

- Direct libgit2 bindings via `git2`
- Syntax highlighting with `syntect`
- TUI rendering with `ratatui`
- LTO-optimized release builds

## License

MIT
