package markdown

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSanitizeMarkdownInputDropsTerminalControls(t *testing.T) {
	input := "ok\n\t日本語" +
		"\x1b[2J" +
		"\a" +
		string([]byte{0x00, 0x1f, 0x7f, 0x80, 0x9f}) +
		string(rune(0x85)) +
		"done"

	got := SanitizeMarkdownInput(input)

	if got != "ok\n\t日本語[2Jdone" {
		t.Fatalf("SanitizeMarkdownInput() = %q", got)
	}
	for _, bad := range []rune{'\x1b', '\a', '\x00', '\x1f', '\x7f', rune(0x85)} {
		if strings.ContainsRune(got, bad) {
			t.Fatalf("sanitized output still contains control %U in %q", bad, got)
		}
	}
}

func TestSanitizeMarkdownInputPreservesNormalMarkdown(t *testing.T) {
	source := strings.Join([]string{
		"# Title",
		"",
		"- [ ] task",
		"Tabbed\ttext",
		"[site](https://example.com/日本語)",
		"",
		"```go",
		"func main() {}",
		"```",
	}, "\n")

	if got := SanitizeMarkdownInput(source); got != source {
		t.Fatalf("normal markdown changed:\n got: %q\nwant: %q", got, source)
	}
}

func TestRenderSanitizesMarkdownBeforeRendering(t *testing.T) {
	rendered, err := Render("# Ti\x1btle\a\n\nbody\x1b]8;;https://example.com\a\n", 80)
	if err != nil {
		t.Fatal(err)
	}

	plain := StripANSI(rendered)
	if strings.ContainsAny(plain, "\x1b\a") {
		t.Fatalf("rendered plain output contains source terminal controls: %q", plain)
	}
	if !strings.Contains(plain, "Title") || !strings.Contains(plain, "body") {
		t.Fatalf("expected normal text to survive sanitization:\n%s", plain)
	}
}

func TestLoadRejectsOversizedFile(t *testing.T) {
	path := writeMarkdownFixture(t, strings.Repeat("a", 33))

	_, err := LoadWithOptions(path, LoadOptions{Width: 80, MaxSize: 32})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error = %q, want too large", err)
	}
}

func TestLoadAcceptsFileAtMaxSize(t *testing.T) {
	source := "# Title\n\nbody\n"
	path := writeMarkdownFixture(t, source)

	doc, err := LoadWithOptions(path, LoadOptions{Width: 80, MaxSize: int64(len(source))})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Raw != source {
		t.Fatalf("doc.Raw = %q, want %q", doc.Raw, source)
	}
}

func TestLoadRejectsSymlinkByDefault(t *testing.T) {
	target := writeMarkdownFixture(t, "# Target\n")
	link := target + ".link"
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := LoadWithOptions(link, LoadOptions{Width: 80})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %q, want symlink", err)
	}
}

func TestLoadAllowsSymlinkWhenExplicit(t *testing.T) {
	target := writeMarkdownFixture(t, "# Target\n")
	link := target + ".link"
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	doc, err := LoadWithOptions(link, LoadOptions{Width: 80, FollowSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}

	if doc.Raw != "# Target\n" {
		t.Fatalf("doc.Raw = %q", doc.Raw)
	}
}

func TestLoadSanitizesRawOutlineAndLinks(t *testing.T) {
	path := writeMarkdownFixture(t, "# Ti\x1btle\n\n[li\x07nk](https://example.com/\x1bpath)\n")

	doc, err := LoadWithOptions(path, LoadOptions{Width: 80})
	if err != nil {
		t.Fatal(err)
	}

	if strings.ContainsAny(doc.Raw, "\x1b\a") {
		t.Fatalf("doc.Raw contains terminal controls: %q", doc.Raw)
	}
	if len(doc.Outline) != 1 || doc.Outline[0].Text != "Title" {
		t.Fatalf("outline = %#v, want sanitized Title", doc.Outline)
	}
	if len(doc.Links) != 1 || doc.Links[0].Text != "link" || doc.Links[0].URL != "https://example.com/path" {
		t.Fatalf("links = %#v, want sanitized link", doc.Links)
	}
}

func TestParseStructureExtractsOutlineTree(t *testing.T) {
	source := []byte(strings.Join([]string{
		"# Title",
		"",
		"## First",
		"### Child",
		"## Second",
	}, "\n"))

	outline, _ := ParseStructure(source)

	if len(outline) != 4 {
		t.Fatalf("expected 4 headings, got %d", len(outline))
	}

	tests := []struct {
		name     string
		index    int
		level    int
		text     string
		line     int
		parent   int
		children []int
	}{
		{name: "root", index: 0, level: 1, text: "Title", line: 1, parent: -1, children: []int{1, 3}},
		{name: "first", index: 1, level: 2, text: "First", line: 3, parent: 0, children: []int{2}},
		{name: "child", index: 2, level: 3, text: "Child", line: 4, parent: 1},
		{name: "second", index: 3, level: 2, text: "Second", line: 5, parent: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := outline[tt.index]
			if got.Level != tt.level || got.Text != tt.text || got.Line != tt.line || got.Parent != tt.parent {
				t.Fatalf("heading mismatch: got %#v", got)
			}
			if len(got.Children) != len(tt.children) {
				t.Fatalf("children mismatch: got %v, want %v", got.Children, tt.children)
			}
			for i := range tt.children {
				if got.Children[i] != tt.children[i] {
					t.Fatalf("children mismatch: got %v, want %v", got.Children, tt.children)
				}
			}
		})
	}
}

