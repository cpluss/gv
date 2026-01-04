package syntax

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Highlighter provides syntax highlighting for diff content
type Highlighter struct {
	style     *chroma.Style
	formatter chroma.Formatter
}

// NewHighlighter creates a new syntax highlighter
func NewHighlighter() *Highlighter {
	style := styles.Get("github-dark")
	if style == nil {
		style = styles.Fallback
	}
	return &Highlighter{
		style:     style,
		formatter: formatters.TTY16m,
	}
}

// HighlightedLine represents a line with syntax highlighting tokens
type HighlightedLine struct {
	Text string
}

// HighlightLines highlights multiple lines of code for a given filename
func (h *Highlighter) HighlightLines(filename string, lines []string) []HighlightedLine {
	result := make([]HighlightedLine, len(lines))
	if len(lines) == 0 {
		return result
	}

	normalizedLines := make([]string, len(lines))
	for i, line := range lines {
		normalizedLines[i] = strings.ReplaceAll(line, "\t", "    ")
	}

	// Get lexer for file type
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Join lines for tokenization
	content := strings.Join(normalizedLines, "\n")

	// Tokenize
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		// Fall back to plain text
		for i, line := range normalizedLines {
			result[i] = HighlightedLine{
				Text: line,
			}
		}
		return result
	}

	var buf bytes.Buffer
	if err := h.formatter.Format(&buf, h.style, iterator); err != nil {
		for i, line := range normalizedLines {
			result[i] = HighlightedLine{
				Text: line,
			}
		}
		return result
	}

	highlightedLines := strings.Split(buf.String(), "\n")
	if len(highlightedLines) > len(lines) {
		highlightedLines = highlightedLines[:len(lines)]
	}
	for i, line := range normalizedLines {
		if i < len(highlightedLines) {
			result[i] = HighlightedLine{
				Text: highlightedLines[i],
			}
			continue
		}
		result[i] = HighlightedLine{
			Text: line,
		}
	}

	return result
}

// DetectLanguage returns the detected language for a filename
func DetectLanguage(filename string) string {
	lexer := lexers.Match(filename)
	if lexer == nil {
		return ""
	}
	config := lexer.Config()
	if config == nil {
		return ""
	}
	return config.Name
}

// GetFileExtension returns the extension of a filename
func GetFileExtension(filename string) string {
	return strings.TrimPrefix(filepath.Ext(filename), ".")
}
