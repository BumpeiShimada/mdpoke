package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	md "github.com/BumpeiShimada/mdpoke/internal/markdown"
)

func TestFindMatches(t *testing.T) {
	lines := []string{
		"Alpha beta alpha",
		"nothing",
		"ALPHA",
	}

	matches := FindMatches(lines, "alpha")
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}

	want := []SearchMatch{
		{Line: 0, Start: 0, End: 5},
		{Line: 0, Start: 11, End: 16},
		{Line: 2, Start: 0, End: 5},
	}
	for i := range want {
		if matches[i] != want[i] {
			t.Fatalf("match %d: got %#v, want %#v", i, matches[i], want[i])
		}
	}
}

func TestRenderContentHighlightsEverySearchMatch(t *testing.T) {
	model := New(md.Document{
		Rendered: "alpha beta alpha\n",
		Raw:      "alpha beta alpha\n",
	})
	model.searchMatches = FindMatches(model.contentLines, "alpha")
	model.selectedMatch = 1
	oldSearchMatchStyle := searchMatchStyle
	oldActiveSearchMatchStyle := activeSearchMatchStyle
	searchMatchStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	activeSearchMatchStyle = lipgloss.NewStyle().Transform(func(s string) string { return "{" + s + "}" })
	defer func() {
		searchMatchStyle = oldSearchMatchStyle
		activeSearchMatchStyle = oldActiveSearchMatchStyle
	}()

	rendered := model.renderContent()
	if !strings.Contains(rendered, "[alpha]") {
		t.Fatalf("expected non-focused match to be highlighted, got %q", rendered)
	}
	if !strings.Contains(rendered, "{alpha}") {
		t.Fatalf("expected focused match to be highlighted differently, got %q", rendered)
	}
}

func TestRenderContentHighlightsFocusedLinkURLWhenVisible(t *testing.T) {
	model := New(md.Document{
		Rendered: "Heading with Docs docs/plan.md link\n",
		Raw:      "# Heading with [Docs](docs/plan.md) link\n",
		Links: []md.Link{
			{Text: "Docs", URL: "docs/plan.md", Line: 1},
		},
	})
	model.selectedLink = 0
	oldLinkFocusStyle := linkFocusStyle
	linkFocusStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	defer func() {
		linkFocusStyle = oldLinkFocusStyle
	}()

	rendered := model.renderContent()
	if !strings.Contains(rendered, "[docs/plan.md]") {
		t.Fatalf("expected focused link URL to be highlighted, got %q", rendered)
	}
	if strings.Contains(rendered, "[Docs]") || strings.Contains(rendered, "[Heading") || strings.Contains(rendered, "link]") {
		t.Fatalf("expected only visible URL to be highlighted, got %q", rendered)
	}
}

func TestRenderContentFallsBackToFocusedLinkText(t *testing.T) {
	model := New(md.Document{
		Rendered: "Heading with Docs link\n",
		Raw:      "# Heading with [Docs](docs/plan.md) link\n",
		Links: []md.Link{
			{Text: "Docs", URL: "docs/plan.md", Line: 1},
		},
	})
	model.selectedLink = 0
	oldLinkFocusStyle := linkFocusStyle
	linkFocusStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	defer func() {
		linkFocusStyle = oldLinkFocusStyle
	}()

	rendered := model.renderContent()
	if !strings.Contains(rendered, "[Docs]") {
		t.Fatalf("expected focused link text to be highlighted when URL is hidden, got %q", rendered)
	}
}

func TestRenderContentStylesNamedExternalLinksLikeBareURLs(t *testing.T) {
	raw := "[External](https://example.com)\n[Jump](#jump-target)\n"
	rendered, err := md.Render(raw, 80)
	if err != nil {
		t.Fatal(err)
	}
	_, links := md.ParseStructure([]byte(raw))

	oldBareAutoLinkStyle := bareAutoLinkStyle
	bareAutoLinkStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	defer func() {
		bareAutoLinkStyle = oldBareAutoLinkStyle
	}()

	model := New(md.Document{
		Rendered: rendered,
		Raw:      raw,
		Links:    links,
	})

	content := model.renderContent()
	if !strings.Contains(content, "[External]") {
		t.Fatalf("expected named external link text to use bare URL style:\n%s", content)
	}
	if strings.Contains(content, "[Jump]") {
		t.Fatalf("internal anchor link should keep a distinct style:\n%s", content)
	}
	if strings.Contains(md.StripANSI(content), "https://example.com") {
		t.Fatalf("named external link URL should not be visible:\n%s", content)
	}
}

func TestRenderContentExtendsBareAutolinkHighlightToCopiedURL(t *testing.T) {
	url := "https://example.com/path/to/日本語リソース名"
	oldBareAutoLinkStyle := bareAutoLinkStyle
	bareAutoLinkStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	defer func() {
		bareAutoLinkStyle = oldBareAutoLinkStyle
	}()
	model := New(md.Document{
		Rendered: "こちらがURL: \x1b[4mhttps://example.com/path/to/\x1b[0m日本語リソース名\n",
		Raw:      "こちらがURL: " + url + "\n",
		Links: []md.Link{
			{Text: url, URL: url, Line: 1},
		},
	})

	rendered := model.renderContent()
	if !strings.Contains(rendered, "["+url+"]") {
		t.Fatalf("expected full copied URL to be highlighted, got %q", rendered)
	}
	if strings.Contains(rendered, "[https://example.com/path/to/]日本語リソース名") {
		t.Fatalf("highlight stopped before non-ASCII URL suffix: %q", rendered)
	}
}

func TestRenderContentHighlightsWrappedBareAutolinkSegments(t *testing.T) {
	url := "https://example.com/path/to/日本語リソース名"
	oldBareAutoLinkStyle := bareAutoLinkStyle
	bareAutoLinkStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	defer func() {
		bareAutoLinkStyle = oldBareAutoLinkStyle
	}()
	model := New(md.Document{
		Rendered: "こちらがURL: https://example.com/path/to/\n↪ 日本語リソース名\n",
		Raw:      "こちらがURL: " + url + "\n",
		Links: []md.Link{
			{Text: url, URL: url, Line: 1},
		},
	})

	rendered := model.renderContent()
	if !strings.Contains(rendered, "[https://example.com/path/to/]") {
		t.Fatalf("expected first wrapped URL segment to be highlighted, got %q", rendered)
	}
	if !strings.Contains(rendered, "[日本語リソース名]") {
		t.Fatalf("expected continuation URL segment to be highlighted, got %q", rendered)
	}
}

func TestFixtureFirstExternalLinkFocusIsVisible(t *testing.T) {
	model, _ := fixtureModel(t)
	index := linkIndexByURL(t, model.doc.Links, "https://example.com")
	model.selectedLink = index
	oldLinkFocusStyle := linkFocusStyle
	linkFocusStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	defer func() {
		linkFocusStyle = oldLinkFocusStyle
	}()

	rendered := model.renderContent()
	if !strings.Contains(rendered, "[External Example]") && !strings.Contains(rendered, "[https://example.com]") {
		t.Fatalf("expected first external link focus to be visible:\n%s", rendered)
	}
}

func TestModalOverlayKeepsBorderColumnsAlignedWithStyledContent(t *testing.T) {
	model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
	model.width = 80
	model.height = 12
	model.ready = true
	model.modalKind = modalMessage
	model.modalTitle = "Copy Failed"
	model.modalBody = "exit status 1\n\n" + mutedStyle.Render("press any key to close")

	rendered := model.modalOverlay(strings.Repeat(strings.Repeat(" ", model.width)+"\n", model.height))
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	left, _, boxWidth, _, ok := model.modalBoxBounds()
	if !ok {
		t.Fatal("expected modal bounds")
	}
	wantRight := left + boxWidth - 1

	for _, line := range lines {
		plain := md.StripANSI(line)
		if !strings.ContainsAny(plain, "╭│╰") {
			continue
		}
		right := max(strings.LastIndex(plain, "╮"), strings.LastIndex(plain, "│"))
		right = max(right, strings.LastIndex(plain, "╯"))
		rightColumn := lipgloss.Width(plain[:right])
		if rightColumn != wantRight {
			t.Fatalf("right border column = %d, want %d in %q", rightColumn, wantRight, plain)
		}
	}
}

func TestModalOverlayWrapsLongURLWithinBorders(t *testing.T) {
	model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
	model.width = 52
	model.height = 12
	model.ready = true
	model.modalKind = modalConfirmJump
	model.modalTitle = "Jump?"
	model.modalBody = "Jump to https://example.com/path/to/veryveryverylongresource-name-without-spaces?\n\n" + mutedStyle.Render("y confirm   n cancel")

	rendered := model.modalOverlay(strings.Repeat(strings.Repeat(" ", model.width)+"\n", model.height))
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	left, _, boxWidth, _, ok := model.modalBoxBounds()
	if !ok {
		t.Fatal("expected modal bounds")
	}
	wantRight := left + boxWidth - 1
	bodyRows := 0
	for _, line := range lines {
		plain := md.StripANSI(line)
		if !strings.Contains(plain, "│") {
			continue
		}
		bodyRows++
		right := strings.LastIndex(plain, "│")
		rightColumn := lipgloss.Width(plain[:right])
		if rightColumn != wantRight {
			t.Fatalf("right border column = %d, want %d in %q", rightColumn, wantRight, plain)
		}
	}
	if bodyRows < 4 {
		t.Fatalf("expected long modal body to wrap across rows, got %d:\n%s", bodyRows, rendered)
	}
}

func TestModalOverlayKeepsRectangleOnStyledNarrowBase(t *testing.T) {
	model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
	model.width = 44
	model.height = 12
	model.ready = true
	model.modalKind = modalConfirmJump
	model.modalTitle = "Jump To A Very Long Wrapped URL?"
	model.modalBody = "Jump to https://example.com/path/to/very-long-resource-name-without-spaces-and-日本語?\n\n" + mutedStyle.Render("y confirm   n cancel")

	baseLine := lipgloss.NewStyle().Background(lipgloss.Color("238")).Render(strings.Repeat("x", model.width))
	base := strings.Repeat(baseLine+"\n", model.height)
	rendered := model.modalOverlay(base)
	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	left, _, boxWidth, _, ok := model.modalBoxBounds()
	if !ok {
		t.Fatal("expected modal bounds")
	}
	wantRight := left + boxWidth - 1
	boxRows := 0

	for _, line := range lines {
		plain := md.StripANSI(line)
		if !strings.ContainsAny(plain, "╭│╰") {
			continue
		}
		boxRows++
		leftBorder := strings.IndexAny(plain, "╭│╰")
		rightBorder := max(strings.LastIndex(plain, "╮"), strings.LastIndex(plain, "│"))
		rightBorder = max(rightBorder, strings.LastIndex(plain, "╯"))
		if leftBorder < 0 || rightBorder < 0 {
			t.Fatalf("missing modal border in %q", plain)
		}
		if lipgloss.Width(plain[:leftBorder]) != left {
			t.Fatalf("left border column = %d, want %d in %q", lipgloss.Width(plain[:leftBorder]), left, plain)
		}
		if lipgloss.Width(plain[:rightBorder]) != wantRight {
			t.Fatalf("right border column = %d, want %d in %q", lipgloss.Width(plain[:rightBorder]), wantRight, plain)
		}
	}
	if boxRows != model.modalHeight() {
		t.Fatalf("box rows = %d, want modal height %d:\n%s", boxRows, model.modalHeight(), rendered)
	}
}

func TestGuideLabelsTabAsCheckboxFocus(t *testing.T) {
	model := New(md.Document{
		Rendered: "[ ] Pending\n",
		Raw:      "- [ ] Pending\n",
	})

	items := strings.Join(model.guideItems(), " ")
	if !strings.Contains(items, "tab checkbox") {
		t.Fatalf("guide items = %q, want tab checkbox", items)
	}

	model.selectedTask = 0
	items = strings.Join(model.guideItems(), " ")
	if !strings.Contains(items, "tab next box") {
		t.Fatalf("focused guide items = %q, want tab next box", items)
	}
	if !strings.Contains(items, "space toggle") {
		t.Fatalf("focused guide items = %q, want space toggle", items)
	}
}

func TestTabWithoutCheckboxesShowsMessage(t *testing.T) {
	model := New(md.Document{
		Rendered: "No tasks here\n",
		Raw:      "No tasks here\n",
	})

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	got := next.(Model)

	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want message", got.modalKind)
	}
	if !strings.Contains(got.modalTitle, "No Checkboxes") {
		t.Fatalf("modalTitle = %q, want No Checkboxes", got.modalTitle)
	}
}

func TestReadmeKeyTableMatchesSearchableHelp(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readmeKeys := readmeKeyTableKeys(t, string(data))
	helpKeys := make(map[string]bool)
	for _, item := range helpItems() {
		for _, key := range splitReadmeKeyCell(item.Key) {
			helpKeys[normalizeReadmeKey(key)] = true
		}
	}

	for _, cell := range readmeKeys {
		for _, key := range splitReadmeKeyCell(cell) {
			normalized := normalizeReadmeKey(key)
			if !helpKeys[normalized] {
				t.Fatalf("README key %q from cell %q is missing from searchable help; help keys=%v", normalized, cell, helpKeys)
			}
		}
	}
}

func TestTabFocusesCheckboxInsteadOfLink(t *testing.T) {
	source := "Read [docs](https://example.com/docs) first\n- [ ] Pending\n"
	model, _ := taskFixtureModel(t, source)

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	got := next.(Model)

	if got.selectedTask != 0 {
		t.Fatalf("selectedTask = %d, want first checkbox", got.selectedTask)
	}
	if got.selectedLink != -1 {
		t.Fatalf("selectedLink = %d, want no focused link from tab", got.selectedLink)
	}
}

func TestTabAndSpaceToggleCheckboxAndUpdateFile(t *testing.T) {
	model, path := taskFixtureModel(t, "- [ ] Pending\n- [x] Done\n")
	oldTaskFocusStyle := taskFocusStyle
	taskFocusStyle = lipgloss.NewStyle().Transform(func(s string) string { return "{" + s + "}" })
	defer func() {
		taskFocusStyle = oldTaskFocusStyle
	}()

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	got := next.(Model)
	if got.selectedTask != 0 {
		t.Fatalf("selectedTask = %d, want first checkbox", got.selectedTask)
	}
	if !strings.Contains(got.renderContent(), "{[ ]}") {
		t.Fatalf("expected focused checkbox to be highlighted:\n%s", got.renderContent())
	}

	next, _ = got.Update(tea.KeyMsg(tea.Key{Type: tea.KeySpace}))
	got = next.(Model)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] Pending") {
		t.Fatalf("file was not toggled:\n%s", string(data))
	}
	if got.tasks[0].Checked != true {
		t.Fatalf("first task Checked = false, want true")
	}
}

func TestEnterTogglesFocusedCheckbox(t *testing.T) {
	model, path := taskFixtureModel(t, "- [ ] Pending\n")

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	next, _ = next.(Model).Update(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	got := next.(Model)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] Pending") {
		t.Fatalf("file was not toggled by enter:\n%s", string(data))
	}
	if got.tasks[0].Checked != true {
		t.Fatalf("first task Checked = false, want true")
	}
}

func TestToggleFocusedCheckboxScrollsToIt(t *testing.T) {
	raw := strings.Repeat("plain line\n", 24) + "- [ ] Pending\n"
	model := New(md.Document{
		Rendered: raw,
		Raw:      raw,
	})
	model.body.Width = 60
	model.body.Height = 10
	model.rebuildContent()
	model.selectedTask = 0
	oldTaskFocusStyle := taskFocusStyle
	taskFocusStyle = lipgloss.NewStyle().Transform(func(s string) string { return "{" + s + "}" })
	defer func() {
		taskFocusStyle = oldTaskFocusStyle
	}()

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeySpace}))
	got := next.(Model)
	taskLine := got.lineForTask(got.tasks[0])

	if got.body.YOffset != taskLine-got.body.Height/2 {
		t.Fatalf("YOffset = %d, want focused task centered at %d", got.body.YOffset, taskLine-got.body.Height/2)
	}
	if !strings.Contains(got.body.View(), "{[x]}") {
		t.Fatalf("expected toggled checkbox to be highlighted:\n%s", got.body.View())
	}
}