func TestParseStructureExtractsLinks(t *testing.T) {
	source := []byte("Read [the docs](docs/plan.md) and [site](https://example.com), plus <https://example.com/autolink-fixture>.\n")

	_, links := ParseStructure(source)

	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d", len(links))
	}
	if links[0].Text != "the docs" || links[0].URL != "docs/plan.md" || links[0].Line != 1 {
		t.Fatalf("unexpected first link: %#v", links[0])
	}
	if links[1].Text != "site" || links[1].URL != "https://example.com" || links[1].Line != 1 {
		t.Fatalf("unexpected second link: %#v", links[1])
	}
	if links[2].Text != "https://example.com/autolink-fixture" || links[2].URL != "https://example.com/autolink-fixture" || links[2].Line != 1 {
		t.Fatalf("unexpected autolink: %#v", links[2])
	}
	if links[0].StartColumn >= links[0].EndColumn {
		t.Fatalf("expected link columns to increase: %#v", links[0])
	}
}

func TestParseStructureExpandsBareAutolinkWithNonASCIIPath(t *testing.T) {
	source := []byte("- こちらがURL: https://example.com/path/to/日本語リソース名\n")

	_, links := ParseStructure(source)

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %#v", len(links), links)
	}
	want := "https://example.com/path/to/日本語リソース名"
	if links[0].Text != want || links[0].URL != want {
		t.Fatalf("link = %#v, want URL %q", links[0], want)
	}
}

func TestParseStructureDoesNotExpandNamedMarkdownLinkDestination(t *testing.T) {
	source := []byte("[site](https://example.com/path/to/日本語リソース名)\n")

	_, links := ParseStructure(source)

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %#v", len(links), links)
	}
	if links[0].Text != "site" || links[0].URL != "https://example.com/path/to/日本語リソース名" {
		t.Fatalf("unexpected link: %#v", links[0])
	}
}

func TestStripANSIRemovesOSCHyperlinks(t *testing.T) {
	input := "\x1b]8;;https://example.com\aExample\x1b]8;;\a"
	if got := StripANSI(input); got != "Example" {
		t.Fatalf("StripANSI() = %q, want Example", got)
	}
}

func TestRenderKeepsNestedBulletsIndented(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"- parent",
		"  - child",
		"    - grandchild",
	}, "\n"), 80)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(StripANSI(rendered), "\n")
	var child, grandchild string
	for _, line := range lines {
		if strings.Contains(line, "child") && !strings.Contains(line, "grandchild") {
			child = line
		}
		if strings.Contains(line, "grandchild") {
			grandchild = line
		}
	}

	if child == "" || grandchild == "" {
		t.Fatalf("expected nested bullet lines in rendered output:\n%s", StripANSI(rendered))
	}
	if leadingSpaces(grandchild) <= leadingSpaces(child) {
		t.Fatalf("expected grandchild to be more indented than child:\n%s", StripANSI(rendered))
	}
}

func TestRenderBulletMarkersWhenItemStartsWithInlineCode(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"- `aaa`",
		"    - `xxx`",
		"- `111`",
		"    - `000`",
	}, "\n"), 80)
	if err != nil {
		t.Fatal(err)
	}

	plain := StripANSI(rendered)
	for _, want := range []string{"• aaa", "• xxx", "• 111", "• 000"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected bullet marker before inline code item %q:\n%s", want, plain)
		}
	}
	for _, bad := range []string{"  aaa", "    xxx", "  111", "    000"} {
		if strings.Contains(plain, bad) {
			t.Fatalf("bullet marker disappeared before inline code item as %q:\n%s", bad, plain)
		}
	}
}

