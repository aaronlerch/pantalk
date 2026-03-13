package formatting

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// markdownEscapeRE matches a backslash followed by a markdown special character.
// These escapes are produced by AI models (e.g. Claude) and are meaningful in
// markdown renderers, but show up as literal backslashes in Slack messages.
var markdownEscapeRE = regexp.MustCompile(`\\([!*_#\[\]()~` + "`" + `>+\-={}.:|\\])`)

// blockCloseTagRE matches the closing tag of any HTML block-level element.
var blockCloseTagRE = regexp.MustCompile(`(?i)</(p|pre|ul|ol|h[1-6]|blockquote|table|div|li|tr)>`)

// tagRE matches any HTML open or close tag.  Group 1 = "/" for close tags,
// group 2 = element name, group 3 = attributes + optional self-close slash.
var tagRE = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)([^>]*)>`)

// htmlTagStripRE matches any HTML tag for stripping purposes.
var htmlTagStripRE = regexp.MustCompile(`<[^>]*>`)

// voidElements are HTML elements that must not have a closing tag.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true,
	"embed": true, "hr": true, "img": true, "input": true,
	"link": true, "meta": true, "source": true, "track": true, "wbr": true,
}

// htmlEntities maps the most common HTML character references to their
// plain-text equivalents.  Used by StripHTML.
var htmlEntities = map[string]string{
	"&amp;":  "&",
	"&lt;":   "<",
	"&gt;":   ">",
	"&quot;": `"`,
	"&#39;":  "'",
	"&apos;": "'",
	"&nbsp;": " ",
}

const (
	FormatPlain    = "plain"
	FormatMarkdown = "markdown"
	FormatHTML     = "html"
)

func NormalizeFormat(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return FormatPlain, nil
	}

	switch normalized {
	case FormatPlain:
		return FormatPlain, nil
	case FormatMarkdown, "md":
		return FormatMarkdown, nil
	case FormatHTML:
		return FormatHTML, nil
	default:
		return "", fmt.Errorf("unsupported format %q (supported: plain, markdown, html)", value)
	}
}

func MarkdownToHTML(markdown string) (string, error) {
	converter := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	var output bytes.Buffer
	if err := converter.Convert([]byte(markdown), &output); err != nil {
		return "", err
	}

	return strings.TrimSpace(output.String()), nil
}

