package syntax

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Highlighter provides syntax highlighting for diff content
type Highlighter struct {
	style *chroma.Style
}

// NewHighlighter creates a new syntax highlighter
func NewHighlighter() *Highlighter {
	return &Highlighter{
		style: styles.Get("monokai"),
	}
}

// HighlightedLine represents a line with syntax highlighting tokens
type HighlightedLine struct {
	Tokens []Token
}

// Token represents a syntax-highlighted token
type Token struct {
	Text  string
	Style TokenStyle
}

// TokenStyle contains styling information for a token
type TokenStyle struct {
	Color     string
	Bold      bool
	Italic    bool
	Underline bool
}

// HighlightLines highlights multiple lines of code for a given filename
func (h *Highlighter) HighlightLines(filename string, lines []string) []HighlightedLine {
	result := make([]HighlightedLine, len(lines))

	// Get lexer for file type
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Join lines for tokenization
	content := strings.Join(lines, "\n")

	// Tokenize
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		// Fall back to plain text
		for i, line := range lines {
			result[i] = HighlightedLine{
				Tokens: []Token{{Text: line}},
			}
		}
		return result
	}

	// Split tokens back to lines
	lineIdx := 0
	for _, token := range iterator.Tokens() {
		style := h.tokenStyle(token.Type)

		// Handle multi-line tokens
		parts := strings.Split(token.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lineIdx++
				if lineIdx >= len(result) {
					break
				}
			}
			if lineIdx < len(result) && part != "" {
				result[lineIdx].Tokens = append(result[lineIdx].Tokens, Token{
					Text:  part,
					Style: style,
				})
			}
		}
	}

	return result
}

// tokenStyle converts a chroma token type to our TokenStyle
func (h *Highlighter) tokenStyle(t chroma.TokenType) TokenStyle {
	entry := h.style.Get(t)
	style := TokenStyle{}

	if entry.Colour.IsSet() {
		style.Color = entry.Colour.String()
	}
	style.Bold = entry.Bold == chroma.Yes
	style.Italic = entry.Italic == chroma.Yes
	style.Underline = entry.Underline == chroma.Yes

	return style
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
