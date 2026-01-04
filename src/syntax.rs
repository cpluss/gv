//! Syntax highlighting module
//!
//! Provides syntax highlighting for code using syntect.
//! Supports detection of languages from file paths and caching
//! of highlighted lines for performance.

use std::collections::HashMap;
use std::path::Path;
use syntect::highlighting::{ThemeSet, Style, FontStyle};
use syntect::parsing::SyntaxSet;
use syntect::easy::HighlightLines;
use ratatui::style::{Color, Modifier, Style as RatatuiStyle};

/// A styled token for display
#[derive(Debug, Clone)]
pub struct Token {
    /// The text content
    pub text: String,
    /// The ratatui style to apply
    pub style: RatatuiStyle,
}

/// A line of highlighted tokens
pub type HighlightedLine = Vec<Token>;

/// Syntax highlighter with caching
pub struct Highlighter {
    syntax_set: SyntaxSet,
    theme_set: ThemeSet,
    /// Cache of highlighted lines by filename
    cache: HashMap<String, Vec<HighlightedLine>>,
}

impl Highlighter {
    /// Create a new highlighter
    pub fn new() -> Self {
        Self {
            syntax_set: SyntaxSet::load_defaults_newlines(),
            theme_set: ThemeSet::load_defaults(),
            cache: HashMap::new(),
        }
    }

    /// Highlight a set of lines for a given file
    ///
    /// Returns a vector of highlighted lines, where each line is a vector of tokens.
    pub fn highlight_lines(&mut self, filename: &str, lines: &[&str]) -> Vec<HighlightedLine> {
        // Check cache first
        if let Some(cached) = self.cache.get(filename) {
            if cached.len() == lines.len() {
                return cached.clone();
            }
        }

        let highlighted = self.do_highlight(filename, lines);

        // Cache the result
        self.cache.insert(filename.to_string(), highlighted.clone());

        highlighted
    }

    /// Perform the actual highlighting
    fn do_highlight(&self, filename: &str, lines: &[&str]) -> Vec<HighlightedLine> {
        let syntax = self.detect_syntax(filename);
        let theme = &self.theme_set.themes["base16-ocean.dark"];

        let mut highlighter = HighlightLines::new(syntax, theme);
        let mut result = Vec::with_capacity(lines.len());

        for line in lines {
            match highlighter.highlight_line(line, &self.syntax_set) {
                Ok(ranges) => {
                    let tokens: Vec<Token> = ranges
                        .into_iter()
                        .map(|(style, text)| Token {
                            text: text.to_string(),
                            style: syntect_style_to_ratatui(style),
                        })
                        .collect();
                    result.push(tokens);
                }
                Err(_) => {
                    // Fall back to plain text
                    result.push(vec![Token {
                        text: line.to_string(),
                        style: RatatuiStyle::default(),
                    }]);
                }
            }
        }

        result
    }

    /// Detect the syntax for a file based on its path
    fn detect_syntax(&self, filename: &str) -> &syntect::parsing::SyntaxReference {
        let path = Path::new(filename);

        // Try by extension first
        if let Some(ext) = path.extension().and_then(|e| e.to_str()) {
            if let Some(syntax) = self.syntax_set.find_syntax_by_extension(ext) {
                return syntax;
            }
        }

        // Try by filename
        if let Some(name) = path.file_name().and_then(|n| n.to_str()) {
            if let Some(syntax) = self.syntax_set.find_syntax_by_extension(name) {
                return syntax;
            }
        }

        // Default to plain text
        self.syntax_set.find_syntax_plain_text()
    }

    /// Clear the cache
    pub fn clear_cache(&mut self) {
        self.cache.clear();
    }

    /// Get a cached highlighted line, or highlight it on demand
    pub fn get_line(&mut self, filename: &str, line_index: usize, line_content: &str) -> HighlightedLine {
        // Check if we have this file cached
        if let Some(cached) = self.cache.get(filename) {
            if let Some(line) = cached.get(line_index) {
                return line.clone();
            }
        }

        // Highlight just this one line
        let lines = vec![line_content];
        let highlighted = self.do_highlight(filename, &lines);
        highlighted.into_iter().next().unwrap_or_default()
    }
}

impl Default for Highlighter {
    fn default() -> Self {
        Self::new()
    }
}

/// Convert a syntect Style to a ratatui Style
fn syntect_style_to_ratatui(style: Style) -> RatatuiStyle {
    let fg = Color::Rgb(
        style.foreground.r,
        style.foreground.g,
        style.foreground.b,
    );

    let mut ratatui_style = RatatuiStyle::default().fg(fg);

    if style.font_style.contains(FontStyle::BOLD) {
        ratatui_style = ratatui_style.add_modifier(Modifier::BOLD);
    }
    if style.font_style.contains(FontStyle::ITALIC) {
        ratatui_style = ratatui_style.add_modifier(Modifier::ITALIC);
    }
    if style.font_style.contains(FontStyle::UNDERLINE) {
        ratatui_style = ratatui_style.add_modifier(Modifier::UNDERLINED);
    }

    ratatui_style
}

/// Detect language from a filename (for display purposes)
#[allow(dead_code)]
pub fn detect_language(filename: &str) -> &'static str {
    let path = Path::new(filename);
    let ext = path.extension()
        .and_then(|e| e.to_str())
        .unwrap_or("");

    match ext {
        "rs" => "Rust",
        "go" => "Go",
        "js" => "JavaScript",
        "ts" => "TypeScript",
        "tsx" => "TypeScript React",
        "jsx" => "JavaScript React",
        "py" => "Python",
        "rb" => "Ruby",
        "java" => "Java",
        "c" => "C",
        "cpp" | "cc" | "cxx" => "C++",
        "h" | "hpp" => "C/C++ Header",
        "md" => "Markdown",
        "json" => "JSON",
        "yaml" | "yml" => "YAML",
        "toml" => "TOML",
        "html" => "HTML",
        "css" => "CSS",
        "scss" | "sass" => "Sass",
        "sql" => "SQL",
        "sh" | "bash" => "Shell",
        "dockerfile" => "Dockerfile",
        _ => "Plain Text",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_detect_language() {
        assert_eq!(detect_language("main.rs"), "Rust");
        assert_eq!(detect_language("app.go"), "Go");
        assert_eq!(detect_language("index.ts"), "TypeScript");
        assert_eq!(detect_language("unknown.xyz"), "Plain Text");
    }

    #[test]
    fn test_highlighter_creation() {
        let highlighter = Highlighter::new();
        assert!(highlighter.cache.is_empty());
    }
}