func TestRenderOrderedListKeepsSpaceAfterMarker(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"1. First item",
		"2. `code` item",
		"3. `.env.example` item",
		"10. Tenth item",
	}, "\n"), 80)
	if err != nil {
		t.Fatal(err)
	}

	plain := StripANSI(rendered)
	for _, want := range []string{"1. First item", "2. code item", "3. .env.example item", "4. Tenth item"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected ordered list marker spacing %q:\n%s", want, plain)
		}
	}
	for _, bad := range []string{"1First item", "2code item", "3.env.example item", "4Tenth item"} {
		if strings.Contains(plain, bad) {
			t.Fatalf("ordered list marker spacing collapsed as %q:\n%s", bad, plain)
		}
	}
}

func TestRenderStylesHeadingsAndCodeBlocks(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"# H1",
		"## H2",
		"### H3",
		"#### H4",
		"##### H5",
		"###### H6",
		"",
		"```go",
		"func main() {}",
		"```",
	}, "\n"), 80)
	if err != nil {
		t.Fatal(err)
	}

	plain := StripANSI(rendered)
	for _, want := range []string{"H1", "## H2", "### H3", "#### H4", "##### H5", "###### H6", "╭─ go", "╰"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected rendered output to contain %q:\n%s", want, plain)
		}
	}
}

func TestRenderInlineCodeDoesNotAddPaddingSpaces(t *testing.T) {
	rendered, err := Render("before `code` after\n", 80)
	if err != nil {
		t.Fatal(err)
	}

	plain := StripANSI(rendered)
	if !strings.Contains(plain, "before code after") {
		t.Fatalf("expected inline code without added padding spaces:\n%s", plain)
	}
	if strings.Contains(plain, "before  code  after") {
		t.Fatalf("inline code should not add spaces around code text:\n%s", plain)
	}
}

func TestRenderPreservesSourceLineBreaksWithinParagraph(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"first source line",
		"second source line without trailing spaces",
		"third source line",
	}, "\n"), 80)
	if err != nil {
		t.Fatal(err)
	}

	plain := StripANSI(rendered)
	lines := strings.Split(plain, "\n")
	firstLine := -1
	secondLine := -1
	thirdLine := -1
	for i, line := range lines {
		switch {
		case strings.Contains(line, "first source line"):
			firstLine = i
		case strings.Contains(line, "second source line without trailing spaces"):
			secondLine = i
		case strings.Contains(line, "third source line"):
			thirdLine = i
		}
	}

	if firstLine < 0 || secondLine < 0 || thirdLine < 0 {
		t.Fatalf("expected all source lines in rendered output:\n%s", plain)
	}
	if firstLine == secondLine || secondLine == thirdLine {
		t.Fatalf("expected source line breaks to remain visible:\n%s", plain)
	}
	if strings.Contains(plain, "first source line second source line") {
		t.Fatalf("source line break collapsed into a space:\n%s", plain)
	}
}

func TestRenderCodeBlockBorderFitsLongestLine(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"```go",
		"short",
		"longerLine := true",
		"```",
	}, "\n"), 80)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(StripANSI(rendered), "\n")
	topWidth := 0
	bottomWidth := 0
	for _, line := range lines {
		if strings.Contains(line, "╭─ go") {
			topWidth = displayWidth(line)
		}
		if strings.Contains(line, "╰") {
			bottomWidth = displayWidth(line)
		}
	}

	if topWidth == 0 || bottomWidth == 0 {
		t.Fatalf("expected code block borders:\n%s", StripANSI(rendered))
	}
	wantWidth := len(customBlockIndent) + len("longerLine := true") + 2
	if topWidth != wantWidth || bottomWidth != wantWidth {
		t.Fatalf("expected borders to fit the longest code line, got top=%d bottom=%d want=%d:\n%s", topWidth, bottomWidth, wantWidth, StripANSI(rendered))
	}
}

