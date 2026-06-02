package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

var ErrInvalidInput = errors.New("invalid input")

type Document struct {
	Path     string
	Raw      string
	Rendered string
	Outline  []Heading
	Links    []Link
}

type Heading struct {
	Level    int
	Text     string
	Line     int
	Parent   int
	Children []int
}

type Link struct {
	Text        string
	URL         string
	Line        int
	StartColumn int
	EndColumn   int
}

func Load(path string, width int) (Document, error) {
	if strings.TrimSpace(path) == "" {
		return Document{}, fmt.Errorf("%w: empty file path", ErrInvalidInput)
	}

	info, err := os.Stat(path)
	if err != nil {
		return Document{}, err
	}
	if info.IsDir() {
		return Document{}, fmt.Errorf("%w: %s is a directory", ErrInvalidInput, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}

	rendered, err := Render(string(data), width)
	if err != nil {
		return Document{}, err
	}

	outline, links := ParseStructure(data)
	return Document{
		Path:     filepath.Clean(path),
		Raw:      string(data),
		Rendered: rendered,
		Outline:  outline,
		Links:    links,
	}, nil
}

func (d Document) Render(width int) (Document, error) {
	rendered, err := Render(d.Raw, width)
	if err != nil {
		return Document{}, err
	}
	d.Rendered = rendered
	return d, nil
}

func Render(markdown string, width int) (string, error) {
	if width < 20 {
		width = 20
	}

	prepared, blocks := prepareCustomBlocks(markdown, width)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylesFromJSONBytes([]byte(mdpokeStyleJSON())),
		glamour.WithChromaFormatter("terminal16m"),
		glamour.WithPreservedNewLines(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", err
	}

	rendered, err := renderer.Render(prepared)
	if err != nil {
		return "", err
	}
	rendered = replaceCustomBlocks(rendered, blocks)
	rendered = hardWrapRendered(rendered, width)
	return strings.TrimRight(rendered, "\n") + "\n", nil
}

func hardWrapRendered(rendered string, width int) string {
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, hardWrapANSILine(line, width)...)
	}
	return strings.Join(wrapped, "\n")
}

func hardWrapANSILine(line string, width int) []string {
	if width <= 0 || lipgloss.Width(StripANSI(line)) <= width {
		return []string{line}
	}

	prefix := wrapContinuationPrefix(line, width)
	prefixWidth := lipgloss.Width(prefix)
	nextWidth := max(1, width-prefixWidth)
	totalWidth := lipgloss.Width(StripANSI(line))
	lines := make([]string, 0, (totalWidth/width)+1)

	start := 0
	segmentWidth := width
	for start < totalWidth {
		end := min(totalWidth, start+segmentWidth)
		segment := ansiVisibleColumnSlice(line, start, end)
		if start > 0 {
			segment = prefix + segment
		}
		lines = append(lines, segment)
		start = end
		segmentWidth = nextWidth
	}
	return lines
}

func wrapContinuationPrefix(line string, width int) string {
	indent := renderedLeadingWhitespace(StripANSI(line))
	prefix := indent + "↪ "
	if lipgloss.Width(prefix) < width {
		return prefix
	}
	if width > lipgloss.Width("↪ ") {
		return "↪ "
	}
	return ""
}