// StripHTML removes all HTML tags and decodes common character entities,
// returning plain text suitable for platforms that have no markup support.
func StripHTML(htmlStr string) string {
	text := htmlTagStripRE.ReplaceAllString(htmlStr, "")
	for entity, replacement := range htmlEntities {
		text = strings.ReplaceAll(text, entity, replacement)
	}
	// Collapse runs of whitespace that remain after tag removal.
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

// MarkdownToPlain converts a Markdown string to plain text by first rendering
// to HTML and then stripping all tags.
func MarkdownToPlain(markdown string) string {
	htmlStr, err := MarkdownToHTML(markdown)
	if err != nil {
		return markdown
	}
	return StripHTML(htmlStr)
}

// StripMarkdownEscapes removes backslash escapes before markdown special
// characters. AI models commonly produce these (e.g. "\!" or "\*") which
// render correctly in markdown but appear as literal backslashes in Slack.
func StripMarkdownEscapes(text string) string {
	return markdownEscapeRE.ReplaceAllString(text, "$1")
}

func SplitText(text string, maxLen int) []string {
	if maxLen <= 0 || utf8.RuneCountInString(text) <= maxLen {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n\n")
	chunks := make([]string, 0, len(paragraphs))
	current := ""

	for _, paragraph := range paragraphs {
		trimmedParagraph := strings.TrimSpace(paragraph)
		if trimmedParagraph == "" {
			continue
		}

		if utf8.RuneCountInString(trimmedParagraph) > maxLen {
			if current != "" {
				chunks = append(chunks, current)
				current = ""
			}

			chunks = append(chunks, hardSplit(trimmedParagraph, maxLen)...)
			continue
		}

		candidate := trimmedParagraph
		if current != "" {
			candidate = current + "\n\n" + trimmedParagraph
		}

		if utf8.RuneCountInString(candidate) <= maxLen {
			current = candidate
			continue
		}

		chunks = append(chunks, current)
		current = trimmedParagraph
	}

	if current != "" {
		chunks = append(chunks, current)
	}

	if len(chunks) == 0 {
		return []string{text}
	}

	return chunks
}

// SplitHTML splits an HTML string into chunks of at most maxLen runes,
// preferring to break after block-level closing tags so that tags are never
// torn apart mid-element.  Each resulting chunk is repaired to be well-formed
// HTML: unclosed tags are closed at the end of the chunk and reopened at the
// start of the next one.
func SplitHTML(htmlStr string, maxLen int) []string {
	if maxLen <= 0 || utf8.RuneCountInString(htmlStr) <= maxLen {
		return []string{htmlStr}
	}

	matches := blockCloseTagRE.FindAllStringIndex(htmlStr, -1)
	if len(matches) == 0 {
		return repairHTMLChunks(hardSplit(htmlStr, maxLen))
	}

	// Break htmlStr into logical blocks that each end at a block closing tag.
	var blocks []string
	prev := 0
	for _, m := range matches {
		block := strings.TrimSpace(htmlStr[prev:m[1]])
		if block != "" {
			blocks = append(blocks, block)
		}
		prev = m[1]
	}
	if remainder := strings.TrimSpace(htmlStr[prev:]); remainder != "" {
		blocks = append(blocks, remainder)
	}

	chunks := make([]string, 0, len(blocks))
	current := ""

	for _, block := range blocks {
		if utf8.RuneCountInString(block) > maxLen {
			if current != "" {
				chunks = append(chunks, current)
				current = ""
			}
			chunks = append(chunks, hardSplit(block, maxLen)...)
			continue
		}

		candidate := block
		if current != "" {
			candidate = current + "\n" + block
		}

		if utf8.RuneCountInString(candidate) <= maxLen {
			current = candidate
		} else {
			chunks = append(chunks, current)
			current = block
		}
	}

	if current != "" {
		chunks = append(chunks, current)
	}

	if len(chunks) == 0 {
		return []string{htmlStr}
	}

	return repairHTMLChunks(chunks)
}

// tagInfo captures an opening HTML tag and its element name so that it can
// be closed and reopened across chunk boundaries.
type tagInfo struct {
	name    string // lowercase element name, e.g. "blockquote"
	openTag string // full opening tag, e.g. `<a href="...">`
}

// repairHTMLChunks ensures each chunk is well-formed HTML.  Unclosed opening
// tags at the end of a chunk are closed, and the corresponding opening tags
// are prepended to the next chunk.
func repairHTMLChunks(chunks []string) []string {
	if len(chunks) <= 1 {
		return chunks
	}

	result := make([]string, len(chunks))
	var carry []tagInfo

	for i, chunk := range chunks {
		// Prepend carried-over opening tags from the previous chunk.
		var prefix strings.Builder
		for _, t := range carry {
			prefix.WriteString(t.openTag)
		}
		working := prefix.String() + chunk

		// Walk all tags and track which are still open.
		var stack []tagInfo
		for _, m := range tagRE.FindAllStringSubmatch(working, -1) {
			isClose := m[1] == "/"
			name := strings.ToLower(m[2])
			attrs := m[3]

			selfClose := strings.HasSuffix(strings.TrimSpace(attrs), "/")
			if voidElements[name] || selfClose {
				continue
			}

			if isClose {
				// Pop the innermost matching open tag.
				for j := len(stack) - 1; j >= 0; j-- {
					if stack[j].name == name {
						stack = append(stack[:j], stack[j+1:]...)
						break
					}
				}
			} else {
				stack = append(stack, tagInfo{name: name, openTag: m[0]})
			}
		}

		// Close every tag that is still open (innermost first).
		var suffix strings.Builder
		for j := len(stack) - 1; j >= 0; j-- {
			suffix.WriteString("</")
			suffix.WriteString(stack[j].name)
			suffix.WriteString(">")
		}

		result[i] = working + suffix.String()
		carry = stack
	}

	return result
}

func hardSplit(text string, maxLen int) []string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return []string{text}
	}

	chunks := make([]string, 0, len(runes)/maxLen+1)
	for start := 0; start < len(runes); {
		end := start + maxLen
		if end >= len(runes) {
			end = len(runes)
		} else {
			safe := findSafeBreak(runes, start, end)
			// If findSafeBreak returns start, we'd make zero progress.
			// Force the split at the original position to avoid an infinite loop.
			if safe > start {
				end = safe
			}
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		start = end
	}

	return chunks
}

// findSafeBreak walks backwards from the proposed split position to avoid
// cutting inside an HTML tag (<...>) or character entity (&...;).
func findSafeBreak(runes []rune, start, end int) int {
	searchStart := end - 100
	if searchStart < start {
		searchStart = start
	}

	// Avoid splitting inside an HTML tag.
	for i := end - 1; i >= searchStart; i-- {
		if runes[i] == '>' {
			break // found a tag close – we are outside a tag
		}
		if runes[i] == '<' {
			return i // inside a tag – split before '<'
		}
	}

	// Avoid splitting inside a character entity.
	for i := end - 1; i >= searchStart; i-- {
		r := runes[i]
		if r == ';' || r == ' ' || r == '<' || r == '>' || r == '\n' {
			break
		}
		if r == '&' {
			return i // inside an entity – split before '&'
		}
	}

	return end
}