func TestClickCheckboxTogglesIt(t *testing.T) {
	model, path := taskFixtureModel(t, "- [ ] Pending\n")
	model.body.Width = 60
	model.body.Height = 10
	line := lineContaining(model.contentLines, "[ ]")
	if line < 0 {
		t.Fatalf("checkbox not rendered: %#v", model.contentLines)
	}
	x := strings.Index(model.contentLines[line], "[ ]")
	if x < 0 {
		t.Fatalf("checkbox not rendered: %q", model.contentLines[line])
	}
	model.body.SetYOffset(line)

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] Pending") {
		t.Fatalf("file was not toggled by click:\n%s", string(data))
	}
	if got.selectedTask != 0 {
		t.Fatalf("selectedTask = %d, want clicked checkbox", got.selectedTask)
	}
}

func TestReleaseOnlyCheckboxClickTogglesIt(t *testing.T) {
	model, path := taskFixtureModel(t, "- [ ] Pending\n")
	model.body.Width = 60
	model.body.Height = 10
	line := lineContaining(model.contentLines, "[ ]")
	if line < 0 {
		t.Fatalf("checkbox not rendered: %#v", model.contentLines)
	}
	x := strings.Index(model.contentLines[line], "[ ]")
	if x < 0 {
		t.Fatalf("checkbox not rendered: %q", model.contentLines[line])
	}
	model.body.SetYOffset(line)

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] Pending") {
		t.Fatalf("file was not toggled by release-only click:\n%s", string(data))
	}
	if got.selectedTask != 0 {
		t.Fatalf("selectedTask = %d, want clicked checkbox", got.selectedTask)
	}
}

func TestCheckboxPressReleaseTogglesOnlyOnce(t *testing.T) {
	model, path := taskFixtureModel(t, "- [ ] Pending\n")
	model.body.Width = 60
	model.body.Height = 10
	line := lineContaining(model.contentLines, "[ ]")
	if line < 0 {
		t.Fatalf("checkbox not rendered: %#v", model.contentLines)
	}
	x := strings.Index(model.contentLines[line], "[ ]")
	if x < 0 {
		t.Fatalf("checkbox not rendered: %q", model.contentLines[line])
	}
	model.body.SetYOffset(line)

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "- [x] Pending") {
		t.Fatalf("checkbox should stay checked after press+release:\n%s", text)
	}
	if strings.Contains(text, "- [ ] Pending") {
		t.Fatalf("checkbox toggled twice after press+release:\n%s", text)
	}
	if got.selectedTask != 0 {
		t.Fatalf("selectedTask = %d, want clicked checkbox", got.selectedTask)
	}
}

func TestClickCheckboxDoesNotScrollToIt(t *testing.T) {
	model, path := taskFixtureModel(t, strings.Repeat("plain line\n", 24)+"- [ ] Pending\n")
	model.body.Width = 60
	model.body.Height = 10
	model.rebuildContent()

	line := lineContaining(model.contentLines, "[ ]")
	if line < 0 {
		t.Fatalf("checkbox not rendered: %#v", model.contentLines)
	}
	x := strings.Index(model.contentLines[line], "[ ]")
	if x < 0 {
		t.Fatalf("checkbox not rendered: %q", model.contentLines[line])
	}
	model.body.SetYOffset(line - model.body.Height + 2)
	beforeOffset := model.body.YOffset

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      line - beforeOffset,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] Pending") {
		t.Fatalf("file was not toggled by click:\n%s", string(data))
	}
	if got.body.YOffset != beforeOffset {
		t.Fatalf("YOffset = %d, want unchanged %d", got.body.YOffset, beforeOffset)
	}
	if got.selectedTask != 0 {
		t.Fatalf("selectedTask = %d, want clicked checkbox", got.selectedTask)
	}
}

func TestClickCheckboxOnLinkedTaskDoesNotCopyURL(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	source := "- [ ] Read [docs](https://example.com/docs) before continuing\n"
	model, path := taskFixtureModel(t, source)
	checkboxLine := model.lineForRaw(1)
	start, _, ok := taskCheckboxColumns(model.contentLines[checkboxLine])
	if !ok {
		t.Fatalf("checkbox columns not found in %q", model.contentLines[checkboxLine])
	}

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      start,
		Y:      checkboxLine,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if copied != "" {
		t.Fatalf("copied = %q, want no URL copy from checkbox click", copied)
	}
	if got.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want no copied modal", got.modalKind)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] Read [docs](https://example.com/docs)") {
		t.Fatalf("checkbox click did not toggle linked task:\n%s", string(data))
	}
}

func TestClickWrappedCheckboxRefreshesVisibleContent(t *testing.T) {
	source := strings.Join([]string{
		"- [x] 長いチェック項目その一として画面幅を越えるだけの説明文を置く",
		"- [ ] 長いチェック項目その二としてクリック対象になる説明文を置く",
		"- [ ] 長いチェック項目その三として後続行の対応を見る説明文を置く",
	}, "\n") + "\n"
	path := filepath.Join(t.TempDir(), "tasks.md")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	rendered, err := md.Render(source, 72)
	if err != nil {
		t.Fatal(err)
	}
	model := New(md.Document{Path: path, Raw: source, Rendered: rendered})
	model.body.Width = 72
	model.body.Height = 10
	model.rebuildContent()

	line := lineContaining(model.contentLines, "その二")
	if line < 0 {
		t.Fatalf("checkbox not rendered: %#v", model.contentLines)
	}
	start, _, ok := taskCheckboxColumns(model.contentLines[line])
	if !ok {
		t.Fatalf("checkbox columns not found in %q", model.contentLines[line])
	}
	model.body.SetYOffset(line)

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      start,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [x] 長いチェック項目その二") {
		t.Fatalf("file was not toggled by click:\n%s", string(data))
	}
	if strings.Contains(md.StripANSI(got.body.View()), "[ ] 長いチェック項目その二") {
		t.Fatalf("visible checkbox did not refresh after click:\n%s", md.StripANSI(got.body.View()))
	}
	if !strings.Contains(md.StripANSI(got.body.View()), "[x] 長いチェック項目その二") {
		t.Fatalf("visible checkbox is missing toggled state:\n%s", md.StripANSI(got.body.View()))
	}
}

func TestClickWrappedCheckboxesToggleOnlyClickedTask(t *testing.T) {
	source := strings.Join([]string{
		"- [ ] 共通接頭辞のチェック項目その一として長い説明文を置く",
		"- [ ] 共通接頭辞のチェック項目その二として長い説明文を置く",
		"- [ ] 共通接頭辞のチェック項目その三として長い説明文を置く",
	}, "\n") + "\n"

	tests := []struct {
		name     string
		needle   string
		wantLine string
	}{
		{name: "first", needle: "その一", wantLine: "- [x] 共通接頭辞のチェック項目その一"},
		{name: "second", needle: "その二", wantLine: "- [x] 共通接頭辞のチェック項目その二"},
		{name: "third", needle: "その三", wantLine: "- [x] 共通接頭辞のチェック項目その三"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tasks.md")
			if err := os.WriteFile(path, []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
			rendered, err := md.Render(source, 72)
			if err != nil {
				t.Fatal(err)
			}
			model := New(md.Document{Path: path, Raw: source, Rendered: rendered})
			model.body.Width = 72
			model.body.Height = 10
			model.rebuildContent()

			line := lineContaining(model.contentLines, tt.needle)
			if line < 0 {
				t.Fatalf("checkbox not rendered: %#v", model.contentLines)
			}
			start, _, ok := taskCheckboxColumns(model.contentLines[line])
			if !ok {
				t.Fatalf("checkbox columns not found in %q", model.contentLines[line])
			}
			model.body.SetYOffset(line)

			next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
				X:      start,
				Y:      0,
				Action: tea.MouseActionPress,
				Button: tea.MouseButtonLeft,
			}))
			got := next.(Model)

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if !strings.Contains(text, tt.wantLine) {
				t.Fatalf("clicked task was not toggled:\n%s", text)
			}
			if strings.Count(text, "- [x]") != 1 {
				t.Fatalf("unexpected task toggled; got %d checked tasks:\n%s", strings.Count(text, "- [x]"), text)
			}
			if !strings.Contains(md.StripANSI(got.body.View()), strings.TrimPrefix(tt.wantLine, "- ")) {
				t.Fatalf("visible content did not show clicked task checked:\n%s", md.StripANSI(got.body.View()))
			}
		})
	}
}

func TestClickSplitCheckboxLineTogglesMatchingRawTask(t *testing.T) {
	source := strings.Join([]string{
		"- [ ] 共通接頭辞のチェック項目その一として長い長い説明文を置く",
		"- [ ] 共通接頭辞のチェック項目その二として長い長い説明文を置く",
		"- [ ] 共通接頭辞のチェック項目その三として長い長い説明文を置く",
	}, "\n") + "\n"
	path := filepath.Join(t.TempDir(), "tasks.md")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	model := markdownModelAtWidth(t, source, 26)
	model.doc.Path = path

	targetRaw := 2
	checkboxLine := model.lineForRaw(targetRaw)
	if checkboxLine < 0 || checkboxLine >= len(model.contentLines) {
		t.Fatalf("lineForRaw(%d) = %d outside content", targetRaw, checkboxLine)
	}
	if !strings.Contains(model.contentLines[checkboxLine], "[ ]") {
		t.Fatalf("target raw line does not start on checkbox line: %q", model.contentLines[checkboxLine])
	}
	if strings.Contains(model.contentLines[checkboxLine], "その二") {
		t.Fatalf("test fixture did not split checkbox and text: %q", model.contentLines[checkboxLine])
	}
	foundTextContinuation := false
	for line := checkboxLine + 1; line < len(model.contentLines) && model.rawLineForRendered(line) == targetRaw; line++ {
		if strings.Contains(model.contentLines[line], "その二") {
			foundTextContinuation = true
			break
		}
	}
	if !foundTextContinuation {
		t.Fatalf("target raw line text continuation not found near line %d with raw map %#v:\n%s", checkboxLine, model.rawToRendered, strings.Join(model.contentLines, "\n"))
	}
	start, _, ok := taskCheckboxColumns(model.contentLines[checkboxLine])
	if !ok {
		t.Fatalf("checkbox columns not found in %q", model.contentLines[checkboxLine])
	}
	model.body.SetYOffset(checkboxLine)

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      start,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "- [x] 共通接頭辞のチェック項目その二") {
		t.Fatalf("split checkbox click toggled the wrong task:\n%s", text)
	}
	if strings.Count(text, "- [x]") != 1 {
		t.Fatalf("unexpected checked task count after split click:\n%s", text)
	}
	if !strings.Contains(md.StripANSI(got.body.View()), "[x]") {
		t.Fatalf("visible split checkbox did not refresh:\n%s", md.StripANSI(got.body.View()))
	}
}

func TestFileWatchReloadsExternalChanges(t *testing.T) {
	model, path := taskFixtureModel(t, "# First\n")
	model.body.Width = 80
	model.body.Height = 10
	model.stamp = currentFileStamp(path)

	nextSource := "# First\n\nExternal update marker\n"
	if err := os.WriteFile(path, []byte(nextSource), 0644); err != nil {
		t.Fatal(err)
	}

	next, _ := model.Update(fileWatchMsg{})
	got := next.(Model)

	if !strings.Contains(got.doc.Raw, "External update marker") {
		t.Fatalf("doc was not reloaded:\n%s", got.doc.Raw)
	}
	if !strings.Contains(md.StripANSI(got.renderContent()), "External update marker") {
		t.Fatalf("rendered content was not refreshed:\n%s", md.StripANSI(got.renderContent()))
	}
	if got.status != "reloaded" {
		t.Fatalf("status = %q, want reloaded", got.status)
	}
}

func TestNoWatchInitDoesNotScheduleReload(t *testing.T) {
	model := NewWithOptions(md.Document{Rendered: "body\n", Raw: "body\n"}, Options{NoWatch: true})

	if cmd := model.Init(); cmd != nil {
		t.Fatal("Init returned a watch command with NoWatch enabled")
	}
}

func TestNoWatchIgnoresFileWatchMessages(t *testing.T) {
	model, path := taskFixtureModel(t, "# First\n")
	model = NewWithOptions(model.doc, Options{NoWatch: true})
	model.stamp = fileStamp{}

	if err := os.WriteFile(path, []byte("# First\n\nExternal update marker\n"), 0644); err != nil {
		t.Fatal(err)
	}

	next, cmd := model.Update(fileWatchMsg{})
	got := next.(Model)

	if cmd != nil {
		t.Fatal("fileWatchMsg returned a watch command with NoWatch enabled")
	}
	if strings.Contains(got.doc.Raw, "External update marker") {
		t.Fatalf("doc reloaded despite NoWatch:\n%s", got.doc.Raw)
	}
}

func TestFileWatchRejectsSymlinkReplacementByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte("# Original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := md.LoadWithOptions(path, md.LoadOptions{Width: 80})
	if err != nil {
		t.Fatal(err)
	}
	model := NewWithOptions(doc, Options{})
	model.body.Width = 80
	model.body.Height = 10
	model.stamp = currentFileStamp(path)

	target := filepath.Join(dir, "replacement.md")
	if err := os.WriteFile(target, []byte("# Replacement\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	next, _ := model.Update(fileWatchMsg{})
	got := next.(Model)

	if got.err == nil {
		t.Fatal("expected reload error after symlink replacement")
	}
	if !strings.Contains(got.status, "symlink") {
		t.Fatalf("status = %q, want symlink rejection", got.status)
	}
	if strings.Contains(got.doc.Raw, "Replacement") {
		t.Fatalf("doc reloaded symlink target despite default rejection:\n%s", got.doc.Raw)
	}
}

func TestFileWatchReloadHonorsMaxSizeOption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte("# Small\n"), 0644); err != nil {
		t.Fatal(err)
	}
	loadOptions := md.LoadOptions{Width: 80, MaxSize: 64}
	doc, err := md.LoadWithOptions(path, loadOptions)
	if err != nil {
		t.Fatal(err)
	}
	model := NewWithOptions(doc, Options{LoadOptions: loadOptions})
	model.body.Width = 80
	model.body.Height = 10
	model.stamp = fileStamp{}

	if err := os.WriteFile(path, []byte("# Large\n"+strings.Repeat("x", 128)), 0644); err != nil {
		t.Fatal(err)
	}

	next, _ := model.Update(fileWatchMsg{})
	got := next.(Model)

	if got.err == nil {
		t.Fatal("expected reload error for oversized file")
	}
	if !strings.Contains(got.status, "too large") {
		t.Fatalf("status = %q, want too large", got.status)
	}
	if strings.Contains(got.doc.Raw, "Large") {
		t.Fatalf("doc reloaded oversized content:\n%s", got.doc.Raw)
	}
}

func TestCheckboxToggleRefreshesFileStamp(t *testing.T) {
	model, path := taskFixtureModel(t, "- [ ] Pending\n")
	model.stamp = currentFileStamp(path)

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	next, _ = next.(Model).Update(tea.KeyMsg(tea.Key{Type: tea.KeySpace}))
	got := next.(Model)

	if got.stamp != currentFileStamp(path) {
		t.Fatalf("stamp = %#v, want current %#v", got.stamp, currentFileStamp(path))
	}
}

func TestReadmeQuitKeysReturnQuitCommand(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
	} {
		t.Run(key.String(), func(t *testing.T) {
			model := New(md.Document{Rendered: "body\n", Raw: "body\n"})

			_, cmd := model.Update(key)

			assertQuitCommand(t, cmd)
		})
	}
}