func renderedLeadingWhitespace(line string) string {
	var b strings.Builder
	for _, r := range line {
		if r != ' ' && r != '\t' {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func ansiVisibleColumnSlice(line string, start, end int) string {
	startByte := ansiVisibleColumnByteIndex(line, start)
	endByte := ansiVisibleColumnByteIndex(line, end)
	segment := line[startByte:endByte]
	if active := activeSGRAt(line, startByte); active != "" {
		return active + segment + "\x1b[0m"
	}
	return segment
}

func ansiVisibleColumnByteIndex(line string, column int) int {
	if column <= 0 {
		return 0
	}
	visible := 0
	for i := 0; i < len(line); {
		if line[i] == '\x1b' {
			next := i + 1
			if next < len(line) && line[next] == '[' {
				next++
				for next < len(line) {
					if line[next] >= '@' && line[next] <= '~' {
						next++
						break
					}
					next++
				}
				i = next
				continue
			}
			if next < len(line) && line[next] == ']' {
				next++
				for next < len(line) {
					if line[next] == '\a' {
						next++
						break
					}
					if line[next] == '\x1b' && next+1 < len(line) && line[next+1] == '\\' {
						next += 2
						break
					}
					next++
				}
				i = next
				continue
			}
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		if size <= 0 {
			size = 1
		}
		nextVisible := visible + lipgloss.Width(string(r))
		if nextVisible > column {
			return i
		}
		if nextVisible == column {
			return i + size
		}
		visible = nextVisible
		i += size
	}
	return len(line)
}

func activeSGRAt(line string, byteIndex int) string {
	active := ""
	for i := 0; i < len(line) && i < byteIndex; {
		if line[i] != '\x1b' || i+1 >= len(line) || line[i+1] != '[' {
			_, size := utf8.DecodeRuneInString(line[i:])
			if size <= 0 {
				size = 1
			}
			i += size
			continue
		}

		end := i + 2
		for end < len(line) {
			if line[end] >= '@' && line[end] <= '~' {
				end++
				break
			}
			end++
		}
		seq := line[i:end]
		if strings.HasSuffix(seq, "m") {
			if seq == "\x1b[0m" || strings.Contains(seq, "[0;") || strings.Contains(seq, ";0m") {
				active = ""
			} else {
				active += seq
			}
		}
		i = end
	}
	return active
}

func mdpokeStyleJSON() string {
	return `{
  "document": {
    "color": "#cdd6f4",
    "margin": 2
  },
  "paragraph": {
    "color": "#cdd6f4"
  },
  "heading": {
    "color": "#89b4fa",
    "bold": true,
    "block_prefix": "\n",
    "block_suffix": "\n"
  },
  "h1": {
    "prefix": "  ",
    "suffix": "  ",
    "color": "#11111b",
    "background_color": "#89b4fa",
    "bold": true,
    "upper": false
  },
  "h2": {
    "prefix": "## ",
    "color": "#f9e2af",
    "bold": true
  },
  "h3": {
    "prefix": "### ",
    "color": "#94e2d5",
    "bold": true
  },
  "h4": {
    "prefix": "#### ",
    "color": "#cba6f7",
    "bold": true
  },
  "h5": {
    "prefix": "##### ",
    "color": "#a6e3a1",
    "bold": true
  },
  "h6": {
    "prefix": "###### ",
    "color": "#fab387",
    "bold": true
  },
  "link": {
    "color": "#89b4fa",
    "underline": true
  },
  "link_text": {
    "color": "#74c7ec",
    "bold": true
  },
  "code": {
    "color": "#f38ba8",
    "background_color": "#313244"
  },
  "code_block": {
    "theme": "catppuccin-mocha",
    "margin": 1
  },
  "block_quote": {
    "color": "#bac2de",
    "indent": 1,
    "indent_token": "│ "
  },
  "list": {
    "level_indent": 2
  },
  "item": {
    "block_prefix": "• "
  },
  "enumeration": {
    "block_prefix": ". "
  },
  "task": {
    "ticked": "[x] ",
    "unticked": "[ ] "
  },
  "table": {
    "center_separator": "┼",
    "column_separator": "│",
    "row_separator": "─"
  }
}`
}

func ParseStructure(source []byte) ([]Heading, []Link) {
	parser := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	).Parser()
	root := parser.Parse(text.NewReader(source))

	lineStarts := lineStartOffsets(source)
	outline := make([]Heading, 0)
	links := make([]Link, 0)
	headingStack := make([]int, 7)
	for i := range headingStack {
		headingStack[i] = -1
	}

	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.Heading:
			line := nodeLine(n, lineStarts)
			heading := Heading{
				Level:  n.Level,
				Text:   strings.TrimSpace(string(n.Text(source))),
				Line:   line,
				Parent: nearestParent(headingStack, n.Level),
			}
			outline = append(outline, heading)
			index := len(outline) - 1
			if heading.Parent >= 0 {
				outline[heading.Parent].Children = append(outline[heading.Parent].Children, index)
			}
			headingStack[n.Level] = index
			for level := n.Level + 1; level < len(headingStack); level++ {
				headingStack[level] = -1
			}
		case *ast.Link:
			start, stop := nodeSpan(n)
			line, column := offsetPosition(start, lineStarts)
			_, endColumn := offsetPosition(stop, lineStarts)
			links = append(links, Link{
				Text:        strings.TrimSpace(string(n.Text(source))),
				URL:         string(n.Destination),
				Line:        line,
				StartColumn: column,
				EndColumn:   endColumn,
			})
		case *ast.AutoLink:
			start, stop := nodeSpan(n)
			line, column := offsetPosition(start, lineStarts)
			_, endColumn := offsetPosition(stop, lineStarts)
			links = append(links, Link{
				Text:        strings.TrimSpace(string(n.Label(source))),
				URL:         string(n.URL(source)),
				Line:        line,
				StartColumn: column,
				EndColumn:   endColumn,
			})
		}

		return ast.WalkContinue, nil
	})

	return outline, expandBareAutoLinks(source, lineStarts, links)
}