func TestRenderIndentedCodeBlockKeepsFenceIndentOutsideContent(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"1. mac で git を使えるようにする",
		"",
		"    ```sh",
		"    xcode-select --install",
		"    ```",
	}, "\n"), 80)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(StripANSI(rendered), "\n")
	var borderLine, codeLine string
	for _, line := range lines {
		if strings.Contains(line, "╭─ sh") {
			borderLine = line
		}
		if strings.Contains(line, "xcode-select --install") {
			codeLine = line
		}
	}

	if borderLine == "" || codeLine == "" {
		t.Fatalf("expected indented code block to render:\n%s", StripANSI(rendered))
	}
	wantIndent := len("    ") + len(customBlockIndent)
	if leadingSpaces(borderLine) != wantIndent || leadingSpaces(codeLine) != wantIndent {
		t.Fatalf("expected border and code to keep fence indent outside content, got border=%d code=%d want=%d:\n%s", leadingSpaces(borderLine), leadingSpaces(codeLine), wantIndent, StripANSI(rendered))
	}
	if strings.HasPrefix(strings.TrimPrefix(codeLine, strings.Repeat(" ", wantIndent)), "    ") {
		t.Fatalf("expected code content to strip the fence indent:\n%s", StripANSI(rendered))
	}
}

func TestRenderCodeBlockWrapsLongLines(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"```text",
		"abcdefghijklmnopqrstuvwxyz0123456789",
		"```",
	}, "\n"), 28)
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range strings.Split(StripANSI(rendered), "\n") {
		if displayWidth(line) > 28 {
			t.Fatalf("expected rendered code line to fit width, got %d for %q:\n%s", displayWidth(line), line, StripANSI(rendered))
		}
	}
}

func TestRenderTableUsesContentSizedColumns(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"| A | Longer |",
		"| --- | --- |",
		"| x | yy |",
	}, "\n"), 80)
	if err != nil {
		t.Fatal(err)
	}

	plain := StripANSI(rendered)
	maxLineWidth := 0
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "│") || strings.Contains(line, "┌") || strings.Contains(line, "└") {
			maxLineWidth = max(maxLineWidth, displayWidth(line))
		}
	}
	if maxLineWidth == 0 {
		t.Fatalf("expected rendered table:\n%s", plain)
	}
	if maxLineWidth >= 32 {
		t.Fatalf("expected table to use content-sized columns, got max width %d:\n%s", maxLineWidth, plain)
	}
}

func TestRenderTableWrapsWhenColumnIsTooWide(t *testing.T) {
	rendered, err := Render(strings.Join([]string{
		"| Name | Description |",
		"| --- | --- |",
		"| alpha | abcdefghijklmnopqrstuvwxyz0123456789 |",
	}, "\n"), 36)
	if err != nil {
		t.Fatal(err)
	}

	plain := StripANSI(rendered)
	descriptionLines := 0
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "abcdef") || strings.Contains(line, "uvwxyz") || strings.Contains(line, "012345") {
			descriptionLines++
		}
		if displayWidth(line) > 36 {
			t.Fatalf("expected wrapped table line to fit width, got %d for %q:\n%s", displayWidth(line), line, plain)
		}
	}
	if descriptionLines < 2 {
		t.Fatalf("expected long table cell to wrap across lines:\n%s", plain)
	}
}

func TestRenderHardWrapsLongTextWithoutSpaces(t *testing.T) {
	source := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz\n"

	rendered, err := Render(source, 24)
	if err != nil {
		t.Fatal(err)
	}
	plain := StripANSI(rendered)
	lines := strings.Split(strings.TrimSpace(plain), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected long text to wrap, got:\n%s", plain)
	}
	for _, line := range lines {
		if displayWidth(line) > 24 {
			t.Fatalf("line width = %d, want <= 24 for %q:\n%s", displayWidth(line), line, plain)
		}
	}
	if !strings.Contains(plain, "↪") {
		t.Fatalf("expected continuation marker in wrapped text:\n%s", plain)
	}
	if !strings.Contains(lines[1], "↪ ") {
		t.Fatalf("second wrapped line = %q, want continuation marker:\n%s", lines[1], plain)
	}
}

func TestHardWrapContinuationMarkerFollowsIndent(t *testing.T) {
	lines := hardWrapANSILine("    abcdefghijklmnopqrstuvwxyz", 14)
	if len(lines) < 2 {
		t.Fatalf("expected hard wrap, got %#v", lines)
	}
	if !strings.HasPrefix(lines[1], "    ↪ ") {
		t.Fatalf("continuation line = %q, want marker after indent", lines[1])
	}
}

func leadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r != ' ' {
			return count
		}
		count++
	}
	return count
}

func displayWidth(s string) int {
	return lipgloss.Width(s)
}

func writeMarkdownFixture(t *testing.T, source string) string {
	t.Helper()
	path := t.TempDir() + "/fixture.md"
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