func TestReadmeCtrlCQuitsFromModalSearchAndHelp(t *testing.T) {
	tests := []struct {
		name  string
		model Model
	}{
		{
			name: "modal",
			model: func() Model {
				model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
				model.modalKind = modalMessage
				model.modalTitle = "Message"
				model.modalBody = "body"
				return model
			}(),
		},
		{
			name: "search",
			model: func() Model {
				model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
				model.mode = modeSearch
				return model
			}(),
		},
		{
			name: "help",
			model: func() Model {
				model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
				model.mode = modeHelp
				return model
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cmd := tt.model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

			assertQuitCommand(t, cmd)
		})
	}
}

func TestEscExitsURLFocus(t *testing.T) {
	model := New(md.Document{
		Rendered: "External alpha\nJump Target\n",
		Raw:      "[External](https://example.com) alpha\n## Jump Target\n",
		Links: []md.Link{
			{Text: "External", URL: "https://example.com", Line: 1},
		},
	})
	model.focusNextLink(1)
	model.searchQuery = "alpha"
	model.searchMatches = FindMatches(model.contentLines, "alpha")
	model.selectedMatch = 0
	model.focusedJumpLine = 2
	model.focusedJumpText = "Jump Target"
	model.outlineVisible = true
	model.status = "busy"
	if model.selectedLink != 0 {
		t.Fatalf("selectedLink = %d, want focused URL", model.selectedLink)
	}

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEsc}))
	got := next.(Model)
	if got.selectedLink != -1 {
		t.Fatalf("selectedLink = %d, want URL focus cleared", got.selectedLink)
	}
	if got.status != "" {
		t.Fatalf("status = %q, want cleared", got.status)
	}
	if got.outlineVisible {
		t.Fatal("outlineVisible = true, want closed")
	}
	if got.searchQuery != "" || len(got.searchMatches) != 0 || got.selectedMatch != -1 {
		t.Fatalf("search highlight state not cleared: query=%q matches=%d selected=%d", got.searchQuery, len(got.searchMatches), got.selectedMatch)
	}
	if got.focusedJumpLine != -1 || got.focusedJumpText != "" {
		t.Fatalf("jump focus not cleared: line=%d text=%q", got.focusedJumpLine, got.focusedJumpText)
	}
	items := strings.Join(got.guideItems(), " ")
	if !strings.Contains(items, "o outline") || strings.Contains(items, "n next") || strings.Contains(items, "esc exit") {
		t.Fatalf("guide items = %q, want initial actions", items)
	}
}

func TestReadmeHelpKeyOpensSearchableGuideAndEscCloses(t *testing.T) {
	model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
	next, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	model = next.(Model)

	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	opened := next.(Model)
	if opened.mode != modeHelp {
		t.Fatalf("mode = %v, want help", opened.mode)
	}
	if !strings.Contains(md.StripANSI(opened.View()), "mdpoke keys") {
		t.Fatalf("help view missing title:\n%s", md.StripANSI(opened.View()))
	}

	next, _ = opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	filtered := next.(Model)
	view := md.StripANSI(filtered.View())
	if !strings.Contains(view, "copy") {
		t.Fatalf("filtered help missing copy entry:\n%s", view)
	}
	if strings.Contains(view, "outline") {
		t.Fatalf("filtered help should not show unrelated outline entry:\n%s", view)
	}

	next, _ = filtered.Update(tea.KeyMsg{Type: tea.KeyEsc})
	closed := next.(Model)
	if closed.mode != modeNormal {
		t.Fatalf("mode = %v, want normal after esc", closed.mode)
	}
}

func TestReadmeSearchFlowFindsAndNavigatesMatches(t *testing.T) {
	model := New(md.Document{
		Rendered: "alpha\nTARGET one\nbeta\ntarget two\n",
		Raw:      "alpha\nTARGET one\nbeta\ntarget two\n",
	})
	model.body.Height = 3

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	searching := next.(Model)
	if searching.mode != modeSearch {
		t.Fatalf("mode = %v, want search", searching.mode)
	}
	for _, r := range "target" {
		next, _ = searching.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		searching = next.(Model)
	}
	next, _ = searching.Update(tea.KeyMsg{Type: tea.KeyEnter})
	found := next.(Model)

	if found.mode != modeNormal {
		t.Fatalf("mode = %v, want normal after enter", found.mode)
	}
	if len(found.searchMatches) != 2 {
		t.Fatalf("matches = %d, want 2", len(found.searchMatches))
	}
	if found.selectedMatch != 0 {
		t.Fatalf("selectedMatch = %d, want first match", found.selectedMatch)
	}
	if gotLine := found.searchMatches[found.selectedMatch].Line; gotLine != 1 {
		t.Fatalf("selected match line = %d, want 1", gotLine)
	}

	next, _ = found.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	nextMatch := next.(Model)
	if nextMatch.selectedMatch != 1 {
		t.Fatalf("selectedMatch after n = %d, want second match", nextMatch.selectedMatch)
	}
	if gotLine := nextMatch.searchMatches[nextMatch.selectedMatch].Line; gotLine != 3 {
		t.Fatalf("selected match line after n = %d, want 3", gotLine)
	}

	next, _ = nextMatch.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	previousMatch := next.(Model)
	if previousMatch.selectedMatch != 0 {
		t.Fatalf("selectedMatch after N = %d, want first match", previousMatch.selectedMatch)
	}
}

func TestReadmeEscCancelsSearchWithoutApplyingQuery(t *testing.T) {
	model := New(md.Document{
		Rendered: "alpha\ntarget\n",
		Raw:      "alpha\ntarget\n",
	})

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	searching := next.(Model)
	next, _ = searching.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	searching = next.(Model)
	next, _ = searching.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(Model)

	if got.mode != modeNormal {
		t.Fatalf("mode = %v, want normal after esc", got.mode)
	}
	if got.searchQuery != "" || len(got.searchMatches) != 0 || got.selectedMatch != -1 {
		t.Fatalf("search state was applied despite esc: query=%q matches=%d selected=%d", got.searchQuery, len(got.searchMatches), got.selectedMatch)
	}
}

func TestReadmeNormalNavigationKeysScrollBody(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	text := strings.Join(lines, "\n") + "\n"
	model := New(md.Document{Rendered: text, Raw: text})
	model.body.Height = 5
	model.rebuildContent()

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := next.(Model)
	if got.body.YOffset != 1 {
		t.Fatalf("YOffset after j = %d, want 1", got.body.YOffset)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = next.(Model)
	if got.body.YOffset != 2 {
		t.Fatalf("YOffset after down = %d, want 2", got.body.YOffset)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got = next.(Model)
	if got.body.YOffset != 1 {
		t.Fatalf("YOffset after k = %d, want 1", got.body.YOffset)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyUp})
	got = next.(Model)
	if got.body.YOffset != 0 {
		t.Fatalf("YOffset after up = %d, want 0", got.body.YOffset)
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got = next.(Model)
	if got.body.YOffset == 0 {
		t.Fatal("expected G to jump to bottom")
	}

	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = next.(Model)
	if got.body.YOffset != 0 {
		t.Fatalf("YOffset after g = %d, want 0", got.body.YOffset)
	}
}

func TestReadmePageNavigationKeysScrollBody(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	text := strings.Join(lines, "\n") + "\n"
	model := New(md.Document{Rendered: text, Raw: text})
	model.body.Height = 6
	model.rebuildContent()

	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyPgDown},
		{Type: tea.KeyCtrlF},
	} {
		t.Run(key.String(), func(t *testing.T) {
			next, _ := model.Update(key)
			got := next.(Model)
			if got.body.YOffset <= 0 {
				t.Fatalf("YOffset after %s = %d, want scrolled down", key.String(), got.body.YOffset)
			}
		})
	}

	model.body.GotoBottom()
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyPgUp},
		{Type: tea.KeyCtrlB},
	} {
		t.Run(key.String(), func(t *testing.T) {
			next, _ := model.Update(key)
			got := next.(Model)
			if got.body.YOffset >= model.body.YOffset {
				t.Fatalf("YOffset after %s = %d, want less than %d", key.String(), got.body.YOffset, model.body.YOffset)
			}
		})
	}
}

func TestReadmeMouseWheelScrollsBody(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	text := strings.Join(lines, "\n") + "\n"
	model := New(md.Document{Rendered: text, Raw: text})
	model.body.Height = 5
	model.body.Width = 40
	model.body.MouseWheelDelta = 3
	model.rebuildContent()

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	}))
	got := next.(Model)
	if got.body.YOffset != 3 {
		t.Fatalf("YOffset after wheel down = %d, want 3", got.body.YOffset)
	}

	next, _ = got.Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	}))
	got = next.(Model)
	if got.body.YOffset != 0 {
		t.Fatalf("YOffset after wheel up = %d, want 0", got.body.YOffset)
	}
}

func TestReadmeMouseWheelScrollsOutlinePane(t *testing.T) {
	headings := make([]string, 18)
	for i := range headings {
		headings[i] = fmt.Sprintf("## Heading %02d\nbody", i+1)
	}
	raw := "# Root\n\n" + strings.Join(headings, "\n\n") + "\n"
	rendered, err := md.Render(raw, 78)
	if err != nil {
		t.Fatal(err)
	}
	outline, links := md.ParseStructure([]byte(raw))
	model := New(md.Document{Raw: raw, Rendered: rendered, Outline: outline, Links: links})
	model.width = 100
	model.height = 8
	model.ready = true
	model.resize()
	model.toggleOutline()
	model.outline.MouseWheelDelta = 2
	model.outline.Height = 4

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      model.body.Width + 2,
		Y:      1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	}))
	got := next.(Model)
	if got.outline.YOffset != 2 {
		t.Fatalf("outline YOffset after wheel down = %d, want 2", got.outline.YOffset)
	}
	if got.body.YOffset != model.body.YOffset {
		t.Fatalf("body YOffset changed from %d to %d while scrolling outline", model.body.YOffset, got.body.YOffset)
	}
}

func TestReadmeArrowKeysOpenAndCloseOutline(t *testing.T) {
	raw := "# Root\n\n## Second\nbody\n"
	rendered, err := md.Render(raw, 78)
	if err != nil {
		t.Fatal(err)
	}
	outline, links := md.ParseStructure([]byte(raw))
	model := New(md.Document{Raw: raw, Rendered: rendered, Outline: outline, Links: links})
	model.width = 80
	model.height = 10
	model.ready = true
	model.resize()

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	opened := next.(Model)
	if !opened.outlineVisible {
		t.Fatal("outlineVisible = false after right, want open")
	}

	next, _ = opened.Update(tea.KeyMsg{Type: tea.KeyLeft})
	closed := next.(Model)
	if closed.outlineVisible {
		t.Fatal("outlineVisible = true after left, want closed")
	}
}

func TestReadmeOutlineNavigationKeysSelectAndScrollToHeadings(t *testing.T) {
	raw := strings.Join([]string{
		"# Root",
		"",
		strings.Repeat("alpha ", 20),
		"",
		"## Second",
		"second body",
		"",
		"## Third",
		"third body",
	}, "\n") + "\n"
	rendered, err := md.Render(raw, 78)
	if err != nil {
		t.Fatal(err)
	}
	outline, links := md.ParseStructure([]byte(raw))
	model := New(md.Document{Raw: raw, Rendered: rendered, Outline: outline, Links: links})
	model.width = 80
	model.height = 8
	model.ready = true
	model.resize()
	model.toggleOutline()

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	second := next.(Model)
	if second.selectedOutline != 1 {
		t.Fatalf("selectedOutline after j = %d, want second heading", second.selectedOutline)
	}
	if topRaw := second.rawLineForRendered(second.body.YOffset); topRaw > outline[1].Line {
		t.Fatalf("body top raw line = %d, want at or before selected heading raw %d", topRaw, outline[1].Line)
	}

	next, _ = second.Update(tea.KeyMsg{Type: tea.KeyDown})
	third := next.(Model)
	if third.selectedOutline != 2 {
		t.Fatalf("selectedOutline after down = %d, want third heading", third.selectedOutline)
	}
	if topRaw := third.rawLineForRendered(third.body.YOffset); topRaw > outline[2].Line {
		t.Fatalf("body top raw line = %d, want at or before selected heading raw %d", topRaw, outline[2].Line)
	}

	next, _ = third.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	backToSecond := next.(Model)
	if backToSecond.selectedOutline != 1 {
		t.Fatalf("selectedOutline after k = %d, want second heading", backToSecond.selectedOutline)
	}

	next, _ = backToSecond.Update(tea.KeyMsg{Type: tea.KeyUp})
	backToRoot := next.(Model)
	if backToRoot.selectedOutline != 0 {
		t.Fatalf("selectedOutline after up = %d, want root heading", backToRoot.selectedOutline)
	}
}

func TestReadmeYCopiesFirstLinkOnCurrentLine(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Rendered: "first link and second link\n",
		Raw:      "[first](https://example.com/first) link and [second](https://example.com/second) link\n",
		Links: []md.Link{
			{Text: "first", URL: "https://example.com/first", Line: 1},
			{Text: "second", URL: "https://example.com/second", Line: 1},
		},
	})
	model.body.SetYOffset(0)

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := next.(Model)

	if copied != "https://example.com/first" {
		t.Fatalf("copied = %q, want first current-line URL", copied)
	}
	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want copied message", got.modalKind)
	}
}

func TestStyleANSIVisibleRangeKeepsSurroundingANSI(t *testing.T) {
	line := "\x1b[31malpha beta\x1b[0m"
	style := lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	got := styleANSIVisibleRange(line, 6, 10, style)

	if !strings.Contains(got, "[beta]") {
		t.Fatalf("expected beta to remain visible: %q", got)
	}
	if strings.Count(got, "\x1b[") < 2 {
		t.Fatalf("expected ANSI styling to be present: %q", got)
	}
}

func TestStyleANSIVisibleRangeSkipsOSCHyperlinkSequences(t *testing.T) {
	line := "\x1b]8;;https://example.com\aurl\x1b]8;;\a"
	style := lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })
	got := styleANSIVisibleRange(line, 0, 3, style)

	if !strings.Contains(got, "[url]") {
		t.Fatalf("expected URL text to be highlighted without OSC sequence drift: %q", got)
	}
}

func TestJumpToRenderedLineCentersTarget(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}
	model := New(md.Document{
		Rendered: strings.Join(lines, "\n"),
		Raw:      strings.Join(lines, "\n"),
	})
	model.body.Height = 20

	model.jumpToRenderedLine(50, false)
	if model.body.YOffset != 40 {
		t.Fatalf("YOffset = %d, want 40", model.body.YOffset)
	}
}

func TestViewerCanScrollPastLastLine(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	model := New(md.Document{
		Rendered: strings.Join(lines, "\n"),
		Raw:      strings.Join(lines, "\n"),
	})
	model.body.Height = 10
	model.rebuildContent()

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'G'}}))
	got := next.(Model)
	want := len(model.renderedLines) + bottomScrollPad - model.body.Height
	if got.body.YOffset != want {
		t.Fatalf("YOffset = %d, want %d", got.body.YOffset, want)
	}
	if got.body.YOffset <= len(model.renderedLines)-model.body.Height {
		t.Fatalf("expected viewport to scroll into bottom padding, got offset %d", got.body.YOffset)
	}
}

func TestSpaceDoesNotScrollViewer(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	model := New(md.Document{
		Rendered: strings.Join(lines, "\n"),
		Raw:      strings.Join(lines, "\n"),
	})
	model.body.Height = 10
	model.rebuildContent()

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeySpace, Runes: []rune{' '}}))
	got := next.(Model)
	if got.body.YOffset != 0 {
		t.Fatalf("YOffset = %d, want unchanged 0", got.body.YOffset)
	}
}

