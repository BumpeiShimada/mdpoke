package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/glamour"
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
	return strings.TrimRight(rendered, "\n") + "\n", nil
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

	return outline, links
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