func expandBareAutoLinks(source []byte, lineStarts []int, links []Link) []Link {
	for i, link := range links {
		if link.Text != link.URL || !hasURLScheme(link.URL) || link.Line <= 0 || link.Line > len(lineStarts) {
			continue
		}

		lineStart := lineStarts[link.Line-1]
		lineStop := len(source)
		if link.Line < len(lineStarts) {
			lineStop = lineStarts[link.Line] - 1
		}
		start := lineStart + bareURLLineOffset(source[lineStart:lineStop], link)
		if start < lineStart || start >= lineStop {
			continue
		}

		stop := scanBareURLStop(source, start, lineStop)
		if stop <= start {
			continue
		}
		url := string(source[start:stop])
		links[i].Text = url
		links[i].URL = url
		links[i].EndColumn = stop - lineStart + 1
	}
	return links
}

func bareURLLineOffset(line []byte, link Link) int {
	target := []byte(link.URL)
	best := -1
	bestDistance := 0
	offset := 0
	for offset <= len(line) {
		found := bytes.Index(line[offset:], target)
		if found < 0 {
			break
		}
		index := offset + found
		distance := abs(index + 1 - link.StartColumn)
		if best < 0 || distance < bestDistance {
			best = index
			bestDistance = distance
		}
		step := len(target)
		if step < 1 {
			step = 1
		}
		offset = index + step
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func hasURLScheme(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func scanBareURLStop(source []byte, start, lineStop int) int {
	stop := start
	for stop < lineStop {
		r, size := utf8.DecodeRune(source[stop:lineStop])
		if r == utf8.RuneError && size == 1 {
			break
		}
		if unicode.IsSpace(r) || strings.ContainsRune("<>()[]{}\"'", r) {
			break
		}
		stop += size
	}
	return stop
}

func nearestParent(stack []int, level int) int {
	for l := level - 1; l >= 1; l-- {
		if stack[l] >= 0 {
			return stack[l]
		}
	}
	return -1
}

func nodeLine(node ast.Node, lineStarts []int) int {
	start, _ := nodeSpan(node)
	line, _ := offsetPosition(start, lineStarts)
	return line
}

func nodeSpan(node ast.Node) (int, int) {
	if node.Type() == ast.TypeBlock {
		lines := node.Lines()
		if lines != nil && lines.Len() > 0 {
			segment := lines.At(0)
			return segment.Start, segment.Stop
		}
	}

	if textNode, ok := node.(*ast.Text); ok {
		return textNode.Segment.Start, textNode.Segment.Stop
	}

	start, stop := node.Pos(), node.Pos()
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		childStart, childStop := nodeSpan(child)
		if childStart >= 0 && (start < 0 || childStart < start) {
			start = childStart
		}
		if childStop > stop {
			stop = childStop
		}
	}
	if start < 0 {
		start = 0
	}
	if stop < start {
		stop = start
	}
	return start, stop
}

func lineStartOffsets(source []byte) []int {
	starts := []int{0}
	for i, b := range source {
		if b == '\n' && i+1 < len(source) {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func offsetPosition(offset int, lineStarts []int) (int, int) {
	if offset < 0 {
		return 1, 1
	}

	lineIndex := 0
	for i := len(lineStarts) - 1; i >= 0; i-- {
		if offset >= lineStarts[i] {
			lineIndex = i
			break
		}
	}
	return lineIndex + 1, offset - lineStarts[lineIndex] + 1
}

func StripANSI(s string) string {
	var out bytes.Buffer
	for i := 0; i < len(s); i++ {
		if s[i] != '\x1b' {
			out.WriteByte(s[i])
			continue
		}

		if i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) {
				if s[i] >= '@' && s[i] <= '~' {
					break
				}
				i++
			}
			continue
		}
		if i+1 < len(s) && s[i+1] == ']' {
			i += 2
			for i < len(s) {
				if s[i] == '\a' {
					break
				}
				if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
					i++
					break
				}
				i++
			}
			continue
		}
	}
	return out.String()
}