func TestViewFillsWindowHeight(t *testing.T) {
	raw := "# Title\n\nbody\n"
	rendered, err := md.Render(raw, 60)
	if err != nil {
		t.Fatal(err)
	}

	model := New(md.Document{
		Path:     "fixture.md",
		Raw:      raw,
		Rendered: rendered,
	})
	next, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	got := next.(Model).View()

	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 12 {
		t.Fatalf("view rendered %d lines, want 12:\n%s", len(lines), got)
	}
}

func TestNormalizeMarkdownLine(t *testing.T) {
	tests := map[string]string{
		"## Heading":               "Heading",
		"  - [Docs](docs/plan.md)": "Docs",
		"<https://example.com>":    "https://example.com",
		"1. **Numbered item**":     "Numbered item",
		"plain text":               "plain text",
		"`code`":                   "code",
	}

	for input, want := range tests {
		if got := normalizeMarkdownLine(input); got != want {
			t.Fatalf("normalizeMarkdownLine(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeMarkdownLineTruncatesWideTextSafely(t *testing.T) {
	got := normalizeMarkdownLine("## " + strings.Repeat("椿佐", 40) + strings.Repeat(" ", 120))

	if !utf8.ValidString(got) {
		t.Fatalf("normalized line is not valid UTF-8: %q", got)
	}
	if len(got) > 48 {
		t.Fatalf("normalized byte length = %d, want <= 48 for %q", len(got), got)
	}
	if lipgloss.Width(got) > 48 {
		t.Fatalf("normalized width = %d, want <= 48 for %q", lipgloss.Width(got), got)
	}
	if !strings.HasPrefix(got, "椿佐椿佐") {
		t.Fatalf("normalized line = %q, want original heading prefix", got)
	}
}

func TestSlug(t *testing.T) {
	if got := slug("Hello Markdown 101"); got != "hello-markdown-101" {
		t.Fatalf("slug mismatch: %q", got)
	}
	if got := slug("Heading With Punctuation: alpha/beta, gamma?"); got != "heading-with-punctuation-alpha-beta-gamma" {
		t.Fatalf("punctuation slug mismatch: %q", got)
	}
}

func TestRenderOutlineShowsHeadingHierarchy(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 1, Text: "Root", Parent: -1, Children: []int{1, 3}},
			{Level: 2, Text: "Child A", Parent: 0, Children: []int{2}},
			{Level: 3, Text: "Grandchild", Parent: 1},
			{Level: 2, Text: "Child B", Parent: 0},
		},
		Rendered: "Root\nChild A\nGrandchild\nChild B\n",
		Raw:      "# Root\n## Child A\n### Grandchild\n## Child B\n",
	})
	model.outline.Width = 30

	outline := model.renderOutline()
	for _, want := range []string{"Root", "  Child A", "    Grandchild", "  Child B"} {
		if !strings.Contains(outline, want) {
			t.Fatalf("expected outline to contain %q:\n%s", want, outline)
		}
	}
	for _, unwanted := range []string{"├", "└", "│"} {
		if strings.Contains(outline, unwanted) {
			t.Fatalf("expected compact outline without tree symbols:\n%s", outline)
		}
	}
}

func TestOutlineClickUsesWrappedVisualLines(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 1, Text: "Very Long Heading That Wraps", Line: 1, Parent: -1},
			{Level: 2, Text: "Next", Line: 2, Parent: 0},
		},
		Rendered: "Very Long Heading That Wraps\nNext\n",
		Raw:      "# Very Long Heading That Wraps\n## Next\n",
	})
	model.outlineVisible = true
	model.outline.Width = 12
	model.outline.Height = 10
	model.body.Width = 40
	model.body.Height = 10
	model.outline.SetContent(model.renderOutline())

	nextY := model.outlineLineForHeading(1)
	if nextY <= 1 {
		t.Fatalf("expected first heading to wrap before second heading, nextY=%d", nextY)
	}

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      42,
		Y:      nextY,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if got.selectedOutline != 1 {
		t.Fatalf("selectedOutline = %d, want second heading", got.selectedOutline)
	}
	if got.focusedOutline != 1 {
		t.Fatalf("focusedOutline = %d, want clicked heading", got.focusedOutline)
	}
}

func TestOpeningOutlineDoesNotFocusHeadingInBody(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 1, Text: "Root", Line: 1, Parent: -1},
		},
		Rendered: "Root\nbody\n",
		Raw:      "# Root\nbody\n",
	})
	model.body.Height = 10

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'o'}}))
	got := next.(Model)

	if !got.outlineVisible {
		t.Fatal("outlineVisible = false, want open")
	}
	if got.focusedOutline != -1 {
		t.Fatalf("focusedOutline = %d, want no body focus on open", got.focusedOutline)
	}
}

func TestToggleOutlinePreservesTopRawLineAfterRewrap(t *testing.T) {
	longParagraph := strings.Repeat("alpha beta gamma delta ", 18)
	raw := strings.Join([]string{
		"# Root",
		"",
		longParagraph,
		"",
		"## Target",
		"target body",
	}, "\n") + "\n"
	rendered, err := md.Render(raw, 78)
	if err != nil {
		t.Fatal(err)
	}
	outline, _ := md.ParseStructure([]byte(raw))
	model := New(md.Document{
		Outline:  outline,
		Rendered: rendered,
		Raw:      raw,
	})
	model.width = 80
	model.height = 12
	model.ready = true
	model.resize()
	model.body.SetYOffset(model.lineForRaw(5))

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'o'}}))
	got := next.(Model)

	if !got.outlineVisible {
		t.Fatal("outlineVisible = false, want open")
	}
	if topRaw := got.rawLineForRendered(got.body.YOffset); topRaw != 5 {
		t.Fatalf("top raw line = %d, want 5 after outline rewrap; offset=%d target=%d", topRaw, got.body.YOffset, got.lineForRaw(5))
	}
	if got.body.YOffset != got.lineForRaw(5) {
		t.Fatalf("YOffset = %d, want remapped target %d", got.body.YOffset, got.lineForRaw(5))
	}
}

func TestToggleOutlineRewrapsUnbrokenTextWithContinuationMarker(t *testing.T) {
	raw := "# Root\n\n" + strings.Repeat("abcdef", 24) + "\n"
	model := markdownModelAtWidth(t, raw, 78)
	model.width = 80
	model.height = 10
	model.ready = true
	model.resize()

	if !strings.Contains(strings.Join(model.contentLines, "\n"), "↪") {
		t.Fatalf("expected unbroken text to wrap before outline toggle:\n%s", strings.Join(model.contentLines, "\n"))
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got := next.(Model)

	if !got.outlineVisible {
		t.Fatal("outlineVisible = false, want open")
	}
	if !strings.Contains(strings.Join(got.contentLines, "\n"), "↪") {
		t.Fatalf("expected unbroken text to rewrap after outline toggle:\n%s", strings.Join(got.contentLines, "\n"))
	}
	for _, line := range got.contentLines {
		if lipgloss.Width(line) > got.body.Width {
			t.Fatalf("line width = %d, want <= body width %d for %q", lipgloss.Width(line), got.body.Width, line)
		}
	}
}

func TestToggleOutlineClearsSelectionButKeepsViewportAnchor(t *testing.T) {
	raw := strings.Join([]string{
		"# Root",
		"",
		strings.Repeat("alpha beta gamma delta ", 14),
		"",
		"## Target",
		"target body",
	}, "\n") + "\n"
	rendered, err := md.Render(raw, 78)
	if err != nil {
		t.Fatal(err)
	}
	outline, _ := md.ParseStructure([]byte(raw))
	model := New(md.Document{
		Outline:  outline,
		Rendered: rendered,
		Raw:      raw,
	})
	model.width = 80
	model.height = 12
	model.ready = true
	model.resize()
	model.body.SetYOffset(model.lineForRaw(5))
	model.textSelectionStart = selectionPoint{Line: model.lineForRaw(5), Column: 0}
	model.textSelectionEnd = selectionPoint{Line: model.lineForRaw(5), Column: 6}

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'o'}}))
	got := next.(Model)

	if _, _, ok := got.textSelectionRange(); ok {
		t.Fatal("expected outline toggle resize to clear line selection")
	}
	if topRaw := got.rawLineForRendered(got.body.YOffset); topRaw != 5 {
		t.Fatalf("top raw line = %d, want 5 after selection-clearing resize", topRaw)
	}
}

func TestMoveOutlineFocusesHeadingNearTopUntilNextKey(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	lines[20] = "  Target Heading"
	raw := strings.Join(lines, "\n")

	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 2, Text: "Target Heading", Line: 21, Parent: -1},
		},
		Rendered: raw + "\n",
		Raw:      raw + "\n",
	})
	model.outlineVisible = true
	model.body.Height = 20
	model.selectedOutline = -1
	oldOutlineHeadingStyle := outlineHeadingStyle
	outlineHeadingStyle = func(level int, selected bool) lipgloss.Style {
		if selected {
			return lipgloss.NewStyle().Transform(func(s string) string { return "[[" + s + "]]" })
		}
		return oldOutlineHeadingStyle(level, selected)
	}
	defer func() {
		outlineHeadingStyle = oldOutlineHeadingStyle
	}()

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'j'}}))
	got := next.(Model)

	if got.focusedOutline != 0 {
		t.Fatalf("focusedOutline = %d, want 0", got.focusedOutline)
	}
	if got.body.YOffset != got.lineForRaw(21)-6 {
		t.Fatalf("YOffset = %d, want target near 7th row (%d)", got.body.YOffset, got.lineForRaw(21)-6)
	}
	if !strings.Contains(got.body.View(), "[[  Target Heading]]") {
		t.Fatalf("expected body heading to be highlighted:\n%s", got.body.View())
	}

	next, _ = got.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'g'}}))
	cleared := next.(Model)
	if cleared.focusedOutline != -1 {
		t.Fatalf("focusedOutline = %d, want cleared after next key", cleared.focusedOutline)
	}
	if strings.Contains(cleared.body.View(), "[[  Target Heading]]") {
		t.Fatalf("expected body heading focus to clear:\n%s", cleared.body.View())
	}
}

func TestFocusNextLinkDoesNotMoveOutlineSelection(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 1, Text: "Root", Line: 1, Parent: -1},
			{Level: 2, Text: "Links", Line: 4, Parent: 0},
		},
		Links: []md.Link{
			{Text: "Docs", URL: "docs/plan.md", Line: 4},
		},
		Rendered: "Root\n\nLinks\nDocs\n",
		Raw:      "# Root\n\n## Links\n[Docs](docs/plan.md)\n",
	})
	model.selectedOutline = 0

	model.focusNextLink(1)

	if model.selectedLink != 0 {
		t.Fatalf("selectedLink = %d, want 0", model.selectedLink)
	}
	if model.selectedOutline != 0 {
		t.Fatalf("selectedOutline = %d, want unchanged 0", model.selectedOutline)
	}
}

func TestFocusNextLinkSkipsInternalJumpLinks(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 2, Text: "Jump Target", Line: 4, Parent: -1},
		},
		Links: []md.Link{
			{Text: "Jump Target", URL: "#jump-target", Line: 1},
			{Text: "External", URL: "https://example.com", Line: 2},
		},
		Rendered: "Jump Target\nExternal\n\nJump Target\n",
		Raw:      "[Jump Target](#jump-target)\n[External](https://example.com)\n\n## Jump Target\n",
	})

	model.focusNextLink(1)

	if model.selectedLink != 1 {
		t.Fatalf("selectedLink = %d, want external link index 1", model.selectedLink)
	}
}

func TestFocusNextLinkIgnoresInternalOnlyLinks(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 2, Text: "Jump Target", Line: 2, Parent: -1},
		},
		Links: []md.Link{
			{Text: "Jump Target", URL: "#jump-target", Line: 1},
		},
		Rendered: "Jump Target\nJump Target\n",
		Raw:      "[Jump Target](#jump-target)\n## Jump Target\n",
	})

	model.focusNextLink(1)

	if model.selectedLink != -1 {
		t.Fatalf("selectedLink = %d, want no selected link", model.selectedLink)
	}
	if !strings.Contains(model.status, "no URL links") {
		t.Fatalf("status = %q, want no URL links", model.status)
	}
}

func TestInternalLinkJumpFocusesTargetUntilNextKey(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 2, Text: "Jump Target", Line: 3, Parent: -1},
		},
		Links: []md.Link{
			{Text: "Go there", URL: "#jump-target", Line: 1},
		},
		Rendered: "Go there\n\nJump Target\n",
		Raw:      "[Go there](#jump-target)\n\n## Jump Target\n",
	})
	model.selectedLink = 0
	model.body.Width = 60
	oldJumpFocusStyle := jumpFocusStyle
	jumpFocusStyle = lipgloss.NewStyle().Transform(func(s string) string { return "{" + s + "}" })
	defer func() {
		jumpFocusStyle = oldJumpFocusStyle
	}()

	model.followSelection()

	if model.focusedJumpLine != 3 {
		t.Fatalf("focusedJumpLine = %d, want 3", model.focusedJumpLine)
	}
	if model.focusedJumpText != "Jump Target" {
		t.Fatalf("focusedJumpText = %q, want Jump Target", model.focusedJumpText)
	}
	if !strings.Contains(model.body.View(), "{Jump Target") {
		t.Fatalf("expected jump target to be focused:\n%s", model.body.View())
	}

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'j'}}))
	got := next.(Model)
	if got.focusedJumpLine != -1 {
		t.Fatalf("focusedJumpLine = %d, want cleared", got.focusedJumpLine)
	}
	if got.focusedJumpText != "" {
		t.Fatalf("focusedJumpText = %q, want cleared", got.focusedJumpText)
	}
	if strings.Contains(got.body.View(), "{Jump Target") {
		t.Fatalf("expected jump focus to clear after next key:\n%s", got.body.View())
	}
}

func TestJumpFocusHighlightsTargetTextOnly(t *testing.T) {
	model := New(md.Document{
		Rendered: "## Jump Target\n",
		Raw:      "## Jump Target\n",
	})
	model.focusedJumpLine = 1
	model.focusedJumpText = "Jump Target"
	oldJumpFocusStyle := jumpFocusStyle
	jumpFocusStyle = lipgloss.NewStyle().Transform(func(s string) string { return "{" + s + "}" })
	defer func() {
		jumpFocusStyle = oldJumpFocusStyle
	}()

	rendered := model.renderContent()
	if !strings.Contains(rendered, "## {Jump Target}") {
		t.Fatalf("expected only heading text to be focused, got %q", rendered)
	}
	if strings.Contains(rendered, "{##") {
		t.Fatalf("expected heading marker not to be focused, got %q", rendered)
	}
}

func TestJumpFocusOverridesANSIInsideTargetText(t *testing.T) {
	model := New(md.Document{
		Rendered: "## \x1b[33mJump\x1b[0m Target\n",
		Raw:      "## Jump Target\n",
	})
	model.focusedJumpLine = 1
	model.focusedJumpText = "Jump Target"
	oldJumpFocusStyle := jumpFocusStyle
	jumpFocusStyle = lipgloss.NewStyle().Transform(func(s string) string { return "{" + s + "}" })
	defer func() {
		jumpFocusStyle = oldJumpFocusStyle
	}()

	rendered := model.renderContent()
	plain := md.StripANSI(rendered)
	if !strings.Contains(plain, "## {Jump Target}") {
		t.Fatalf("expected full target text to be focused despite inner ANSI, got %q", rendered)
	}
	if strings.Contains(plain, "{Jump}") {
		t.Fatalf("expected focus to span all target words, got %q", rendered)
	}
}

func TestClickInternalLinkOpensJumpConfirmation(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 2, Text: "Jump Target", Line: 3, Parent: -1},
		},
		Links: []md.Link{
			{Text: "Go there", URL: "#jump-target", Line: 1},
		},
		Rendered: "Go there\n\nJump Target\n",
		Raw:      "[Go there](#jump-target)\n\n## Jump Target\n",
	})
	model.body.Width = 60
	model.body.Height = 10

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if got.selectedLink != -1 {
		t.Fatalf("selectedLink = %d, want no focused link", got.selectedLink)
	}
	if got.modalKind != modalConfirmJump {
		t.Fatalf("modalKind = %d, want jump confirmation", got.modalKind)
	}
	if got.pendingJumpURL != "#jump-target" {
		t.Fatalf("pendingJumpURL = %q, want #jump-target", got.pendingJumpURL)
	}
	if got.focusedJumpLine != -1 {
		t.Fatalf("focusedJumpLine = %d, want no jump before confirmation", got.focusedJumpLine)
	}
}

func TestConfirmJumpModalAcceptsAndCancels(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 2, Text: "Jump Target", Line: 3, Parent: -1},
		},
		Rendered: "Go there\n\nJump Target\n",
		Raw:      "[Go there](#jump-target)\n\n## Jump Target\n",
	})
	model.modalKind = modalConfirmJump
	model.pendingJumpURL = "#jump-target"

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'y'}}))
	accepted := next.(Model)
	if accepted.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want dismissed", accepted.modalKind)
	}
	if accepted.focusedJumpLine != 3 {
		t.Fatalf("focusedJumpLine = %d, want 3", accepted.focusedJumpLine)
	}

	model.modalKind = modalConfirmJump
	model.pendingJumpURL = "#jump-target"
	next, _ = model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'n'}}))
	cancelled := next.(Model)
	if cancelled.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want dismissed", cancelled.modalKind)
	}
	if cancelled.focusedJumpLine != -1 {
		t.Fatalf("focusedJumpLine = %d, want no jump", cancelled.focusedJumpLine)
	}
}

func TestFixtureInternalAnchorsResolveToTargetHeadings(t *testing.T) {
	model, outline := fixtureModel(t)

	tests := []struct {
		url         string
		headingText string
	}{
		{url: "#table-samples", headingText: "Table Samples"},
		{url: "#heading-with-punctuation-alpha-beta-gamma", headingText: "Heading With Punctuation: alpha/beta, gamma?"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			model.focusedJumpLine = -1
			model.focusedJumpText = ""

			if !model.followMarkdownLink(tt.url) {
				t.Fatalf("expected %s to be handled", tt.url)
			}
			if model.focusedJumpText != tt.headingText {
				t.Fatalf("focusedJumpText = %q, want %q", model.focusedJumpText, tt.headingText)
			}

			targetLine := headingLine(outline, tt.headingText)
			if model.focusedJumpLine != targetLine {
				t.Fatalf("focusedJumpLine = %d, want %d", model.focusedJumpLine, targetLine)
			}
			if md.StripANSI(model.renderedLines[model.body.YOffset]) == "" {
				t.Fatalf("expected jump to land on rendered content")
			}
			if !strings.Contains(md.StripANSI(model.renderedLines[model.lineForRaw(targetLine)]), tt.headingText) {
				t.Fatalf("target raw line maps away from heading %q: %q", tt.headingText, md.StripANSI(model.renderedLines[model.lineForRaw(targetLine)]))
			}
		})
	}
}

func TestFixtureInternalAnchorClicksOpenJumpConfirmation(t *testing.T) {
	model, outline := fixtureModel(t)
	model.body.Height = 20

	tests := []struct {
		url         string
		headingText string
	}{
		{url: "#table-samples", headingText: "Table Samples"},
		{url: "#heading-with-punctuation-alpha-beta-gamma", headingText: "Heading With Punctuation: alpha/beta, gamma?"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			link := linkByURL(t, model.doc.Links, tt.url)
			model.body.SetYOffset(model.lineForRaw(link.Line))
			model.focusedJumpLine = -1
			model.focusedJumpText = ""
			x := clickXForLink(t, model, link)

			next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
				X:      x,
				Y:      0,
				Action: tea.MouseActionPress,
				Button: tea.MouseButtonLeft,
			}))
			next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
				X:      x,
				Y:      0,
				Action: tea.MouseActionRelease,
				Button: tea.MouseButtonLeft,
			}))
			got := next.(Model)

			if got.modalKind != modalConfirmJump {
				t.Fatalf("modalKind = %d, want jump confirmation", got.modalKind)
			}
			if got.pendingJumpURL != tt.url {
				t.Fatalf("pendingJumpURL = %q, want %q", got.pendingJumpURL, tt.url)
			}
			if headingLine(outline, tt.headingText) == 0 {
				t.Fatalf("missing heading %q", tt.headingText)
			}
		})
	}
}

func TestClickURLCopiesAndShowsModal(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Links: []md.Link{
			{Text: "External", URL: "https://example.com", Line: 1},
		},
		Rendered: "External\n",
		Raw:      "[External](https://example.com)\n",
	})
	model.body.Width = 60
	model.body.Height = 10

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want message", got.modalKind)
	}
	if copied != "https://example.com" {
		t.Fatalf("copied = %q, want URL", copied)
	}
	if !strings.Contains(got.modalTitle, "Copied") {
		t.Fatalf("modalTitle = %q, want Copied", got.modalTitle)
	}
	if strings.Contains(got.modalBody, "https://example.com") {
		t.Fatalf("modalBody = %q, should not include copied URL", got.modalBody)
	}
	if !strings.Contains(got.modalBody, "Copied to clipboard") {
		t.Fatalf("modalBody = %q, want generic copied message", got.modalBody)
	}
}

func TestClickRenderedNamedExternalLinkCopiesHiddenURL(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	raw := "[External](https://example.com)\n"
	rendered, err := md.Render(raw, 80)
	if err != nil {
		t.Fatal(err)
	}
	_, links := md.ParseStructure([]byte(raw))
	model := New(md.Document{
		Links:    links,
		Rendered: rendered,
		Raw:      raw,
	})
	model.body.Width = 60
	model.body.Height = 10
	link := linkByURL(t, links, "https://example.com")
	x := clickXForLink(t, model, link)

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want message", got.modalKind)
	}
	if copied != "https://example.com" {
		t.Fatalf("copied = %q, want hidden URL", copied)
	}
	if strings.Contains(md.StripANSI(got.body.View()), "https://example.com") {
		t.Fatalf("hidden URL should not be visible in body:\n%s", got.body.View())
	}
}

func TestClickOutsideSingleLinkTextDoesNotCopy(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Links: []md.Link{
			{Text: "External", URL: "https://example.com", Line: 1},
		},
		Rendered: "External trailing text\n",
		Raw:      "[External](https://example.com) trailing text\n",
	})
	model.body.Width = 60
	model.body.Height = 10
	x := strings.Index(model.renderedLines[0], "trailing")
	if x < 0 {
		t.Fatal("missing trailing text")
	}

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if copied != "" {
		t.Fatalf("copied = %q, want no clipboard write", copied)
	}
	if got.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want no modal", got.modalKind)
	}
}

func TestClickChoosesLinkByHorizontalPosition(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Links: []md.Link{
			{Text: "jump target", URL: "#jump-target", Line: 1},
			{Text: "external URL", URL: "https://example.com/path?query=fixture", Line: 1},
		},
		Rendered: "Read jump target, then copy external URL.\n",
		Raw:      "Read [the jump target](#jump-target), then copy [the external URL](https://example.com/path?query=fixture).\n",
	})
	model.body.Width = 80
	model.body.Height = 10
	x := strings.Index(model.renderedLines[0], "external URL")
	if x < 0 {
		t.Fatal("missing rendered external URL text")
	}

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want copy message", got.modalKind)
	}
	if copied != "https://example.com/path?query=fixture" {
		t.Fatalf("copied = %q, want external URL", copied)
	}
}

func TestClickBetweenTwoLinksDoesNotChooseEither(t *testing.T) {
	model := New(md.Document{
		Links: []md.Link{
			{Text: "first", URL: "https://example.com/first", Line: 1},
			{Text: "second", URL: "https://example.com/second", Line: 1},
		},
		Rendered: "first gap second\n",
		Raw:      "[first](https://example.com/first) gap [second](https://example.com/second)\n",
	})
	x := strings.Index(model.renderedLines[0], "gap")
	if x < 0 {
		t.Fatal("missing gap text")
	}

	if link, ok := model.linkAtRenderedPosition(0, x, 1); ok {
		t.Fatalf("linkAtRenderedPosition returned %#v for gap click", link)
	}
}

func TestClickLinkFallsBackToVisibleRenderedLine(t *testing.T) {
	model := New(md.Document{
		Links: []md.Link{
			{Text: "External", URL: "https://example.com", Line: 10},
		},
		Rendered: "External\n",
		Raw:      strings.Repeat("plain\n", 9) + "[External](https://example.com)\n",
	})

	link, ok := model.linkAtRenderedPosition(0, 1, 1)
	if !ok {
		t.Fatal("expected visible link to be found despite mismatched raw line")
	}
	if link.URL != "https://example.com" {
		t.Fatalf("URL = %q, want https://example.com", link.URL)
	}
}

func TestClickChoosesLongestOverlappingVisibleLink(t *testing.T) {
	model := New(md.Document{
		Links: []md.Link{
			{Text: "Short", URL: "https://example.com", Line: 1},
			{Text: "Long", URL: "https://example.com/autolink-fixture", Line: 2},
		},
		Rendered: "https://example.com/autolink-fixture\n",
		Raw:      "[Short](https://example.com)\nhttps://example.com/autolink-fixture\n",
	})

	link, ok := model.linkAtRenderedPosition(0, 1, 1)
	if !ok {
		t.Fatal("expected overlapping visible link to be found")
	}
	if link.URL != "https://example.com/autolink-fixture" {
		t.Fatalf("URL = %q, want autolink fixture URL", link.URL)
	}
}

func TestClickURLAfterNonASCIITextUsesDisplayColumns(t *testing.T) {
	url := "https://example.com/path/to/日本語リソース名"
	line := "こちらがURL: " + url
	model := New(md.Document{
		Links: []md.Link{
			{Text: url, URL: url, Line: 1},
		},
		Rendered: line + "\n",
		Raw:      line + "\n",
	})
	x := lipgloss.Width("こちらがURL: ") + lipgloss.Width("https://example.com/path/to/")

	link, ok := model.linkAtRenderedPosition(0, x, 1)
	if !ok {
		t.Fatal("expected URL to be clickable after non-ASCII text")
	}
	if link.URL != url {
		t.Fatalf("URL = %q, want %q", link.URL, url)
	}
}

func TestClickWrappedBareURLContinuationCopiesFullURL(t *testing.T) {
	url := "https://example.com/path/to/日本語リソース名"
	model := New(md.Document{
		Links: []md.Link{
			{Text: url, URL: url, Line: 1},
		},
		Rendered: "こちらがURL: https://example.com/path/to/\n日本語リソース名\n",
		Raw:      "こちらがURL: " + url + "\n",
	})

	link, ok := model.linkAtRenderedPosition(1, lipgloss.Width("↪ 日本"), 1)
	if !ok {
		t.Fatal("expected wrapped URL continuation to be clickable")
	}
	if link.URL != url {
		t.Fatalf("URL = %q, want %q", link.URL, url)
	}
}

func TestClickRenderedWrappedBareURLContinuationCopiesFullURL(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	url := "https://example.com/path/to/日本語リソース名"
	source := "こちらがURL: " + url + "\n"
	model := markdownModelAtWidth(t, source, 36)
	model.body.Width = 36
	model.body.Height = 8

	clickLine := -1
	clickX := 0
	for line, renderedLine := range model.contentLines {
		if strings.Contains(renderedLine, "日本語") {
			clickLine = line
			clickX = lipgloss.Width(renderedLine[:strings.Index(renderedLine, "日本語")])
			break
		}
	}
	if clickLine < 0 {
		t.Fatalf("wrapped URL continuation not found:\n%s", strings.Join(model.contentLines, "\n"))
	}
	model.body.SetYOffset(clickLine)

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      clickX,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      clickX,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if copied != url {
		t.Fatalf("copied = %q, want %q", copied, url)
	}
	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want copied message", got.modalKind)
	}
}

func TestClickFixtureAutolinkCopiesURL(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model, _ := fixtureModel(t)
	link := linkByURL(t, model.doc.Links, "https://example.com/autolink-fixture")
	renderedLine := model.lineForLink(link)
	model.body.SetYOffset(renderedLine)
	model.body.Width = 100
	model.body.Height = 10
	x := strings.Index(md.StripANSI(model.renderedLines[renderedLine]), "https://example.com/autolink-fixture")
	if x < 0 {
		t.Fatalf("autolink not visible on rendered line: %q", md.StripANSI(model.renderedLines[renderedLine]))
	}

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      x,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want copied message", got.modalKind)
	}
	if copied != "https://example.com/autolink-fixture" {
		t.Fatalf("copied = %q, want autolink URL", copied)
	}
}

func TestDragSelectTextCopiesRenderedTextOnRelease(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Rendered: "alpha\n\x1b[31mbeta\x1b[0m\ngamma\n",
		Raw:      "alpha\nbeta\ngamma\n",
	})
	model.body.Width = 40
	model.body.Height = 10

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      2,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      2,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if copied != "lpha\nbeta\ng" {
		t.Fatalf("copied = %q, want selected rendered text", copied)
	}
	if !got.hasLineSelection() {
		t.Fatal("expected drag to leave selected text")
	}
	items := strings.Join(got.guideItems(), " ")
	if strings.Contains(items, "y copy text") {
		t.Fatalf("guide items = %q, should not include copy text hint", items)
	}
	if !strings.Contains(got.status, "copied 3 lines") {
		t.Fatalf("status = %q, want copied line count", got.status)
	}
	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want copied message", got.modalKind)
	}
	if strings.Contains(got.modalBody, copied) {
		t.Fatalf("modalBody = %q, should not include copied text", got.modalBody)
	}
	if !strings.Contains(got.modalBody, "Copied to clipboard") {
		t.Fatalf("modalBody = %q, want generic copied message", got.modalBody)
	}

	next, _ = got.Update(tea.KeyMsg(tea.Key{Type: tea.KeyEsc}))
	dismissed := next.(Model)
	if !dismissed.hasLineSelection() {
		t.Fatal("expected copied selection to remain after closing modal")
	}

	next, _ = dismissed.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'j'}}))
	cleared := next.(Model)
	if cleared.hasLineSelection() {
		t.Fatal("expected next normal key press to clear copied selection")
	}
}

func TestDragSelectTextMouseWheelScrollsAndExtendsSelection(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i)
	}
	text := strings.Join(lines, "\n") + "\n"
	model := New(md.Document{
		Rendered: text,
		Raw:      text,
	})
	model.body.Width = 40
	model.body.Height = 4
	model.body.MouseWheelDelta = 3

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      lipgloss.Width("line 00"),
		Y:      3,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}))
	dragging := next.(Model)
	endBeforeWheel := dragging.textSelectionEnd.Line

	next, _ = dragging.Update(tea.MouseMsg(tea.MouseEvent{
		X:      lipgloss.Width("line 00"),
		Y:      3,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	}))
	got := next.(Model)

	if got.body.YOffset != 3 {
		t.Fatalf("body offset = %d, want wheel delta 3", got.body.YOffset)
	}
	if got.textSelectionEnd.Line <= endBeforeWheel {
		t.Fatalf("selection end line = %d, want after %d", got.textSelectionEnd.Line, endBeforeWheel)
	}
	if !got.hasLineSelection() {
		t.Fatal("expected selection to remain active after wheel scroll")
	}
}

func TestDragSelectTextAutoScrollsNearBottomEdge(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i)
	}
	text := strings.Join(lines, "\n") + "\n"
	model := New(md.Document{
		Rendered: text,
		Raw:      text,
	})
	model.body.Width = 40
	model.body.Height = 6

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      lipgloss.Width("line 00"),
		Y:      5,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}))
	dragging := next.(Model)
	if !dragging.textSelectionAutoScrollActive {
		t.Fatal("expected auto-scroll tick to be scheduled near the bottom edge")
	}
	endBeforeTick := dragging.textSelectionEnd.Line

	next, _ = dragging.Update(lineSelectionAutoScrollMsg{})
	got := next.(Model)

	if got.body.YOffset <= 0 {
		t.Fatalf("body offset = %d, want auto-scroll to move down", got.body.YOffset)
	}
	if got.textSelectionEnd.Line <= endBeforeTick {
		t.Fatalf("selection end line = %d, want after %d", got.textSelectionEnd.Line, endBeforeTick)
	}
	if !got.textSelectionAutoScrollActive {
		t.Fatal("expected auto-scroll to continue while the pointer stays near the bottom edge")
	}
}

func TestLineSelectionAutoScrollSpeedsUpCloserToEdge(t *testing.T) {
	model := New(md.Document{
		Rendered: strings.Repeat("line\n", 20),
		Raw:      strings.Repeat("line\n", 20),
	})
	model.body.Height = 10
	model.textSelectionAnchor = selectionPoint{Line: 0, Column: 0}
	model.textSelectionDragging = true
	model.textSelectionHasLastMouse = true

	model.textSelectionLastMouseY = 6
	bottomSlow := model.lineSelectionAutoScrollDelta()
	model.textSelectionLastMouseY = 9
	bottomFast := model.lineSelectionAutoScrollDelta()

	if bottomSlow <= 0 || bottomFast <= bottomSlow {
		t.Fatalf("bottom deltas slow=%d fast=%d, want faster near edge", bottomSlow, bottomFast)
	}

	model.textSelectionLastMouseY = 3
	topSlow := -model.lineSelectionAutoScrollDelta()
	model.textSelectionLastMouseY = 0
	topFast := -model.lineSelectionAutoScrollDelta()

	if topSlow <= 0 || topFast <= topSlow {
		t.Fatalf("top deltas slow=%d fast=%d, want faster near edge", topSlow, topFast)
	}
}

func TestDragSelectTextJoinsVisualWrapsWithinRawLine(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Rendered: "abcdefghijklmnop\n↪ qrstuvwxyz\n",
		Raw:      "abcdefghijklmnopqrstuvwxyz\n",
	})
	model.body.Width = 40
	model.body.Height = 10

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      lipgloss.Width("↪ qrstuvwxyz"),
		Y:      1,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      lipgloss.Width("↪ qrstuvwxyz"),
		Y:      1,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if copied != "abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("copied = %q, want visual wrap joined without newline", copied)
	}
	if !strings.Contains(got.status, "copied 1 line") {
		t.Fatalf("status = %q, want copied line count based on copied text", got.status)
	}
}

func TestSelectedWrappedCodeBlockDoesNotInsertSpaceInsideToken(t *testing.T) {
	model := New(md.Document{
		Rendered: strings.Join([]string{
			"  ╭─ ruby ─────────────────────────────────╮",
			"  Fixture = Struct.new(:name, :enabled, ke",
			"  yword_init: true)",
			"  ╰────────────────────────────────────────╯",
			"",
		}, "\n"),
		Raw: strings.Join([]string{
			"```ruby",
			"Fixture = Struct.new(:name, :enabled, keyword_init: true)",
			"```",
			"",
		}, "\n"),
	})
	model.textSelectionStart = selectionPoint{
		Line:   1,
		Column: selectionLineStartColumn(model.contentLines[1]),
	}
	model.textSelectionEnd = selectionPoint{
		Line:   2,
		Column: lipgloss.Width(model.contentLines[2]),
	}

	text, _, ok := model.selectedLineText()
	if !ok {
		t.Fatal("expected selected code block text")
	}
	if strings.Contains(text, "ke yword_init") {
		t.Fatalf("copied code block inserted wrap space: %q", text)
	}
	if !strings.Contains(text, "keyword_init") {
		t.Fatalf("copied code block did not preserve wrapped token: %q", text)
	}
}

func TestSelectedTextTrimsRendererPaddingOnEveryLine(t *testing.T) {
	model := New(md.Document{
		Rendered: strings.Join([]string{
			"  Internal links:                                                          ",
			"                                                                         ",
			"  • Jump Target                                                            ",
			"",
		}, "\n"),
		Raw: "Internal links:\n\n- [Jump Target](#jump-target)\n",
	})
	model.textSelectionStart = selectionPoint{
		Line:   0,
		Column: selectionLineStartColumn(model.contentLines[0]),
	}
	model.textSelectionEnd = selectionPoint{
		Line:   2,
		Column: lipgloss.Width(model.contentLines[2]),
	}

	text, _, ok := model.selectedLineText()
	if !ok {
		t.Fatal("expected selected text")
	}
	want := "Internal links:\n\n• Jump Target"
	if text != want {
		t.Fatalf("copied = %q, want %q", text, want)
	}
}

func TestSelectedQuickChecklistPreservesRawBulletBreaks(t *testing.T) {
	model, _ := fixtureModel(t)
	startLine := model.lineForRaw(13)
	endLine := model.lineForRaw(26) - 1
	model.textSelectionStart = selectionPoint{
		Line:   startLine,
		Column: selectionLineStartColumn(model.contentLines[startLine]),
	}
	model.textSelectionEnd = selectionPoint{
		Line:   endLine,
		Column: lipgloss.Width(model.contentLines[endLine]),
	}

	text, _, ok := model.selectedLineText()
	if !ok {
		t.Fatal("expected selected Quick Checklist text")
	}
	normalized := trimLineEndSpaces(text)
	if strings.Contains(normalized, "mouse wheel.         • Press /fixture") {
		t.Fatalf("copied checklist collapsed adjacent bullets: %q", normalized)
	}
	if !strings.Contains(normalized, "mouse wheel.\n• Press /fixture") {
		t.Fatalf("copied checklist did not preserve bullet line break: %q", normalized)
	}
	if !strings.Contains(normalized, "Jump Target.\n• Press y on External Example") {
		t.Fatalf("copied checklist collapsed link bullets: %q", normalized)
	}
	if !strings.Contains(normalized, "• Press ? and search for copy.") {
		t.Fatalf("copied checklist did not include final bullet: %q", normalized)
	}
}

func TestSelectedCodeBlockKeepsBottomBorderOnSeparateLine(t *testing.T) {
	model, _ := fixtureModel(t)
	startLine := model.lineForRaw(215)
	endLine := model.lineForRaw(220) - 1
	model.textSelectionStart = selectionPoint{
		Line:   startLine,
		Column: selectionLineStartColumn(model.contentLines[startLine]),
	}
	model.textSelectionEnd = selectionPoint{
		Line:   endLine,
		Column: lipgloss.Width(model.contentLines[endLine]),
	}

	text, _, ok := model.selectedLineText()
	if !ok {
		t.Fatal("expected selected code block text")
	}
	if strings.Contains(text, "\"));╰") {
		t.Fatalf("copied code block joined final code line and bottom border: %q", text)
	}
	if !strings.Contains(text, "console.log(fixture.join(\", \"));\n╰") {
		t.Fatalf("copied code block did not keep bottom border on a separate line: %q", text)
	}
}

func TestSelectedLongWrappingRestoresSoftWrapSpaces(t *testing.T) {
	model, _ := fixtureModelAtWidth(t, 60)
	startLine := model.lineForRaw(445)
	endLine := model.lineForRaw(449) - 1
	model.textSelectionStart = selectionPoint{
		Line:   startLine,
		Column: selectionLineStartColumn(model.contentLines[startLine]),
	}
	model.textSelectionEnd = selectionPoint{
		Line:   endLine,
		Column: lipgloss.Width(model.contentLines[endLine]),
	}

	text, _, ok := model.selectedLineText()
	if !ok {
		t.Fatal("expected selected Long Wrapping text")
	}
	normalized := trimLineEndSpaces(text)
	if strings.Contains(normalized, "wrapacross") {
		t.Fatalf("copied long paragraph collapsed soft-wrap space: %q", normalized)
	}
	if !strings.Contains(normalized, "wrap across multiple terminal widths") {
		t.Fatalf("copied long paragraph did not restore soft-wrap spaces: %q", normalized)
	}
	if strings.Contains(normalized, "↪") {
		t.Fatalf("copied long paragraph included visual wrap marker: %q", normalized)
	}
}

func TestSelectedNestedListsPreservesItemBreaks(t *testing.T) {
	model, _ := fixtureModelAtWidth(t, 28)
	startLine := model.lineForRaw(138)
	endLine := lastRenderedLineForRaw(model, 143)
	model.textSelectionStart = selectionPoint{
		Line:   startLine,
		Column: selectionLineStartColumn(model.contentLines[startLine]),
	}
	model.textSelectionEnd = selectionPoint{
		Line:   endLine,
		Column: lipgloss.Width(model.contentLines[endLine]),
	}

	text, _, ok := model.selectedLineText()
	if !ok {
		t.Fatal("expected selected nested list text")
	}
	normalized := trimLineEndSpaces(text)
	if strings.Contains(normalized, "numbered item     1. Nested") {
		t.Fatalf("copied nested list collapsed adjacent items: %q", normalized)
	}
	if !strings.Contains(normalized, "First numbered item\n  1. Nested numbered item") {
		t.Fatalf("copied nested list did not preserve nested item break: %q", normalized)
	}
	if !strings.Contains(normalized, "Second numbered item\n  • Mixed bullet under numbered item") {
		t.Fatalf("copied nested list did not preserve mixed bullet break: %q", normalized)
	}
	if !strings.Contains(normalized, "Mixed bullet under numbered item\n    • Mixed nested bullet under numbered item") {
		t.Fatalf("copied nested list did not preserve deep mixed bullet break: %q", normalized)
	}
	if strings.Contains(normalized, "↪") {
		t.Fatalf("copied nested list included visual wrap marker: %q", normalized)
	}
}

func TestSelectedTaskListPreservesChildItemBreaks(t *testing.T) {
	source := strings.Join([]string{
		"### ダミー確認セクション",
		"",
		"- [x] ダミー項目その一",
		"  - 補足説明: ダミーの説明文をここに長めに置く。幅が狭くても別行として扱い、次の行と連結しない。",
		"- [x] ダミー項目その二",
		"  - 補足説明: 二つ目の説明文をここに長めに置く。同じ接頭辞で始まっても別行として扱う。",
		"- [x] ダミー項目その三",
		"  - 補足説明: 三つ目の説明文をここに長めに置く。同じ接頭辞でも前の行に吸い寄せない。",
	}, "\n") + "\n"
	rendered, err := md.Render(source, 54)
	if err != nil {
		t.Fatal(err)
	}
	model := New(md.Document{Raw: source, Rendered: rendered})
	model.body.Width = 54
	model.body.Height = 12
	model.rebuildContent()

	startLine := model.lineForRaw(1)
	endLine := lastRenderedLineForRaw(model, 8)
	model.textSelectionStart = selectionPoint{
		Line:   startLine,
		Column: selectionLineStartColumn(model.contentLines[startLine]),
	}
	model.textSelectionEnd = selectionPoint{
		Line:   endLine,
		Column: lipgloss.Width(model.contentLines[endLine]),
	}

	text, _, ok := model.selectedLineText()
	if !ok {
		t.Fatal("expected selected task list text")
	}
	normalized := trimLineEndSpaces(text)
	if strings.Contains(normalized, "ダミー項目その一 • 補足結果") {
		t.Fatalf("copied task item collapsed into child bullet: %q", normalized)
	}
	if strings.Contains(normalized, "次の行と連結しない。 [x] ダミー項目その二") {
		t.Fatalf("copied child bullet collapsed into next task: %q", normalized)
	}
	if !strings.Contains(normalized, "[x] ダミー項目その一\n  • 補足説明") {
		t.Fatalf("copied task list did not preserve child item break: %q", normalized)
	}
	if !strings.Contains(normalized, "別行として扱う。\n[x] ダミー項目その三") {
		t.Fatalf("copied task list did not preserve task item break: %q", normalized)
	}
	if strings.Contains(normalized, "↪") {
		t.Fatalf("copied task list included visual wrap marker: %q", normalized)
	}
}

func TestMixedListRegressionKeepsRawLineMapAndWrapMarkers(t *testing.T) {
	source := strings.Join([]string{
		"### ダミー混在セクション",
		"",
		"- [ ] 共通接頭辞タスクその一として長い説明文を置く",
		"  - 補足説明: 共通接頭辞の子要素その一として長い説明文を置く",
		"- [x] 共通接頭辞タスクその二として長い説明文を置く",
		"  - 補足説明: 共通接頭辞の子要素その二として長い説明文を置く",
		"- 共通接頭辞の通常箇条書きその一として長い説明文を置く",
		"  - 共通接頭辞の通常箇条書きその二として長い説明文を置く",
		"",
		"これは普通の長い段落として視覚上の折り返しマーカーが必要になるだけの説明文をここに置く。",
	}, "\n") + "\n"
	model := markdownModelAtWidth(t, source, 42)

	tests := []struct {
		raw  int
		want string
	}{
		{raw: 3, want: "[ ]"},
		{raw: 4, want: "•"},
		{raw: 5, want: "[x]"},
		{raw: 6, want: "•"},
		{raw: 7, want: "•"},
		{raw: 8, want: "•"},
	}
	seen := make(map[int]int)
	for _, tt := range tests {
		line := model.lineForRaw(tt.raw)
		if gotRaw := model.rawLineForRendered(line); gotRaw != tt.raw {
			t.Fatalf("raw %d maps to rendered line %d with raw %d", tt.raw, line, gotRaw)
		}
		if !strings.Contains(model.contentLines[line], tt.want) {
			t.Fatalf("raw %d rendered line = %q, want marker %q", tt.raw, model.contentLines[line], tt.want)
		}
		if previousRaw, exists := seen[line]; exists {
			t.Fatalf("raw %d and raw %d map to the same rendered line %d", previousRaw, tt.raw, line)
		}
		seen[line] = tt.raw
		for renderedLine := line; renderedLine <= lastRenderedLineForRaw(model, tt.raw); renderedLine++ {
			if strings.Contains(model.contentLines[renderedLine], "↪ [") || strings.Contains(model.contentLines[renderedLine], "↪ •") {
				t.Fatalf("raw %d rendered line %d has list continuation marker: %q", tt.raw, renderedLine, model.contentLines[renderedLine])
			}
		}
	}

	paragraphLine := model.lineForRaw(10)
	foundParagraphMarker := false
	for line := paragraphLine + 1; line <= lastRenderedLineForRaw(model, 10); line++ {
		if strings.Contains(model.contentLines[line], "↪") {
			foundParagraphMarker = true
			break
		}
	}
	if !foundParagraphMarker {
		t.Fatalf("expected ordinary paragraph raw line 10 to keep visible continuation marker:\n%s", strings.Join(model.contentLines, "\n"))
	}
}

func TestComprehensiveFixtureCoversTaskBulletAndJapaneseRegressionCases(t *testing.T) {
	model, _ := fixtureModelAtWidth(t, 42)
	firstTaskRaw := rawLineContaining(t, model.doc.Raw, "fixture-long-checkbox-ja:")
	secondTaskRaw := rawLineContaining(t, model.doc.Raw, "fixture-long-checkbox-ja-second:")
	childBulletRaw := rawLineContaining(t, model.doc.Raw, "fixture-child-bullet-one:")
	nestedTaskRaw := rawLineContaining(t, model.doc.Raw, "fixture-nested-checkbox-pending:")
	japaneseParagraphRaw := rawLineContaining(t, model.doc.Raw, "fixture-ja-long-paragraph:")
	japaneseUnbrokenRaw := rawLineContaining(t, model.doc.Raw, "fixture-ja-unbroken:")
	wrappedURLRaw := rawLineContaining(t, model.doc.Raw, "fixture-wrapped-url-line:")

	firstTaskLine := model.lineForRaw(firstTaskRaw)
	secondTaskLine := model.lineForRaw(secondTaskRaw)
	if firstTaskLine == secondTaskLine {
		t.Fatalf("long checkbox raw lines map to the same rendered line: first=%d second=%d", firstTaskLine, secondTaskLine)
	}
	for _, tt := range []struct {
		name string
		raw  int
		want string
	}{
		{name: "first long checkbox", raw: firstTaskRaw, want: "[ ]"},
		{name: "second long checkbox", raw: secondTaskRaw, want: "[ ]"},
		{name: "child bullet", raw: childBulletRaw, want: "•"},
		{name: "nested checkbox", raw: nestedTaskRaw, want: "[ ]"},
		{name: "wrapped URL list item", raw: wrappedURLRaw, want: "•"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			line := model.lineForRaw(tt.raw)
			if gotRaw := model.rawLineForRendered(line); gotRaw != tt.raw {
				t.Fatalf("raw %d maps to rendered line %d with raw %d", tt.raw, line, gotRaw)
			}
			if !strings.Contains(model.contentLines[line], tt.want) {
				t.Fatalf("raw %d rendered line = %q, want marker %q", tt.raw, model.contentLines[line], tt.want)
			}
			for renderedLine := line; renderedLine <= lastRenderedLineForRaw(model, tt.raw); renderedLine++ {
				if strings.Contains(model.contentLines[renderedLine], "↪ [") || strings.Contains(model.contentLines[renderedLine], "↪ •") {
					t.Fatalf("raw %d rendered line %d has list continuation marker: %q", tt.raw, renderedLine, model.contentLines[renderedLine])
				}
			}
		})
	}

	for _, tt := range []struct {
		name string
		raw  int
	}{
		{name: "Japanese paragraph", raw: japaneseParagraphRaw},
		{name: "Japanese unbroken text", raw: japaneseUnbrokenRaw},
	} {
		t.Run(tt.name, func(t *testing.T) {
			foundMarker := false
			for line := model.lineForRaw(tt.raw) + 1; line <= lastRenderedLineForRaw(model, tt.raw); line++ {
				if strings.Contains(model.contentLines[line], "↪") {
					foundMarker = true
					break
				}
			}
			if !foundMarker {
				t.Fatalf("%s raw line %d did not show a continuation marker:\n%s", tt.name, tt.raw, strings.Join(model.contentLines, "\n"))
			}
		})
	}
}

func TestSelectedExternalLinksKeepsWrappedURLContinuous(t *testing.T) {
	model, _ := fixtureModelAtWidth(t, 58)
	startLine := model.lineForRaw(68)
	endLine := lastRenderedLineForRaw(model, 73)
	model.textSelectionStart = selectionPoint{
		Line:   startLine,
		Column: selectionLineStartColumn(model.contentLines[startLine]),
	}
	model.textSelectionEnd = selectionPoint{
		Line:   endLine,
		Column: lipgloss.Width(model.contentLines[endLine]),
	}

	text, _, ok := model.selectedLineText()
	if !ok {
		t.Fatal("expected selected external links text")
	}
	normalized := trimLineEndSpaces(text)
	if strings.Contains(normalized, "get- started") {
		t.Fatalf("copied URL inserted soft-wrap space: %q", normalized)
	}
	if !strings.Contains(normalized, "https://docs.github.com/en/get-started/writing-on-github") {
		t.Fatalf("copied URL was not kept continuous: %q", normalized)
	}
	if strings.Contains(normalized, "↪") {
		t.Fatalf("copied external links included visual wrap marker: %q", normalized)
	}
}

func TestSoftWrapMarkersSkipQuotesAndTables(t *testing.T) {
	model, _ := fixtureModelAtWidth(t, 60)
	for raw := 381; raw < 394; raw++ {
		for line := model.lineForRaw(raw); line < len(model.contentLines) && model.rawLineForRendered(line) == raw; line++ {
			if strings.Contains(model.contentLines[line], "↪") {
				t.Fatalf("quote raw line %d rendered line %d has unexpected marker: %q", raw, line, model.contentLines[line])
			}
		}
	}
	for raw := 394; raw < 406; raw++ {
		for line := model.lineForRaw(raw); line < len(model.contentLines) && model.rawLineForRendered(line) == raw; line++ {
			if strings.Contains(model.contentLines[line], "↪") {
				t.Fatalf("table raw line %d rendered line %d has unexpected marker: %q", raw, line, model.contentLines[line])
			}
		}
	}
}

func TestSoftWrapMarkersSkipFirstRenderedContentForRawLine(t *testing.T) {
	model, _ := fixtureModelAtWidth(t, 60)
	firstHeading := model.lineForRaw(1)
	if strings.Contains(model.contentLines[firstHeading], "↪") {
		t.Fatalf("first heading line has unexpected wrap marker: %q", model.contentLines[firstHeading])
	}

	longWrapping := model.lineForRaw(447)
	foundMarker := false
	for line := longWrapping + 1; line < len(model.contentLines) && model.rawLineForRendered(line) == 447; line++ {
		if strings.Contains(model.contentLines[line], "↪") {
			foundMarker = true
			break
		}
	}
	if !foundMarker {
		t.Fatal("expected soft-wrap continuation marker in long paragraph")
	}
}

func TestSoftWrapMarkersSkipTaskListItems(t *testing.T) {
	source := strings.Join([]string{
		"- [x] 長いチェック項目その一として画面幅を越えるだけの説明文を置く",
		"- [ ] 長いチェック項目その二として行対応を確認する説明文を置く",
		"- [ ] 長いチェック項目その三として折り返し表示を確認する説明文を置く",
		"- [ ] 長いチェック項目その四として次の行にも影響しないことを見る",
		"- [ ] 長いチェック項目その五として最後の項目も普通に表示される",
	}, "\n") + "\n"
	rendered, err := md.Render(source, 72)
	if err != nil {
		t.Fatal(err)
	}
	model := New(md.Document{Raw: source, Rendered: rendered})
	model.body.Width = 72
	model.body.Height = 10
	model.rebuildContent()

	for _, line := range model.contentLines {
		if strings.Contains(line, "↪ [") {
			t.Fatalf("task list item has unexpected wrap marker: %q\nall lines:\n%s", line, strings.Join(model.contentLines, "\n"))
		}
	}
}

func TestSoftWrapMarkersSkipNestedTaskBullets(t *testing.T) {
	source := strings.Join([]string{
		"- [x] 長いチェック項目その一として本文が複数行に分かれるだけの説明文を置く",
		"  - 補足説明: 子要素にも十分に長い説明文を置いて折り返しと箇条書きの組み合わせを確認する。",
		"- [x] 長いチェック項目その二として本文とチェックボックスが分離する幅を使う",
		"  - 補足あり。",
	}, "\n") + "\n"
	rendered, err := md.Render(source, 96)
	if err != nil {
		t.Fatal(err)
	}
	model := New(md.Document{Raw: source, Rendered: rendered})
	model.body.Width = 96
	model.body.Height = 10
	model.rebuildContent()

	for _, line := range model.contentLines {
		if strings.Contains(line, "↪ •") || strings.Contains(line, "↪ [") {
			t.Fatalf("nested task list line has unexpected wrap marker: %q\nall lines:\n%s", line, strings.Join(model.contentLines, "\n"))
		}
	}
}

func trimLineEndSpaces(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func lastRenderedLineForRaw(model Model, raw int) int {
	line := model.lineForRaw(raw)
	for line+1 < len(model.contentLines) && model.rawLineForRendered(line+1) == raw {
		line++
	}
	return line
}

func TestDragSelectTextSkipsVisualLeftMargin(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Rendered: "  alpha\n  beta\n",
		Raw:      "alpha\nbeta\n",
	})
	model.body.Width = 40
	model.body.Height = 10

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      8,
		Y:      1,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      8,
		Y:      1,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))

	if copied != "alpha\nbeta" {
		t.Fatalf("copied = %q, want text without visual margin", copied)
	}
}

func TestDragSelectTextTrimsCopiedSelectionEdges(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Rendered: "\n  alpha  \n  beta  \n\n",
		Raw:      "\nalpha\nbeta\n\n",
	})
	model.body.Width = 40
	model.body.Height = 10

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      8,
		Y:      4,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      8,
		Y:      4,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if copied != "alpha\nbeta" {
		t.Fatalf("copied = %q, want line-end padding trimmed", copied)
	}
	if !strings.Contains(got.status, "copied 2 lines") {
		t.Fatalf("status = %q, want trimmed line count", got.status)
	}
}

func TestDragSelectTextHighlightsTextThroughInnerANSI(t *testing.T) {
	model := New(md.Document{
		Rendered: "\x1b[31malpha\x1b[0m beta\n",
		Raw:      "alpha beta\n",
	})
	model.body.Width = 40
	model.textSelectionStart = selectionPoint{Line: 0, Column: 1}
	model.textSelectionEnd = selectionPoint{Line: 0, Column: 6}

	oldLineSelectionStyle := lineSelectionStyle
	lineSelectionStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[[" + s + "]]" })
	defer func() {
		lineSelectionStyle = oldLineSelectionStyle
	}()

	rendered := model.renderContent()
	if !strings.Contains(rendered, "a[[lpha ]]beta") {
		t.Fatalf("expected selection highlight to wrap visible text, got %q", rendered)
	}
	if strings.Contains(rendered, "\x1b[31m") {
		t.Fatalf("expected text selection to override inner ANSI, got %q", rendered)
	}
}

func TestYCopiesFocusedLinkInsteadOfSelectedLine(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Links: []md.Link{
			{Text: "External", URL: "https://example.com", Line: 1},
		},
		Rendered: "External\nplain\n",
		Raw:      "[External](https://example.com)\nplain\n",
	})
	model.selectedLink = 0
	model.textSelectionStart = selectionPoint{Line: 1, Column: 0}
	model.textSelectionEnd = selectionPoint{Line: 1, Column: 5}

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'y'}}))
	got := next.(Model)

	if copied != "https://example.com" {
		t.Fatalf("copied = %q, want focused link URL", copied)
	}
	if !strings.Contains(got.status, "copied: https://example.com") {
		t.Fatalf("status = %q, want link copy status", got.status)
	}
	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want copied message", got.modalKind)
	}
}

func TestCopiedDragSelectionSurvivesOutsideClickDismiss(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Rendered: "alpha\nbeta\n",
		Raw:      "alpha\nbeta\n",
	})
	model.width = 80
	model.height = 20
	model.body.Width = 40
	model.body.Height = 10

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      4,
		Y:      1,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      4,
		Y:      1,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	copiedModel := next.(Model)
	if copied != "alpha\nbeta" {
		t.Fatalf("copied = %q, want selected text", copied)
	}
	if copiedModel.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want copied message", copiedModel.modalKind)
	}

	next, _ = copiedModel.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	dismissed := next.(Model)
	if dismissed.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want dismissed", dismissed.modalKind)
	}
	if !dismissed.hasLineSelection() {
		t.Fatal("expected selection to remain after dismissing copied modal")
	}

	next, _ = dismissed.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	afterRelease := next.(Model)
	if !afterRelease.hasLineSelection() {
		t.Fatal("expected release from dismiss click to be ignored")
	}

	next, _ = afterRelease.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'j'}}))
	cleared := next.(Model)
	if cleared.hasLineSelection() {
		t.Fatal("expected next key press to clear copied selection")
	}
}

func TestCopiedDragSelectionClearsOnNextClickAfterDismiss(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	clipboardWrite = func(text string) error {
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Rendered: "alpha\nbeta\n",
		Raw:      "alpha\nbeta\n",
	})
	model.width = 80
	model.height = 20
	model.body.Width = 40
	model.body.Height = 10

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      4,
		Y:      1,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	}))
	next, _ = next.(Model).Update(tea.MouseMsg(tea.MouseEvent{
		X:      4,
		Y:      1,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	copiedModel := next.(Model)

	next, _ = copiedModel.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	dismissed := next.(Model)
	next, _ = dismissed.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}))
	afterDismissRelease := next.(Model)
	if !afterDismissRelease.hasLineSelection() {
		t.Fatal("expected dismiss click release not to clear copied selection")
	}

	next, _ = afterDismissRelease.Update(tea.MouseMsg(tea.MouseEvent{
		X:      1,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)
	if got.hasLineSelection() {
		t.Fatal("expected next normal click to clear copied selection")
	}
}

func TestYCopyFocusedLinkShowsCopiedModal(t *testing.T) {
	oldClipboardWrite := clipboardWrite
	var copied string
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	defer func() {
		clipboardWrite = oldClipboardWrite
	}()

	model := New(md.Document{
		Links: []md.Link{
			{Text: "External", URL: "https://example.com", Line: 1},
		},
		Rendered: "External\n",
		Raw:      "[External](https://example.com)\n",
	})
	model.selectedLink = 0

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'y'}}))
	got := next.(Model)

	if copied != "https://example.com" {
		t.Fatalf("copied = %q, want URL", copied)
	}
	if got.modalKind != modalMessage {
		t.Fatalf("modalKind = %d, want copied message", got.modalKind)
	}
	if strings.Contains(got.modalBody, "https://example.com") {
		t.Fatalf("modalBody = %q, should not include copied URL", got.modalBody)
	}
}

func TestFocusNextLinkClearsLineSelectionWhenNoLinks(t *testing.T) {
	model := New(md.Document{
		Rendered: "alpha\nbeta\n",
		Raw:      "alpha\nbeta\n",
	})
	model.textSelectionStart = selectionPoint{Line: 0, Column: 0}
	model.textSelectionEnd = selectionPoint{Line: 0, Column: 5}
	oldLineSelectionStyle := lineSelectionStyle
	lineSelectionStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[[" + s + "]]" })
	defer func() {
		lineSelectionStyle = oldLineSelectionStyle
	}()
	model.rebuildContent()
	if !strings.Contains(model.body.View(), "[[alpha") {
		t.Fatalf("expected selected line to be highlighted before tab:\n%s", model.body.View())
	}

	next, _ := model.Update(tea.KeyMsg(tea.Key{Type: tea.KeyTab}))
	got := next.(Model)

	if got.hasLineSelection() {
		t.Fatal("expected tab to clear text selection")
	}
	if strings.Contains(got.body.View(), "[[alpha") {
		t.Fatalf("expected selection highlight to clear when no links exist:\n%s", got.body.View())
	}
}

func TestMessageModalDismissesWhenClickingOutside(t *testing.T) {
	model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
	model.width = 80
	model.height = 20
	model.modalKind = modalMessage
	model.modalTitle = "Copied"
	model.modalBody = "Copied to clipboard."

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)
	if got.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want dismissed", got.modalKind)
	}
}

func TestMessageModalOutsideClickClearsTextSelection(t *testing.T) {
	model := New(md.Document{Rendered: "alpha\nbeta\n", Raw: "alpha\nbeta\n"})
	model.width = 80
	model.height = 20
	model.body.Width = 40
	model.body.Height = 10
	model.textSelectionStart = selectionPoint{Line: 0, Column: 0}
	model.textSelectionEnd = selectionPoint{Line: 1, Column: 4}
	model.modalKind = modalMessage
	model.modalTitle = "Copied"
	model.modalBody = "Copied to clipboard."

	oldLineSelectionStyle := lineSelectionStyle
	lineSelectionStyle = lipgloss.NewStyle().Transform(func(s string) string { return "[[" + s + "]]" })
	defer func() {
		lineSelectionStyle = oldLineSelectionStyle
	}()
	model.rebuildContent()
	if !strings.Contains(model.body.View(), "[[alpha") {
		t.Fatalf("expected selection before outside click:\n%s", model.body.View())
	}

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if got.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want dismissed", got.modalKind)
	}
	if got.hasLineSelection() {
		t.Fatal("expected outside click to clear text selection")
	}
	if strings.Contains(got.body.View(), "[[alpha") {
		t.Fatalf("expected selection highlight to clear:\n%s", got.body.View())
	}
}

func TestConfirmModalDismissesWhenClickingOutside(t *testing.T) {
	model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
	model.width = 80
	model.height = 20
	model.modalKind = modalConfirmJump
	model.modalTitle = "Jump?"
	model.modalBody = "Jump to #target?\n\ny confirm   n cancel"
	model.pendingJumpURL = "#target"

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      0,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)
	if got.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want dismissed", got.modalKind)
	}
}

func TestConfirmModalButtonsAreClickable(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 2, Text: "Jump Target", Line: 3, Parent: -1},
		},
		Rendered: "Go there\n\nJump Target\n",
		Raw:      "[Go there](#jump-target)\n\n## Jump Target\n",
	})
	model.width = 80
	model.height = 20
	model.modalKind = modalConfirmJump
	model.modalTitle = "Jump?"
	model.modalBody = "Jump to #jump-target?\n\ny confirm   n cancel"
	model.pendingJumpURL = "#jump-target"

	left, _, y, ok := model.modalConfirmBounds()
	if !ok {
		t.Fatal("expected confirm bounds")
	}
	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      left,
		Y:      y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	confirmed := next.(Model)
	if confirmed.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want dismissed", confirmed.modalKind)
	}
	if confirmed.focusedJumpLine != 3 {
		t.Fatalf("focusedJumpLine = %d, want 3", confirmed.focusedJumpLine)
	}

	model.modalKind = modalConfirmJump
	model.modalTitle = "Jump?"
	model.modalBody = "Jump to #jump-target?\n\ny confirm   n cancel"
	model.pendingJumpURL = "#jump-target"
	left, _, y, ok = model.modalCancelBounds()
	if !ok {
		t.Fatal("expected cancel bounds")
	}
	next, _ = model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      left,
		Y:      y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	cancelled := next.(Model)
	if cancelled.modalKind != modalNone {
		t.Fatalf("modalKind = %d, want dismissed", cancelled.modalKind)
	}
	if cancelled.focusedJumpLine != -1 {
		t.Fatalf("focusedJumpLine = %d, want no jump", cancelled.focusedJumpLine)
	}
}

func TestModalOverlayHasNoCloseButton(t *testing.T) {
	model := New(md.Document{Rendered: "body\n", Raw: "body\n"})
	model.width = 80
	model.height = 20
	model.ready = true
	model.modalKind = modalMessage
	model.modalTitle = "Copied"
	model.modalBody = "https://example.com"

	view := md.StripANSI(model.View())
	if !strings.Contains(view, "Copied") {
		t.Fatalf("expected modal title:\n%s", view)
	}
	if strings.Contains(view, "[ x ]") {
		t.Fatalf("expected no close button in modal:\n%s", view)
	}
}

func TestModalOverlayPreservesContentAroundBox(t *testing.T) {
	base := strings.Join([]string{
		"aaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbb",
		"cccccccccccccccccccc",
	}, "\n")
	box := strings.Join([]string{
		"╭──╮",
		"│ok│",
		"╰──╯",
	}, "\n")

	got := overlayBoxAt(base, box, 8, 0)
	lines := strings.Split(md.StripANSI(got), "\n")

	if lines[0] != "aaaaaaaa╭──╮aaaaaaaa" {
		t.Fatalf("top overlay line = %q", lines[0])
	}
	if lines[1] != "bbbbbbbb│ok│bbbbbbbb" {
		t.Fatalf("middle overlay line = %q", lines[1])
	}
	if lines[2] != "cccccccc╰──╯cccccccc" {
		t.Fatalf("bottom overlay line = %q", lines[2])
	}
}

func TestOverlayBoxRestoresUnderlyingSelectionAfterBox(t *testing.T) {
	base := "\x1b[48;5;116mabcdefghijklmnopqrst\x1b[0m"
	box := "╭──╮"

	got := overlayBoxAt(base, box, 8, 0)
	if plain := md.StripANSI(got); plain != "abcdefgh╭──╮mnopqrst" {
		t.Fatalf("plain overlay = %q", plain)
	}
	if !strings.Contains(got, "\x1b[0m╭──╮\x1b[0m\x1b[48;5;116m") {
		t.Fatalf("expected overlay to reset around box and restore selection style after it, got %q", got)
	}
}

func TestFixtureTableSamplesHeadingDoesNotMapToLinksList(t *testing.T) {
	model, outline := fixtureModel(t)
	link := linkByURL(t, model.doc.Links, "#table-samples")
	targetLine := headingLine(outline, "Table Samples")

	linkRenderedLine := model.lineForRaw(link.Line)
	targetRenderedLine := model.lineForRaw(targetLine)

	if targetRenderedLine <= linkRenderedLine {
		t.Fatalf("target rendered line = %d, want after link rendered line %d", targetRenderedLine, linkRenderedLine)
	}
	if !strings.Contains(md.StripANSI(model.renderedLines[targetRenderedLine]), "Table Samples") {
		t.Fatalf("target rendered line does not contain heading: %q", md.StripANSI(model.renderedLines[targetRenderedLine]))
	}
}

func TestLongWideHeadingsWithTrailingSpacesMapToRenderedHeadingLines(t *testing.T) {
	firstHeading := "椿佐の佐和恒柊"
	secondHeading := "4-0紫: C椿 / Cbjtqj はテーレチァヒイる PgcrXz 凪朗拍畔"
	raw := strings.Join([]string{
		"## " + firstHeading + strings.Repeat(" ", 180),
		"",
		strings.Repeat("NI52 0Iま朔南滋畔ぉ紘咲ほくんむく、ゅはょE和 / Loopyjd Mmplidiaoyw Oypcjrindd / Vcwjvx る拓岳、", 4),
		"",
		strings.Repeat("るみ蒼う、暢暁ろPhkrfwし佳ゃ巴恒若緒和畔ぃ宙紘む謹倭、乃謙ゃKrhcTeな呈伊セ伍若ふむ和佳ひ紺うふ、", 4),
		"",
		strings.Repeat("きぁ、HpfzHyま淡祐よつしぉSP雫紫玄誠ぃいにたひ、南宰おィノアェフ渓笹わ薫もラヒリノャセネゃ、", 4),
		"",
		"## " + secondHeading + strings.Repeat(" ", 180),
		"",
		strings.Repeat("謙迅紘拍の暢佐き、Gepuelぅ萌ぅ庭樹伍朗ゥ", 10),
	}, "\n") + "\n"

	rendered, err := md.Render(raw, 76)
	if err != nil {
		t.Fatal(err)
	}
	outline, links := md.ParseStructure([]byte(raw))
	model := New(md.Document{
		Path:     "wide-headings.md",
		Raw:      raw,
		Rendered: rendered,
		Outline:  outline,
		Links:    links,
	})

	if len(outline) != 2 {
		t.Fatalf("outline length = %d, want 2", len(outline))
	}
	for i, heading := range outline {
		renderedLine := model.lineForRaw(heading.Line)
		if renderedLine < 0 || renderedLine >= len(model.renderedLines) {
			t.Fatalf("heading %d rendered line = %d, outside rendered lines", i, renderedLine)
		}
		line := md.StripANSI(model.renderedLines[renderedLine])
		key := normalizeMarkdownLine("## " + heading.Text)
		if !strings.Contains(line, key) {
			t.Fatalf("heading %d mapped to %q, want line containing %q", i, line, key)
		}
	}

	model.body.SetYOffset(model.lineForRaw(outline[1].Line))
	if got := model.currentHeadingIndex(); got != 1 {
		t.Fatalf("currentHeadingIndex = %d, want second heading", got)
	}
}

func fixtureModel(t *testing.T) (Model, []md.Heading) {
	return fixtureModelAtWidth(t, 100)
}

func fixtureModelAtWidth(t *testing.T, width int) (Model, []md.Heading) {
	t.Helper()
	source, err := os.ReadFile("../../testdata/fixtures/comprehensive.md")
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := md.Render(string(source), width)
	if err != nil {
		t.Fatal(err)
	}
	outline, links := md.ParseStructure(source)
	model := New(md.Document{
		Path:     "comprehensive.md",
		Raw:      string(source),
		Rendered: rendered,
		Outline:  outline,
		Links:    links,
	})
	return model, outline
}

func markdownModelAtWidth(t *testing.T, source string, width int) Model {
	t.Helper()
	rendered, err := md.Render(source, width)
	if err != nil {
		t.Fatal(err)
	}
	outline, links := md.ParseStructure([]byte(source))
	model := New(md.Document{
		Raw:      source,
		Rendered: rendered,
		Outline:  outline,
		Links:    links,
	})
	model.body.Width = width
	model.body.Height = 12
	model.rebuildContent()
	return model
}

func taskFixtureModel(t *testing.T, source string) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.md")
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	rendered, err := md.Render(source, 100)
	if err != nil {
		t.Fatal(err)
	}
	outline, links := md.ParseStructure([]byte(source))
	model := New(md.Document{
		Path:     path,
		Raw:      source,
		Rendered: rendered,
		Outline:  outline,
		Links:    links,
	})
	model.body.Width = 100
	model.body.Height = 10
	model.rebuildContent()
	return model, path
}

func lineContaining(lines []string, needle string) int {
	for i, line := range lines {
		if strings.Contains(line, needle) {
			return i
		}
	}
	return -1
}

func rawLineContaining(t *testing.T, raw, needle string) int {
	t.Helper()
	for i, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	t.Fatalf("raw line containing %q not found", needle)
	return 0
}

func readmeKeyTableKeys(t *testing.T, readme string) []string {
	t.Helper()
	lines := strings.Split(readme, "\n")
	inKeys := false
	keys := make([]string, 0)
	for _, line := range lines {
		switch {
		case strings.TrimSpace(line) == "## Keys":
			inKeys = true
			continue
		case inKeys && strings.HasPrefix(line, "## "):
			inKeys = false
		}
		if !inKeys || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		key := strings.TrimSpace(cells[1])
		if key == "Key" || strings.Trim(key, " -") == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		t.Fatal("README key table was not found")
	}
	return keys
}

func splitReadmeKeyCell(cell string) []string {
	cell = strings.ReplaceAll(cell, "`", "")
	parts := strings.FieldsFunc(cell, func(r rune) bool {
		return r == ',' || r == '/'
	})
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func normalizeReadmeKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.Join(strings.Fields(key), " ")
	switch key {
	case "arrow keys":
		return "arrows"
	case "ctrl+c":
		return "ctrl+c"
	}
	return key
}

func assertQuitCommand(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected quit command, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("command returned %T, want tea.QuitMsg", msg)
	}
}

func headingLine(outline []md.Heading, text string) int {
	for _, heading := range outline {
		if heading.Text == text {
			return heading.Line
		}
	}
	return 0
}

func linkByURL(t *testing.T, links []md.Link, url string) md.Link {
	t.Helper()
	for _, link := range links {
		if link.URL == url {
			return link
		}
	}
	t.Fatalf("missing link %s", url)
	return md.Link{}
}

func clickXForLink(t *testing.T, model Model, link md.Link) int {
	t.Helper()
	line := model.lineForLink(link)
	if line < 0 || line >= len(model.renderedLines) {
		t.Fatalf("link line %d out of range", line)
	}
	_, start := focusedLinkTarget(md.StripANSI(model.renderedLines[line]), link)
	if start < 0 {
		t.Fatalf("missing rendered target for link %#v on line %q", link, md.StripANSI(model.renderedLines[line]))
	}
	return start
}

func linkIndexByURL(t *testing.T, links []md.Link, url string) int {
	t.Helper()
	for i, link := range links {
		if link.URL == url {
			return i
		}
	}
	t.Fatalf("missing link %s", url)
	return -1
}

func TestUpdateOutlineMouseSelectsClickedHeading(t *testing.T) {
	model := New(md.Document{
		Outline: []md.Heading{
			{Level: 1, Text: "Root", Line: 1, Parent: -1, Children: []int{1}},
			{Level: 2, Text: "Child", Line: 2, Parent: 0},
		},
		Rendered: "Root\nChild\n",
		Raw:      "# Root\n## Child\n",
	})
	model.outlineVisible = true
	model.outline.Width = 30
	model.outline.Height = 10
	model.body.Width = 50
	model.body.Height = 10
	model.rebuildContent()

	next, _ := model.Update(tea.MouseMsg(tea.MouseEvent{
		X:      52,
		Y:      1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}))
	got := next.(Model)

	if got.selectedOutline != 1 {
		t.Fatalf("selected outline = %d, want 1", got.selectedOutline)
	}
	if !strings.Contains(got.status, "Child") {
		t.Fatalf("status = %q, want Child", got.status)
	}
}

func BenchmarkLinkAtRenderedPositionLargeDocument(b *testing.B) {
	const linkCount = 1000
	rawLines := make([]string, 0, linkCount)
	renderedLines := make([]string, 0, linkCount)
	links := make([]md.Link, 0, linkCount)
	for i := 0; i < linkCount; i++ {
		text := fmt.Sprintf("Link %03d", i)
		url := fmt.Sprintf("https://example.com/%03d", i)
		rawLines = append(rawLines, fmt.Sprintf("[%s](%s)", text, url))
		renderedLines = append(renderedLines, text)
		links = append(links, md.Link{Text: text, URL: url, Line: i + 1})
	}

	model := New(md.Document{
		Links:    links,
		Rendered: strings.Join(renderedLines, "\n") + "\n",
		Raw:      strings.Join(rawLines, "\n") + "\n",
	})
	line := linkCount - 1
	x := strings.Index(model.renderedLines[line], "Link")
	rawLine := model.rawLineForRendered(line)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := model.linkAtRenderedPosition(line, x, rawLine); !ok {
			b.Fatal("missing link")
		}
	}
}

func BenchmarkRenderContentLargeDocument(b *testing.B) {
	model := newLargeInteractionModel(1200)
	model.selectedTask = len(model.tasks) - 1
	model.selectedLink = len(model.doc.Links) - 1
	model.searchQuery = "Task"
	model.searchMatches = FindMatches(model.contentLines, model.searchQuery)
	model.selectedMatch = len(model.searchMatches) - 1

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.renderContent()
	}
}

func BenchmarkFocusNextTaskLargeDocument(b *testing.B) {
	model := newLargeInteractionModel(1200)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.focusNextTask(1)
	}
}

func BenchmarkMoveOutlineLargeDocument(b *testing.B) {
	model := newLargeInteractionModel(1200)
	model.outlineVisible = true
	model.selectedOutline = 0

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.moveOutline(1)
	}
}

func BenchmarkToggleTaskLargeDocument(b *testing.B) {
	model := newLargeInteractionModel(300)
	model.selectedTask = 0

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.toggleTask(0, true)
	}
}

func newLargeInteractionModel(itemCount int) Model {
	rawLines := make([]string, 0, itemCount*2)
	renderedLines := make([]string, 0, itemCount*2)
	outline := make([]md.Heading, 0, itemCount)
	links := make([]md.Link, 0, itemCount*2)

	for i := 0; i < itemCount; i++ {
		headingLine := len(rawLines) + 1
		headingText := fmt.Sprintf("Heading %04d", i)
		rawLines = append(rawLines, "# "+headingText)
		renderedLines = append(renderedLines, headingText)
		outline = append(outline, md.Heading{Level: 1, Text: headingText, Line: headingLine, Parent: -1})

		linkText := fmt.Sprintf("Link %04d", i)
		linkURL := fmt.Sprintf("https://example.com/%04d", i)
		bareURL := fmt.Sprintf("https://example.org/%04d", i)
		taskLine := len(rawLines) + 1
		rawLines = append(rawLines, fmt.Sprintf("- [ ] Task %04d uses [%s](%s) and %s", i, linkText, linkURL, bareURL))
		renderedLines = append(renderedLines, fmt.Sprintf("[ ] Task %04d uses %s and %s", i, linkText, bareURL))
		links = append(links,
			md.Link{Text: linkText, URL: linkURL, Line: taskLine},
			md.Link{Text: bareURL, URL: bareURL, Line: taskLine},
		)
	}

	return New(md.Document{
		Outline:  outline,
		Links:    links,
		Rendered: strings.Join(renderedLines, "\n") + "\n",
		Raw:      strings.Join(rawLines, "\n") + "\n",
	})
}
