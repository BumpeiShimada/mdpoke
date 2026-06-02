package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	md "github.com/BumpeiShimada/mdpoke/internal/markdown"
)

const DefaultRenderWidth = 88

const (
	footerHeight    = 1
	outlineWidth    = 42
	bottomScrollPad = 20
	reloadInterval  = 500 * time.Millisecond
)

type mode int

const (
	modeNormal mode = iota
	modeSearch
	modeHelp
)

type modalKind int

const (
	modalNone modalKind = iota
	modalConfirmJump
	modalMessage
)

type SearchMatch struct {
	Line  int
	Start int
	End   int
}

type taskItem struct {
	Line    int
	Checked bool
	Text    string
}

type selectionPoint struct {
	Line   int
	Column int
}

type helpItem struct {
	Key         string
	Name        string
	Description string
}

type fileStamp struct {
	ModTime time.Time
	Size    int64
}

type fileWatchMsg struct{}

type Model struct {
	doc md.Document

	body    viewport.Model
	outline viewport.Model

	searchInput textinput.Model
	helpInput   textinput.Model

	mode            mode
	outlineVisible  bool
	selectedOutline int
	selectedLink    int
	selectedTask    int
	focusedJumpLine int
	focusedJumpText string
	focusedOutline  int
	outlineLineMap  []int
	outlineStartMap []int
	modalKind       modalKind
	modalTitle      string
	modalBody       string
	pendingJumpURL  string

	width  int
	height int
	ready  bool
	err    error
	status string
	stamp  fileStamp

	renderedLines []string
	contentLines  []string
	rawToRendered []int
	renderedToRaw []int
	tasks         []taskItem
	searchQuery   string
	searchMatches []SearchMatch
	selectedMatch int

	textSelectionStart    selectionPoint
	textSelectionEnd      selectionPoint
	textSelectionAnchor   selectionPoint
	textSelectionDragging bool
}

func New(doc md.Document) Model {
	body := viewport.New(DefaultRenderWidth, 24)
	body.MouseWheelEnabled = true
	body.MouseWheelDelta = 4

	outline := viewport.New(outlineWidth, 24)
	outline.MouseWheelEnabled = true
	outline.MouseWheelDelta = 3

	search := textinput.New()
	search.Prompt = "/"
	search.Placeholder = "search"
	search.CharLimit = 120
	search.Width = 40

	help := textinput.New()
	help.Prompt = "filter "
	help.Placeholder = "key, action, description"
	help.CharLimit = 120
	help.Width = 36

	m := Model{
		doc:             doc,
		body:            body,
		outline:         outline,
		searchInput:     search,
		helpInput:       help,
		width:           DefaultRenderWidth,
		height:          26,
		selectedOutline: -1,
		selectedLink:    -1,
		selectedTask:    -1,
		focusedJumpLine: -1,
		focusedJumpText: "",
		focusedOutline:  -1,
		selectedMatch:   -1,

		textSelectionStart:  invalidSelectionPoint(),
		textSelectionEnd:    invalidSelectionPoint(),
		textSelectionAnchor: invalidSelectionPoint(),
	}
	m.stamp = currentFileStamp(doc.Path)
	m.rebuildContent()
	return m
}

func (m Model) Init() tea.Cmd {
	return watchFileCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case fileWatchMsg:
		m.reloadIfFileChanged()
		return m, watchFileCmd()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resize()
		return m, nil

	case tea.KeyMsg:
		if m.modalKind != modalNone {
			return m.updateModal(msg)
		}
		if m.mode == modeSearch {
			return m.updateSearch(msg)
		}
		if m.mode == modeHelp {
			return m.updateHelp(msg)
		}
		return m.updateNormal(msg)

	case tea.MouseMsg:
		if m.modalKind != modalNone {
			m.updateModalMouse(msg)
			return m, nil
		}
		if m.textSelectionAnchor.valid() {
			if m.updateLineSelectionMouse(msg) {
				return m, nil
			}
		}
		if m.outlineVisible && m.mouseInOutline(msg) {
			return m.updateOutlineMouse(msg)
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if m.toggleTaskAtMouse(msg) {
				return m, nil
			}
			if m.beginLineSelection(msg) {
				return m, nil
			}
		}
		if isMouseRelease(msg) {
			m.clearLineSelection()
			m.focusLinkAtMouse(msg)
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown ||
			msg.Button == tea.MouseButtonWheelLeft || msg.Button == tea.MouseButtonWheelRight {
			m.body, cmd = m.body.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if m.outlineVisible {
		m.outline, cmd = m.outline.Update(msg)
	}
	m.body, cmd = m.body.Update(msg)
	return m, cmd
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.clearJumpFocus()
	m.clearOutlineFocus()

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
		m.helpInput.Reset()
		cmd := m.helpInput.Focus()
		return m, cmd
	case "/":
		m.mode = modeSearch
		m.searchInput.Reset()
		cmd := m.searchInput.Focus()
		return m, cmd
	case "o":
		m.toggleOutline()
		return m, nil
	case "esc":
		m.clearTransientHighlights()
		return m, nil
	case "tab":
		m.focusNextTask(1)
		return m, nil
	case "shift+tab":
		m.focusNextTask(-1)
		return m, nil
	case "enter":
		if m.toggleSelectedTask() {
			return m, nil
		}
		m.followSelection()
		return m, nil
	case "y":
		m.copySelection()
		return m, nil
	case " ":
		m.toggleSelectedTask()
		return m, nil
	case "n":
		m.moveSearchMatch(1)
		return m, nil
	case "N":
		m.moveSearchMatch(-1)
		return m, nil
	case "h", "left":
		if m.outlineVisible {
			m.outlineVisible = false
			m.resize()
		}
		return m, nil
	case "l", "right":
		if !m.outlineVisible {
			m.outlineVisible = true
			m.selectCurrentOutline()
			m.resize()
		}
		return m, nil
	case "j", "down":
		if m.outlineVisible {
			m.moveOutline(1)
		} else {
			m.body.ScrollDown(1)
		}
		return m, nil
	case "k", "up":
		if m.outlineVisible {
			m.moveOutline(-1)
		} else {
			m.body.ScrollUp(1)
		}
		return m, nil
	case "g":
		m.body.GotoTop()
		m.selectCurrentOutline()
		return m, nil
	case "G":
		m.body.GotoBottom()
		m.selectCurrentOutline()
		return m, nil
	case "pgdown", "ctrl+f":
		m.body.PageDown()
		m.selectCurrentOutline()
		return m, nil
	case "pgup", "ctrl+b":
		m.body.PageUp()
		m.selectCurrentOutline()
		return m, nil
	}

	var cmd tea.Cmd
	if m.outlineVisible {
		m.outline, cmd = m.outline.Update(msg)
	}
	m.body, cmd = m.body.Update(msg)
	return m, cmd
}

func (m Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" || msg.String() == "q" {
		return m, tea.Quit
	}

	switch m.modalKind {
	case modalConfirmJump:
		switch msg.String() {
		case "y", "Y":
			url := m.pendingJumpURL
			m.dismissModal()
			m.followMarkdownLink(url)
			return m, nil
		case "n", "N", "esc":
			m.dismissModal()
			return m, nil
		}
	case modalMessage:
		m.dismissModal()
		return m, nil
	}
	return m, nil
}

func (m *Model) updateModalMouse(msg tea.MouseMsg) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return
	}
	if !m.mouseInModal(msg) {
		m.dismissModal()
		if m.clearLineSelection() {
			m.rebuildContent()
		}
		return
	}
	if m.modalKind != modalConfirmJump {
		return
	}
	left, right, y, ok := m.modalConfirmBounds()
	if ok && msg.Y == y && msg.X >= left && msg.X < right {
		url := m.pendingJumpURL
		m.dismissModal()
		m.followMarkdownLink(url)
		return
	}
	left, right, y, ok = m.modalCancelBounds()
	if ok && msg.Y == y && msg.X >= left && msg.X < right {
		m.dismissModal()
	}
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = modeNormal
		m.searchInput.Blur()
		return m, nil
	case "enter":
		m.mode = modeNormal
		m.searchInput.Blur()
		m.applySearch(m.searchInput.Value())
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "?":
		m.mode = modeNormal
		m.helpInput.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.helpInput, cmd = m.helpInput.Update(msg)
	return m, cmd
}

func (m *Model) resize() {
	topRaw := m.rawLineForRendered(m.body.YOffset)
	m.clearLineSelection()

	contentHeight := max(1, m.height-footerHeight)
	contentWidth := max(20, m.width)
	if m.outlineVisible {
		contentWidth = max(20, m.width-outlineWidth-1)
	}

	renderedDoc, err := m.doc.Render(max(20, contentWidth-2))
	if err != nil {
		m.err = err
		return
	}

	m.doc = renderedDoc
	m.body.Width = contentWidth
	m.body.Height = contentHeight
	m.outline.Width = min(outlineWidth, max(20, m.width/2))
	m.outline.Height = contentHeight
	m.rebuildContent()
	m.body.SetYOffset(clamp(m.lineForRaw(topRaw), 0, max(0, len(m.renderedLines)-1)))
}

func watchFileCmd() tea.Cmd {
	return tea.Tick(reloadInterval, func(time.Time) tea.Msg {
		return fileWatchMsg{}
	})
}

func currentFileStamp(path string) fileStamp {
	if strings.TrimSpace(path) == "" {
		return fileStamp{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{ModTime: info.ModTime(), Size: info.Size()}
}

func (s fileStamp) changed(next fileStamp) bool {
	if s.ModTime.IsZero() && next.ModTime.IsZero() {
		return false
	}
	return !s.ModTime.Equal(next.ModTime) || s.Size != next.Size
}

func (m *Model) reloadIfFileChanged() {
	if strings.TrimSpace(m.doc.Path) == "" {
		return
	}
	nextStamp := currentFileStamp(m.doc.Path)
	if !m.stamp.changed(nextStamp) {
		return
	}
	m.stamp = nextStamp
	if nextStamp.ModTime.IsZero() {
		m.status = "file unavailable"
		return
	}
	m.reloadDocumentFromDisk("reloaded")
}

func (m *Model) reloadDocumentFromDisk(status string) {
	width := m.renderWidth()
	doc, err := md.Load(m.doc.Path, width)
	if err != nil {
		m.err = err
		m.status = fmt.Sprintf("reload failed: %v", err)
		return
	}
	m.doc = doc
	m.err = nil
	m.stamp = currentFileStamp(m.doc.Path)
	m.selectedLink = -1
	m.selectedTask = -1
	m.focusedJumpLine = -1
	m.focusedJumpText = ""
	m.focusedOutline = -1
	m.rebuildContent()
	m.body.SetYOffset(clamp(m.body.YOffset, 0, max(0, len(m.renderedLines)-1)))
	m.status = status
}

func (m Model) renderWidth() int {
	width := m.body.Width
	if width <= 0 {
		width = DefaultRenderWidth
	}
	return max(20, width-2)
}

func (m *Model) rebuildContent() {
	rendered := strings.TrimRight(m.doc.Rendered, "\n")
	plain := strings.TrimRight(md.StripANSI(m.doc.Rendered), "\n")
	if rendered == "" {
		rendered = "(empty document)"
	}
	if plain == "" {
		plain = rendered
	}
	m.renderedLines = strings.Split(rendered, "\n")
	m.contentLines = strings.Split(plain, "\n")
	m.rawToRendered, m.renderedToRaw = buildLineMaps(m.doc.Raw, m.contentLines)
	m.tasks = parseTaskItems(m.doc.Raw)
	if m.selectedTask >= len(m.tasks) {
		m.selectedTask = -1
	}
	m.body.SetContent(m.renderContent())
	m.outline.SetContent(m.renderOutline())
}

func (m Model) renderContent() string {
	lines := make([]string, len(m.renderedLines))
	copy(lines, m.renderedLines)

	m.styleBareAutoLinks(lines)

	matchesByLine := make(map[int][]int)
	for i, match := range m.searchMatches {
		matchesByLine[match.Line] = append(matchesByLine[match.Line], i)
	}
	for lineIndex, indexes := range matchesByLine {
		if lineIndex < 0 || lineIndex >= len(lines) {
			continue
		}
		for i := len(indexes) - 1; i >= 0; i-- {
			matchIndex := indexes[i]
			match := m.searchMatches[matchIndex]
			style := searchMatchStyle
			if matchIndex == m.selectedMatch {
				style = activeSearchMatchStyle
			}
			lines[lineIndex] = styleANSIVisibleRange(lines[lineIndex], match.Start, match.End, style)
		}
	}

	if m.selectedLink >= 0 && m.selectedLink < len(m.doc.Links) {
		link := m.doc.Links[m.selectedLink]
		line := m.lineForLink(link)
		if line >= 0 && line < len(lines) {
			lines[line] = m.styleFocusedLink(lines[line], link)
		}
	}

	if m.selectedTask >= 0 && m.selectedTask < len(m.tasks) {
		task := m.tasks[m.selectedTask]
		line := m.lineForTask(task)
		if line >= 0 && line < len(lines) {
			lines[line] = styleFocusedTaskCheckbox(lines[line])
		}
	}

	if m.focusedJumpLine > 0 {
		line := m.lineForRaw(m.focusedJumpLine)
		if line >= 0 && line < len(lines) {
			lines[line] = m.styleFocusedJump(lines[line])
		}
	}

	if m.focusedOutline >= 0 && m.focusedOutline < len(m.doc.Outline) {
		heading := m.doc.Outline[m.focusedOutline]
		line := m.lineForRaw(heading.Line)
		if line >= 0 && line < len(lines) {
			lines[line] = styleFocusedOutlineHeading(lines[line], heading, m.body.Width)
		}
	}

	if start, end, ok := m.textSelectionRange(); ok {
		for line := start.Line; line <= end.Line && line < len(lines); line++ {
			startColumn, endColumn, ok := m.textSelectionColumnsForLine(line, start, end)
			if !ok {
				continue
			}
			lines[line] = styleDisplayRangePlain(lines[line], startColumn, endColumn, lineSelectionStyle)
		}
	}

	return strings.Join(lines, "\n") + strings.Repeat("\n", bottomScrollPad)
}

func (m *Model) renderOutline() string {
	if len(m.doc.Outline) == 0 {
		m.outlineLineMap = nil
		m.outlineStartMap = nil
		return mutedStyle.Render(" no headings")
	}

	lines := make([]string, 0, len(m.doc.Outline))
	m.outlineLineMap = make([]int, 0, len(m.doc.Outline))
	m.outlineStartMap = make([]int, len(m.doc.Outline))
	for i, heading := range m.doc.Outline {
		prefix := m.outlinePrefix(i)
		line := fmt.Sprintf("%s%s", prefix, heading.Text)
		wrapped := wrapOutlineLine(line, prefix, max(1, m.outline.Width))
		m.outlineStartMap[i] = len(m.outlineLineMap)
		for _, wrappedLine := range wrapped {
			rendered := outlineHeadingStyle(heading.Level, i == m.selectedOutline).Width(m.outline.Width).Render(wrappedLine)
			lines = append(lines, rendered)
			m.outlineLineMap = append(m.outlineLineMap, i)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) outlinePrefix(index int) string {
	if index < 0 || index >= len(m.doc.Outline) {
		return ""
	}

	heading := m.doc.Outline[index]
	return strings.Repeat("  ", max(0, heading.Level-1))
}

func (m Model) outlineLineForHeading(index int) int {
	if index < 0 || index >= len(m.outlineStartMap) {
		return 0
	}
	return m.outlineStartMap[index]
}

var outlineHeadingStyle = func(level int, selected bool) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true)
	switch level {
	case 1:
		style = style.Foreground(lipgloss.Color("235")).Background(lipgloss.Color("111"))
	case 2:
		style = style.Foreground(lipgloss.Color("229"))
	case 3:
		style = style.Foreground(lipgloss.Color("116"))
	case 4:
		style = style.Foreground(lipgloss.Color("183"))
	case 5:
		style = style.Foreground(lipgloss.Color("151"))
	case 6:
		style = style.Foreground(lipgloss.Color("215"))
	default:
		style = style.Foreground(lipgloss.Color("250"))
	}
	if selected && level != 1 {
		style = style.Background(lipgloss.Color("238"))
	}
	return style
}

func styleFocusedOutlineHeading(line string, heading md.Heading, width int) string {
	plain := md.StripANSI(line)
	style := outlineHeadingStyle(heading.Level, true)
	if width > 0 {
		style = style.Width(width)
	}
	return style.Render(plain)
}

func wrapOutlineLine(line, prefix string, width int) []string {
	first := wrapDisplayHard(line, width)
	if len(first) <= 1 || prefix == "" {
		return first
	}

	out := make([]string, 0, len(first))
	out = append(out, first[0])
	continuationWidth := max(1, width-lipgloss.Width(prefix))
	for _, segment := range first[1:] {
		for _, wrapped := range wrapDisplayHard(segment, continuationWidth) {
			out = append(out, prefix+wrapped)
		}
	}
	return out
}

func (m Model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("render error: %v", m.err))
	}
	if !m.ready {
		return "loading..."
	}

	body := m.body.View()
	if m.outlineVisible {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, dividerStyle.Height(m.body.Height).Render(""), m.outline.View())
	}

	view := lipgloss.JoinVertical(lipgloss.Left, body, m.footerView())
	if m.modalKind != modalNone {
		return m.modalOverlay(view)
	}
	if m.mode == modeHelp {
		return m.helpOverlay()
	}
	return view
}

func (m Model) footerView() string {
	file := filepath.Base(m.doc.Path)
	if file == "." || file == string(filepath.Separator) {
		file = m.doc.Path
	}

	leftText := fmt.Sprintf(" %s  %s ", file, m.positionLabel())
	if m.status != "" {
		leftText += m.status + " "
	}
	if m.mode == modeSearch {
		leftText = " " + m.searchInput.View()
	}

	left := footerStyle.Render(leftText)
	right := guideStyle.Render(" " + strings.Join(m.guideItems(), "  ") + " ")
	padding := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	line := left + strings.Repeat(" ", padding) + right
	return lipgloss.NewStyle().Width(m.width).Render(line)
}

func (m Model) positionLabel() string {
	if len(m.searchMatches) > 0 && m.selectedMatch >= 0 {
		return fmt.Sprintf("%3.0f%%  match %d/%d", m.body.ScrollPercent()*100, m.selectedMatch+1, len(m.searchMatches))
	}
	return fmt.Sprintf("%3.0f%%", m.body.ScrollPercent()*100)
}

func (m Model) guideItems() []string {
	items := []string{"o outline", "/ search", "? help", "q quit"}
	if m.hasLineSelection() {
		return []string{"y copy text", "esc clear", "? help"}
	}
	if len(m.tasks) > 0 {
		items = append([]string{"tab checkbox"}, items...)
	}
	if m.selectedTask >= 0 {
		items = []string{"space toggle", "enter toggle", "tab next box", "esc exit box"}
	}
	if m.selectedLink >= 0 {
		items = []string{"y copy", "tab next url", "esc exit url"}
		if m.focusedLinkCanJump() {
			items = append([]string{"enter jump"}, items...)
		}
	}
	if len(m.searchMatches) > 0 {
		items = append([]string{"n next", "N prev"}, items...)
	}
	if m.outlineVisible && m.selectedLink < 0 {
		items = []string{"j/k move", "o close", "? help"}
	}
	if m.mode == modeSearch {
		items = []string{"enter search", "esc cancel"}
	}
	return items
}

func (m Model) helpOverlay() string {
	filter := strings.ToLower(strings.TrimSpace(m.helpInput.Value()))
	lines := []string{titleStyle.Render("mdpoke keys"), "", m.helpInput.View(), ""}
	for _, item := range helpItems() {
		haystack := strings.ToLower(item.Key + " " + item.Name + " " + item.Description)
		if filter != "" && !strings.Contains(haystack, filter) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%-14s %-14s %s", keyStyle.Render(item.Key), item.Name, mutedStyle.Render(item.Description)))
	}
	if len(lines) == 4 {
		lines = append(lines, mutedStyle.Render("no matching keys"))
	}

	box := helpBoxStyle.Width(min(72, max(34, m.width-8))).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box, lipgloss.WithWhitespaceChars(" ")) + "\n"
}

func (m Model) modalOverlay(base string) string {
	boxWidth := m.modalBoxWidth()
	contentWidth := max(12, boxWidth-4)
	title := titleStyle.Render(m.modalTitle)
	topRuleWidth := max(0, boxWidth-2-lipgloss.Width(md.StripANSI(title)))
	topBorder := "╭" + title + strings.Repeat("─", topRuleWidth) + "╮"
	bottom := "╰" + strings.Repeat("─", max(0, boxWidth-2)) + "╯"

	bodyLines := strings.Split(m.modalBody, "\n")
	lines := make([]string, 0, len(bodyLines)+2)
	lines = append(lines, topBorder)
	for i, line := range bodyLines {
		if m.modalKind == modalConfirmJump && i == len(bodyLines)-1 {
			line = confirmButtonStyle.Render("y confirm") + "   " + cancelButtonStyle.Render("n cancel")
		}
		for _, wrapped := range wrapANSIHard(line, contentWidth) {
			padded := padANSIToWidth(wrapped, contentWidth)
			lines = append(lines, "│ "+padded+" │")
		}
	}
	lines = append(lines, bottom)

	box := strings.Join(lines, "\n")
	left, top, _, _, ok := m.modalBoxBounds()
	if !ok {
		return base
	}
	return overlayBoxAt(base, box, left, top)
}

func padANSIToWidth(line string, width int) string {
	padding := max(0, width-lipgloss.Width(md.StripANSI(line)))
	return line + strings.Repeat(" ", padding)
}

func overlayBoxAt(base, box string, left, top int) string {
	baseLines := strings.Split(strings.TrimSuffix(base, "\n"), "\n")
	boxLines := strings.Split(strings.TrimSuffix(box, "\n"), "\n")
	for i, boxLine := range boxLines {
		target := top + i
		for target >= len(baseLines) {
			baseLines = append(baseLines, "")
		}
		if strings.TrimSpace(boxLine) == "" {
			continue
		}
		baseLines[target] = replaceVisibleColumns(baseLines[target], left, left+lipgloss.Width(boxLine), boxLine)
	}
	return strings.Join(baseLines, "\n")
}

func replaceVisibleColumns(line string, start, end int, replacement string) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	prefixByte := ansiVisibleColumnByteIndex(line, start)
	suffixByte := ansiVisibleColumnByteIndex(line, end)
	padding := max(0, start-lipgloss.Width(md.StripANSI(line)))
	activeSuffixStyle := activeSGRAt(line, suffixByte)
	return line[:prefixByte] + "\x1b[0m" + strings.Repeat(" ", padding) + replacement + "\x1b[0m" + activeSuffixStyle + line[suffixByte:]
}

func (m Model) modalBoxBounds() (int, int, int, int, bool) {
	if m.modalKind == modalNone {
		return 0, 0, 0, 0, false
	}
	boxWidth := m.modalBoxWidth()
	boxHeight := m.modalHeight()
	boxLeft := max(0, (m.width-boxWidth)/2)
	boxTop := max(0, (m.height-boxHeight)/2)
	return boxLeft, boxTop, boxWidth, boxHeight, true
}

func (m Model) modalBoxWidth() int {
	return min(66, max(34, m.width-10))
}

func (m Model) mouseInModal(msg tea.MouseMsg) bool {
	left, top, width, height, ok := m.modalBoxBounds()
	if !ok {
		return false
	}
	return msg.X >= left && msg.X < left+width && msg.Y >= top && msg.Y < top+height
}

func (m Model) modalConfirmBounds() (int, int, int, bool) {
	return m.modalButtonBounds("y confirm")
}

func (m Model) modalCancelBounds() (int, int, int, bool) {
	if m.modalKind != modalConfirmJump {
		return 0, 0, 0, false
	}
	confirmWidth := lipgloss.Width("y confirm")
	left, _, y, ok := m.modalButtonBounds("n cancel")
	if !ok {
		return 0, 0, 0, false
	}
	left += confirmWidth + 3
	right := left + lipgloss.Width("n cancel")
	return left, right, y, true
}

func (m Model) modalButtonBounds(label string) (int, int, int, bool) {
	if m.modalKind != modalConfirmJump {
		return 0, 0, 0, false
	}
	boxLeft, boxTop, _, _, ok := m.modalBoxBounds()
	if !ok {
		return 0, 0, 0, false
	}
	y := boxTop + m.modalHeight() - 2
	left := boxLeft + 2
	right := left + lipgloss.Width(label)
	return left, right, y, true
}

func (m Model) modalHeight() int {
	if m.modalKind == modalNone {
		return 0
	}
	contentWidth := max(12, m.modalBoxWidth()-4)
	bodyLines := strings.Split(m.modalBody, "\n")
	height := 2
	for i, line := range bodyLines {
		if m.modalKind == modalConfirmJump && i == len(bodyLines)-1 {
			line = "y confirm   n cancel"
		}
		height += len(wrapANSIHard(line, contentWidth))
	}
	return height
}

func (m *Model) toggleOutline() {
	m.outlineVisible = !m.outlineVisible
	if m.outlineVisible {
		m.selectCurrentOutline()
	}
	m.resize()
}

func (m *Model) moveOutline(delta int) {
	if len(m.doc.Outline) == 0 {
		return
	}
	if m.selectedOutline < 0 {
		m.selectCurrentOutline()
	}
	m.selectedOutline = clamp(m.selectedOutline+delta, 0, len(m.doc.Outline)-1)
	m.outline.SetYOffset(clamp(m.outlineLineForHeading(m.selectedOutline)-2, 0, max(0, len(m.outlineLineMap)-1)))
	m.focusOutlineHeading(m.selectedOutline)
	m.outline.SetContent(m.renderOutline())
}

func (m *Model) selectCurrentOutline() {
	m.selectedOutline = m.currentHeadingIndex()
	m.outline.SetContent(m.renderOutline())
}

func (m *Model) focusOutlineHeading(index int) {
	if index < 0 || index >= len(m.doc.Outline) {
		return
	}
	m.focusedOutline = index
	m.jumpToRenderedLineNearTop(m.lineForRaw(m.doc.Outline[index].Line), 6)
	m.rebuildContent()
}

func (m Model) currentHeadingIndex() int {
	raw := m.rawLineForRendered(m.body.YOffset)
	current := -1
	for i, heading := range m.doc.Outline {
		if heading.Line <= raw {
			current = i
		}
	}
	return current
}

func (m *Model) applySearch(query string) {
	query = strings.TrimSpace(query)
	m.searchQuery = query
	m.searchMatches = FindMatches(m.contentLines, query)
	m.selectedMatch = -1
	if query == "" {
		m.status = ""
		m.rebuildContent()
		return
	}
	if len(m.searchMatches) == 0 {
		m.status = fmt.Sprintf("no matches for %q", query)
		m.rebuildContent()
		return
	}
	m.selectedMatch = 0
	m.status = fmt.Sprintf("%d matches", len(m.searchMatches))
	m.jumpToRenderedLine(m.searchMatches[0].Line, true)
	m.rebuildContent()
}

func (m *Model) moveSearchMatch(delta int) {
	if len(m.searchMatches) == 0 {
		return
	}
	if m.selectedMatch < 0 {
		m.selectedMatch = 0
	} else {
		m.selectedMatch = (m.selectedMatch + delta + len(m.searchMatches)) % len(m.searchMatches)
	}
	m.jumpToRenderedLine(m.searchMatches[m.selectedMatch].Line, true)
	m.rebuildContent()
}

func FindMatches(lines []string, query string) []SearchMatch {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	matches := make([]SearchMatch, 0)
	for lineIndex, line := range lines {
		lower := strings.ToLower(line)
		offset := 0
		for {
			found := strings.Index(lower[offset:], query)
			if found < 0 {
				break
			}
			startByte := offset + found
			endByte := startByte + len(query)
			start := lipgloss.Width(line[:startByte])
			end := start + lipgloss.Width(line[startByte:endByte])
			matches = append(matches, SearchMatch{Line: lineIndex, Start: start, End: end})
			offset = endByte
		}
	}
	return matches
}

func (m *Model) focusNextTask(delta int) {
	hadSelection := m.clearLineSelection()
	m.selectedLink = -1

	if len(m.tasks) == 0 {
		m.showMessage("No Checkboxes", fmt.Sprintf("No checkboxes in this document.\n\n%s", mutedStyle.Render("press any key to close")))
		if hadSelection {
			m.rebuildContent()
		}
		return
	}

	if m.selectedTask < 0 {
		if delta < 0 {
			m.selectedTask = len(m.tasks) - 1
		} else {
			m.selectedTask = 0
		}
	} else {
		m.selectedTask = (m.selectedTask + delta + len(m.tasks)) % len(m.tasks)
	}

	task := m.tasks[m.selectedTask]
	m.status = m.taskStatus(task)
	m.jumpToRenderedLine(m.lineForTask(task), false)
	m.rebuildContent()
}

func (m *Model) toggleSelectedTask() bool {
	if m.selectedTask < 0 || m.selectedTask >= len(m.tasks) {
		return false
	}
	m.toggleTask(m.selectedTask)
	return true
}

func (m *Model) toggleTaskAtMouse(msg tea.MouseMsg) bool {
	index, ok := m.taskAtMouse(msg)
	if !ok {
		return false
	}
	m.selectedLink = -1
	m.selectedTask = index
	m.toggleTask(index)
	return true
}

func (m Model) taskAtMouse(msg tea.MouseMsg) (int, bool) {
	if len(m.tasks) == 0 || !m.mouseInBody(msg) {
		return -1, false
	}
	renderedLine := m.body.YOffset + msg.Y
	rawLine := m.rawLineForRendered(renderedLine)
	for i, task := range m.tasks {
		if task.Line != rawLine && m.lineForTask(task) != renderedLine {
			continue
		}
		start, end, ok := taskCheckboxColumns(m.contentLines[renderedLine])
		if ok && msg.X >= start && msg.X < end {
			return i, true
		}
	}
	return -1, false
}

func (m *Model) toggleTask(index int) {
	if index < 0 || index >= len(m.tasks) {
		return
	}
	task := m.tasks[index]
	lines := strings.Split(m.doc.Raw, "\n")
	lineIndex := task.Line - 1
	if lineIndex < 0 || lineIndex >= len(lines) {
		m.status = "checkbox line not found"
		return
	}

	nextLine, ok := toggleTaskLine(lines[lineIndex])
	if !ok {
		m.status = "checkbox line not found"
		return
	}
	lines[lineIndex] = nextLine
	nextRaw := strings.Join(lines, "\n")
	if strings.TrimSpace(m.doc.Path) != "" {
		if err := os.WriteFile(m.doc.Path, []byte(nextRaw), 0644); err != nil {
			m.showMessage("Update Failed", fmt.Sprintf("%v\n\n%s", err, mutedStyle.Render("press any key to close")))
			m.status = fmt.Sprintf("checkbox update failed: %v", err)
			return
		}
	}

	outline, links := md.ParseStructure([]byte(nextRaw))
	nextDoc := md.Document{
		Path:    m.doc.Path,
		Raw:     nextRaw,
		Outline: outline,
		Links:   links,
	}
	renderedDoc, err := nextDoc.Render(m.renderWidth())
	if err != nil {
		m.showMessage("Render Failed", fmt.Sprintf("%v\n\n%s", err, mutedStyle.Render("press any key to close")))
		m.status = fmt.Sprintf("render failed: %v", err)
		return
	}

	m.doc = renderedDoc
	m.stamp = currentFileStamp(m.doc.Path)
	m.rebuildContent()
	for i, nextTask := range m.tasks {
		if nextTask.Line == task.Line {
			m.selectedTask = i
			m.status = m.taskStatus(nextTask)
			m.jumpToRenderedLine(m.lineForTask(nextTask), false)
			m.rebuildContent()
			return
		}
	}
	m.selectedTask = -1
}

func (m Model) taskStatus(task taskItem) string {
	if task.Checked {
		return "checkbox: checked"
	}
	return "checkbox: unchecked"
}

func (m Model) lineForTask(task taskItem) int {
	preferred := m.lineForRaw(task.Line)
	if lineContainsTaskCheckbox(m.contentLines, preferred, task) {
		return preferred
	}

	start := max(0, preferred-4)
	stop := min(len(m.contentLines), preferred+5)
	for i := start; i < stop; i++ {
		if lineContainsTaskCheckbox(m.contentLines, i, task) {
			return i
		}
	}
	for i := range m.contentLines {
		if lineContainsTaskCheckbox(m.contentLines, i, task) {
			return i
		}
	}
	return preferred
}

func lineContainsTaskCheckbox(lines []string, index int, task taskItem) bool {
	if index < 0 || index >= len(lines) {
		return false
	}
	_, _, ok := taskCheckboxColumns(lines[index])
	return ok && (task.Text == "" || strings.Contains(lines[index], task.Text))
}

func (m *Model) focusNextLink(delta int) {
	hadSelection := m.clearLineSelection()
	m.selectedTask = -1

	indexes := m.focusableLinkIndexes()
	if len(indexes) == 0 {
		m.status = "no URL links"
		if hadSelection {
			m.rebuildContent()
		}
		return
	}

	current := -1
	for i, index := range indexes {
		if index == m.selectedLink {
			current = i
			break
		}
	}
	if current < 0 {
		if delta < 0 {
			current = len(indexes) - 1
		} else {
			current = 0
		}
	} else {
		current = (current + delta + len(indexes)) % len(indexes)
	}

	m.selectedLink = indexes[current]
	link := m.doc.Links[m.selectedLink]
	m.status = m.linkStatus(link)
	m.jumpToRenderedLine(m.lineForLink(link), false)
	m.rebuildContent()
}

func (m Model) focusableLinkIndexes() []int {
	indexes := make([]int, 0, len(m.doc.Links))
	for i, link := range m.doc.Links {
		if m.isInternalJumpLink(link) {
			continue
		}
		indexes = append(indexes, i)
	}
	return indexes
}

func (m *Model) focusLinkAtMouse(msg tea.MouseMsg) {
	if len(m.doc.Links) == 0 || !isBodyMouseLine(msg.Y, m.body.Height) {
		return
	}
	if m.clearLineSelection() {
		m.rebuildContent()
	}

	renderedLine := m.body.YOffset + msg.Y
	raw := m.rawLineForRendered(renderedLine)
	link, ok := m.linkAtRenderedPosition(renderedLine, msg.X, raw)
	if !ok {
		return
	}
	if m.isInternalJumpLink(link) {
		m.selectedLink = -1
		m.openJumpConfirm(link)
		return
	}
	m.selectedLink = -1
	m.copyClickedURL(link)
}

func (m Model) updateOutlineMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown, tea.MouseButtonWheelLeft, tea.MouseButtonWheelRight:
		m.outline, cmd = m.outline.Update(msg)
		return m, cmd
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	index := m.outline.YOffset + msg.Y
	if index < 0 || index >= len(m.outlineLineMap) {
		return m, nil
	}
	headingIndex := m.outlineLineMap[index]
	if headingIndex < 0 || headingIndex >= len(m.doc.Outline) {
		return m, nil
	}
	m.selectedOutline = headingIndex
	m.focusOutlineHeading(headingIndex)
	m.outline.SetContent(m.renderOutline())
	m.status = fmt.Sprintf("jumped to %s", m.doc.Outline[headingIndex].Text)
	return m, nil
}

func (m Model) mouseInOutline(msg tea.MouseMsg) bool {
	if !m.outlineVisible || msg.Y < 0 || msg.Y >= m.outline.Height {
		return false
	}
	return msg.X > m.body.Width
}

func isBodyMouseLine(y, bodyHeight int) bool {
	return y >= 0 && y < bodyHeight
}

func isMouseRelease(msg tea.MouseMsg) bool {
	return msg.Action == tea.MouseActionRelease && (msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonNone)
}

func invalidSelectionPoint() selectionPoint {
	return selectionPoint{Line: -1, Column: -1}
}

func (p selectionPoint) valid() bool {
	return p.Line >= 0 && p.Column >= 0
}

func (m Model) mouseInBody(msg tea.MouseMsg) bool {
	if !isBodyMouseLine(msg.Y, m.body.Height) {
		return false
	}
	return msg.X >= 0 && msg.X < m.body.Width
}

func (m Model) renderedPositionAtMouse(msg tea.MouseMsg) (selectionPoint, bool) {
	if !m.mouseInBody(msg) || len(m.contentLines) == 0 {
		return invalidSelectionPoint(), false
	}
	line := clamp(m.body.YOffset+msg.Y, 0, len(m.contentLines)-1)
	column := clamp(msg.X, 0, lipgloss.Width(m.contentLines[line]))
	if column < selectionLineStartColumn(m.contentLines[line]) {
		column = selectionLineStartColumn(m.contentLines[line])
	}
	return selectionPoint{Line: line, Column: column}, true
}

func (m *Model) beginLineSelection(msg tea.MouseMsg) bool {
	point, ok := m.renderedPositionAtMouse(msg)
	if !ok {
		return false
	}
	if m.clearLineSelection() {
		m.rebuildContent()
	}
	m.textSelectionAnchor = point
	m.textSelectionDragging = false
	return true
}

func (m *Model) updateLineSelectionMouse(msg tea.MouseMsg) bool {
	switch msg.Action {
	case tea.MouseActionMotion:
		point, ok := m.renderedPositionAtMouse(msg)
		if !ok {
			return true
		}
		m.textSelectionStart = m.textSelectionAnchor
		m.textSelectionEnd = point
		m.textSelectionDragging = true
		m.selectedLink = -1
		m.status = m.lineSelectionStatus()
		m.rebuildContent()
		return true
	case tea.MouseActionRelease:
		dragging := m.textSelectionDragging
		point, ok := m.renderedPositionAtMouse(msg)
		if dragging && ok {
			m.textSelectionStart = m.textSelectionAnchor
			m.textSelectionEnd = point
			m.status = m.lineSelectionStatus()
			m.rebuildContent()
		}
		m.textSelectionAnchor = invalidSelectionPoint()
		m.textSelectionDragging = false
		if dragging {
			return true
		}
		return false
	default:
		return true
	}
}

func (m Model) hasLineSelection() bool {
	_, _, ok := m.textSelectionRange()
	return ok
}

func (m Model) textSelectionRange() (selectionPoint, selectionPoint, bool) {
	if !m.textSelectionStart.valid() || !m.textSelectionEnd.valid() || len(m.contentLines) == 0 {
		return invalidSelectionPoint(), invalidSelectionPoint(), false
	}
	start := m.clampSelectionPoint(m.textSelectionStart)
	end := m.clampSelectionPoint(m.textSelectionEnd)
	if selectionPointAfter(start, end) {
		start, end = end, start
	}
	if start == end {
		return invalidSelectionPoint(), invalidSelectionPoint(), false
	}
	return start, end, true
}

func (m Model) clampSelectionPoint(point selectionPoint) selectionPoint {
	line := clamp(point.Line, 0, len(m.contentLines)-1)
	startColumn := selectionLineStartColumn(m.contentLines[line])
	return selectionPoint{
		Line:   line,
		Column: clamp(point.Column, startColumn, lipgloss.Width(m.contentLines[line])),
	}
}

func selectionPointAfter(left, right selectionPoint) bool {
	return left.Line > right.Line || (left.Line == right.Line && left.Column > right.Column)
}

func (m Model) textSelectionColumnsForLine(line int, start, end selectionPoint) (int, int, bool) {
	if line < start.Line || line > end.Line || line < 0 || line >= len(m.contentLines) {
		return 0, 0, false
	}
	lineWidth := lipgloss.Width(m.contentLines[line])
	lineStart := selectionLineStartColumn(m.contentLines[line])
	startColumn := lineStart
	endColumn := lineWidth
	if line == start.Line {
		startColumn = start.Column
	}
	if line == end.Line {
		endColumn = end.Column
	}
	startColumn = clamp(startColumn, lineStart, lineWidth)
	endColumn = clamp(endColumn, 0, lineWidth)
	return startColumn, endColumn, endColumn > startColumn
}

func (m Model) selectedLineText() (string, int, bool) {
	start, end, ok := m.textSelectionRange()
	if !ok {
		return "", 0, false
	}
	lines := make([]string, 0, end.Line-start.Line+1)
	for line := start.Line; line <= end.Line; line++ {
		startColumn, endColumn, _ := m.textSelectionColumnsForLine(line, start, end)
		lines = append(lines, displayColumnSlice(m.contentLines[line], startColumn, endColumn))
	}
	text := strings.TrimSpace(strings.Join(lines, "\n"))
	if text == "" {
		return "", 0, false
	}
	return text, len(strings.Split(text, "\n")), true
}

func (m Model) lineSelectionStatus() string {
	text, _, ok := m.selectedLineText()
	if !ok {
		return ""
	}
	return fmt.Sprintf("selected %d %s", utf8.RuneCountInString(text), plural(utf8.RuneCountInString(text), "char"))
}

func (m *Model) clearLineSelection() bool {
	hadSelection := m.textSelectionStart.valid() ||
		m.textSelectionEnd.valid() ||
		m.textSelectionAnchor.valid() ||
		m.textSelectionDragging
	m.textSelectionStart = invalidSelectionPoint()
	m.textSelectionEnd = invalidSelectionPoint()
	m.textSelectionAnchor = invalidSelectionPoint()
	m.textSelectionDragging = false
	return hadSelection
}

func (m *Model) followSelection() {
	if m.selectedLink < 0 || m.selectedLink >= len(m.doc.Links) {
		return
	}
	link := m.doc.Links[m.selectedLink]
	if m.followMarkdownLink(link.URL) {
		return
	}
	m.status = "external link: press y to copy"
}

func (m Model) styleFocusedLink(line string, link md.Link) string {
	plain := md.StripANSI(line)
	target, start := focusedLinkTarget(plain, link)
	if start >= 0 {
		return styleANSIVisibleRange(line, start, start+lipgloss.Width(target), linkFocusStyle)
	}
	return line
}

func (m Model) styleBareAutoLinks(lines []string) {
	for _, link := range m.doc.Links {
		if link.Text != link.URL || !hasScheme(link.URL) {
			continue
		}
		for line := range lines {
			lines[line] = styleBareAutoLink(lines[line], strings.TrimSpace(link.URL))
		}
	}
}

func styleBareAutoLink(line, target string) string {
	if target == "" {
		return line
	}
	plain := md.StripANSI(line)
	offset := 0
	for offset <= len(plain) {
		segment, segmentStart, segmentEnd, ok := bestLinkSegmentInLine(plain[offset:], target)
		if !ok {
			break
		}
		startByte := offset + segmentStart
		start := lipgloss.Width(plain[:startByte])
		end := start + lipgloss.Width(segment)
		line = styleANSIVisibleRangePlain(line, start, end, bareAutoLinkStyle)
		offset += segmentEnd
	}
	return line
}

func styleFocusedTaskCheckbox(line string) string {
	plain := md.StripANSI(line)
	start, end, ok := taskCheckboxColumns(plain)
	if !ok {
		return line
	}
	return styleANSIVisibleRange(line, start, end, taskFocusStyle)
}

func taskCheckboxColumns(line string) (int, int, bool) {
	loc := renderedTaskCheckboxRE.FindStringIndex(line)
	if loc == nil {
		return 0, 0, false
	}
	return lipgloss.Width(line[:loc[0]]), lipgloss.Width(line[:loc[1]]), true
}

func (m *Model) openJumpConfirm(link md.Link) {
	m.modalKind = modalConfirmJump
	m.modalTitle = "Jump?"
	m.modalBody = fmt.Sprintf("Jump to %s?\n\n%s", link.URL, mutedStyle.Render("y confirm   n cancel"))
	m.pendingJumpURL = link.URL
	m.status = ""
}

func (m *Model) copyClickedURL(link md.Link) {
	if err := clipboardWrite(link.URL); err != nil {
		m.modalKind = modalMessage
		m.modalTitle = "Copy Failed"
		m.modalBody = fmt.Sprintf("%v\n\n%s", err, mutedStyle.Render("press any key to close"))
		m.status = fmt.Sprintf("copy failed: %v", err)
		return
	}
	m.showCopiedMessage(fmt.Sprintf("copied: %s", link.URL))
}

func (m *Model) dismissModal() {
	m.modalKind = modalNone
	m.modalTitle = ""
	m.modalBody = ""
	m.pendingJumpURL = ""
}

func (m Model) lineForLink(link md.Link) int {
	preferred := m.lineForRaw(link.Line)
	if lineContainsFocusedLinkTarget(m.renderedLines, preferred, link) {
		return preferred
	}

	start := max(0, preferred-4)
	stop := min(len(m.renderedLines), preferred+5)
	for i := start; i < stop; i++ {
		if lineContainsFocusedLinkTarget(m.renderedLines, i, link) {
			return i
		}
	}
	for i := range m.renderedLines {
		if lineContainsFocusedLinkTarget(m.renderedLines, i, link) {
			return i
		}
	}
	return preferred
}

func (m Model) linkAtRenderedPosition(renderedLine, x, rawLine int) (md.Link, bool) {
	if renderedLine < 0 || renderedLine >= len(m.renderedLines) {
		return md.Link{}, false
	}

	plain := md.StripANSI(m.renderedLines[renderedLine])
	candidates := make([]md.Link, 0)
	for _, link := range m.doc.Links {
		if link.Line == rawLine {
			candidates = append(candidates, link)
		}
	}
	for _, link := range m.doc.Links {
		if link.Line == rawLine {
			continue
		}
		if _, _, _, ok := clickedLinkTarget(plain, link, x); ok {
			candidates = append(candidates, link)
		}
	}

	var selected md.Link
	selectedWidth := -1
	for _, link := range candidates {
		_, start, end, ok := clickedLinkTarget(plain, link, x)
		if !ok {
			continue
		}
		width := end - start
		if width > selectedWidth {
			selected = link
			selectedWidth = width
		}
	}
	return selected, selectedWidth >= 0
}

func lineContainsFocusedLinkTarget(lines []string, index int, link md.Link) bool {
	if index < 0 || index >= len(lines) {
		return false
	}
	_, start := focusedLinkTarget(md.StripANSI(lines[index]), link)
	return start >= 0
}

func clickedLinkTarget(plain string, link md.Link, x int) (string, int, int, bool) {
	for _, target := range []string{strings.TrimSpace(link.URL), strings.TrimSpace(link.Text)} {
		if target == "" {
			continue
		}
		offset := 0
		for offset <= len(plain) {
			segment, segmentStart, segmentEnd, ok := linkTargetSegmentInLine(plain[offset:], target, link)
			if !ok {
				break
			}
			startByte := offset + segmentStart
			start := lipgloss.Width(plain[:startByte])
			end := start + lipgloss.Width(segment)
			if x >= start && x < end {
				return segment, start, end, true
			}
			offset += segmentEnd
		}
	}
	return "", -1, -1, false
}

func focusedLinkTarget(plain string, link md.Link) (string, int) {
	for _, target := range []string{strings.TrimSpace(link.URL), strings.TrimSpace(link.Text)} {
		if target == "" {
			continue
		}
		if segment, start, _, ok := linkTargetSegmentInLine(plain, target, link); ok {
			return segment, lipgloss.Width(plain[:start])
		}
	}
	return "", -1
}

func linkTargetSegmentInLine(line, target string, link md.Link) (string, int, int, bool) {
	if segment, start, end, ok := exactLinkSegmentInLine(line, target); ok {
		return segment, start, end, true
	}
	if link.Text == link.URL && hasScheme(link.URL) {
		return bestLinkSegmentInLine(line, target)
	}
	return "", 0, 0, false
}

func exactLinkSegmentInLine(line, target string) (string, int, int, bool) {
	start := strings.Index(line, target)
	if start < 0 {
		return "", 0, 0, false
	}
	return target, start, start + len(target), true
}

func bestLinkSegmentInLine(line, target string) (string, int, int, bool) {
	if segment, start, end, ok := exactLinkSegmentInLine(line, target); ok {
		return segment, start, end, true
	}

	bounds := runeBoundaries(line)
	bestStart, bestEnd, bestWidth := 0, 0, 0
	minWidth := min(6, lipgloss.Width(target))
	for i := 0; i < len(bounds)-1; i++ {
		for j := len(bounds) - 1; j > i; j-- {
			segment := strings.Trim(line[bounds[i]:bounds[j]], " ↪<>")
			if segment == "" || !strings.Contains(target, segment) {
				continue
			}
			width := lipgloss.Width(segment)
			if width < minWidth {
				continue
			}
			if !strings.ContainsAny(segment, ":/.-_?&=#%") && !strings.Contains(line, "↪") {
				continue
			}
			if width > bestWidth {
				trimmedStart := bounds[i] + strings.Index(line[bounds[i]:bounds[j]], segment)
				bestStart = trimmedStart
				bestEnd = trimmedStart + len(segment)
				bestWidth = width
			}
			break
		}
	}
	if bestWidth <= 0 {
		return "", 0, 0, false
	}
	return line[bestStart:bestEnd], bestStart, bestEnd, true
}

func runeBoundaries(s string) []int {
	bounds := make([]int, 0, utf8.RuneCountInString(s)+1)
	for i := range s {
		bounds = append(bounds, i)
	}
	return append(bounds, len(s))
}

func (m Model) styleFocusedJump(line string) string {
	plain := md.StripANSI(line)
	target := strings.TrimSpace(m.focusedJumpText)
	if target != "" {
		startByte := strings.Index(plain, target)
		if startByte >= 0 {
			start := lipgloss.Width(plain[:startByte])
			return styleANSIVisibleRangePlain(line, start, start+lipgloss.Width(target), jumpFocusStyle)
		}
	}
	return jumpFocusStyle.Render(line)
}

func (m Model) linkStatus(link md.Link) string {
	if _, ok := m.anchorForMarkdownLink(link.URL); ok {
		return fmt.Sprintf("link: enter jump, y copy  %s", link.URL)
	}
	return fmt.Sprintf("link: y copy  %s", link.URL)
}

func (m *Model) followMarkdownLink(url string) bool {
	anchor, ok := m.anchorForMarkdownLink(url)
	if !ok {
		return false
	}

	for _, heading := range m.doc.Outline {
		if slug(heading.Text) == anchor {
			m.jumpToRawLine(heading.Line, true)
			m.focusedJumpLine = heading.Line
			m.focusedJumpText = heading.Text
			m.status = fmt.Sprintf("jumped to #%s", anchor)
			m.rebuildContent()
			return true
		}
	}
	m.status = fmt.Sprintf("anchor not found: #%s", anchor)
	return true
}

func (m Model) focusedLinkCanJump() bool {
	if m.selectedLink < 0 || m.selectedLink >= len(m.doc.Links) {
		return false
	}
	_, ok := m.anchorForMarkdownLink(m.doc.Links[m.selectedLink].URL)
	return ok
}

func (m Model) isInternalJumpLink(link md.Link) bool {
	_, ok := m.anchorForMarkdownLink(link.URL)
	return ok
}

func (m Model) anchorForMarkdownLink(url string) (string, bool) {
	if strings.HasPrefix(url, "#") {
		anchor := strings.TrimPrefix(url, "#")
		return anchor, anchor != ""
	}
	if hasScheme(url) || !strings.Contains(url, "#") {
		return "", false
	}
	parts := strings.SplitN(url, "#", 2)
	if parts[1] == "" {
		return "", false
	}
	if parts[0] == "" || filepath.Clean(parts[0]) == filepath.Base(m.doc.Path) || filepath.Clean(parts[0]) == m.doc.Path {
		return parts[1], true
	}
	return "", false
}

func (m *Model) clearJumpFocus() {
	if m.focusedJumpLine < 0 {
		return
	}
	m.focusedJumpLine = -1
	m.focusedJumpText = ""
	m.rebuildContent()
}

func (m *Model) clearOutlineFocus() {
	if m.focusedOutline < 0 {
		return
	}
	m.focusedOutline = -1
	m.rebuildContent()
}

func (m *Model) clearTransientHighlights() {
	m.selectedLink = -1
	m.selectedTask = -1
	m.outlineVisible = false
	m.clearLineSelection()
	m.focusedJumpLine = -1
	m.focusedJumpText = ""
	m.focusedOutline = -1
	m.searchQuery = ""
	m.searchMatches = nil
	m.selectedMatch = -1
	m.status = ""
	m.resize()
	m.rebuildContent()
}

func (m *Model) copySelection() {
	text, count, ok := m.selectedLineText()
	if ok {
		if err := clipboardWrite(text); err != nil {
			m.status = fmt.Sprintf("copy failed: %v", err)
			return
		}
		m.showCopiedMessage(fmt.Sprintf("copied %d %s", count, plural(count, "line")))
		return
	}
	m.copySelectedLink()
}

func (m *Model) copySelectedLink() {
	link, ok := m.copyTargetLink()
	if !ok {
		m.status = "no link on current line"
		return
	}
	if err := clipboardWrite(link.URL); err != nil {
		m.status = fmt.Sprintf("copy failed: %v", err)
		return
	}
	m.showCopiedMessage(fmt.Sprintf("copied: %s", link.URL))
}

func (m *Model) showCopiedMessage(status string) {
	m.showMessage("Copied", fmt.Sprintf("Copied to clipboard.\n\n%s", mutedStyle.Render("press any key or click outside to close")))
	m.status = status
}

func (m *Model) showMessage(title, body string) {
	m.modalKind = modalMessage
	m.modalTitle = title
	m.modalBody = body
}

func (m Model) copyTargetLink() (md.Link, bool) {
	if m.selectedLink >= 0 && m.selectedLink < len(m.doc.Links) {
		return m.doc.Links[m.selectedLink], true
	}

	raw := m.rawLineForRendered(m.body.YOffset)
	for _, link := range m.doc.Links {
		if link.Line == raw {
			return link, true
		}
	}
	return md.Link{}, false
}

func (m *Model) jumpToRawLine(rawLine int, syncOutline bool) {
	m.jumpToRenderedLine(m.lineForRaw(rawLine), syncOutline)
}

func (m *Model) jumpToRenderedLine(line int, syncOutline bool) {
	if line < 0 {
		line = 0
	}
	m.body.SetYOffset(max(0, line-(m.body.Height/2)))
	if syncOutline {
		m.selectCurrentOutline()
	}
}

func (m *Model) jumpToRenderedLineNearTop(line, offset int) {
	if line < 0 {
		line = 0
	}
	m.body.SetYOffset(max(0, line-offset))
}

func (m Model) lineForRaw(rawLine int) int {
	if rawLine <= 0 || len(m.rawToRendered) == 0 {
		return 0
	}
	if rawLine >= len(m.rawToRendered) {
		return m.rawToRendered[len(m.rawToRendered)-1]
	}
	return m.rawToRendered[rawLine]
}

func (m Model) rawLineForRendered(line int) int {
	if len(m.renderedToRaw) == 0 {
		return 1
	}
	line = clamp(line, 0, len(m.renderedToRaw)-1)
	return m.renderedToRaw[line]
}

func buildLineMaps(raw string, rendered []string) ([]int, []int) {
	rawLines := strings.Split(raw, "\n")
	rawToRendered := make([]int, len(rawLines)+1)
	renderedToRaw := make([]int, max(1, len(rendered)))

	cursor := 0
	for i, rawLine := range rawLines {
		rawNumber := i + 1
		key := normalizeMarkdownLine(rawLine)
		if key != "" && len(rendered) > 0 {
			cursor = findRenderedLine(rendered, key, cursor)
		}
		rawToRendered[rawNumber] = cursor
	}

	nextRaw := 1
	for line := range renderedToRaw {
		for nextRaw+1 < len(rawToRendered) && rawToRendered[nextRaw+1] <= line {
			nextRaw++
		}
		renderedToRaw[line] = nextRaw
	}

	return rawToRendered, renderedToRaw
}

func findRenderedLine(rendered []string, key string, cursor int) int {
	windowEnd := min(len(rendered), cursor+16)
	for i := cursor; i < windowEnd; i++ {
		if normalizedContains(rendered[i], key) {
			return i
		}
	}
	for i := cursor; i < len(rendered); i++ {
		if normalizedContains(rendered[i], key) {
			return i
		}
	}
	for i := 0; i < cursor; i++ {
		if normalizedContains(rendered[i], key) {
			return i
		}
	}
	return clamp(cursor, 0, max(0, len(rendered)-1))
}

func normalizedContains(line, key string) bool {
	return strings.Contains(strings.ToLower(line), strings.ToLower(key))
}

var (
	markdownLinkRE         = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	listMarkerRE           = regexp.MustCompile(`^([-*+]|\d+[.)])\s+`)
	taskLineRE             = regexp.MustCompile(`^(\s*[-*+]\s+\[)([ xX])(\])`)
	renderedTaskCheckboxRE = regexp.MustCompile(`\[[ xX]\]`)
)

func parseTaskItems(raw string) []taskItem {
	lines := strings.Split(raw, "\n")
	tasks := make([]taskItem, 0)
	for i, line := range lines {
		matches := taskLineRE.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		text := strings.TrimSpace(line[len(matches[0]):])
		tasks = append(tasks, taskItem{
			Line:    i + 1,
			Checked: strings.EqualFold(matches[2], "x"),
			Text:    strings.Trim(text, "`*_~"),
		})
	}
	return tasks
}

func toggleTaskLine(line string) (string, bool) {
	loc := taskLineRE.FindStringSubmatchIndex(line)
	if loc == nil {
		return line, false
	}
	stateStart, stateEnd := loc[4], loc[5]
	next := "x"
	if strings.EqualFold(line[stateStart:stateEnd], "x") {
		next = " "
	}
	return line[:stateStart] + next + line[stateEnd:], true
}

func normalizeMarkdownLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)
	line = listMarkerRE.ReplaceAllString(line, "")
	line = markdownLinkRE.ReplaceAllString(line, "$1")
	line = strings.Trim(line, "<>")
	line = strings.Trim(line, "`*_~")
	if len(line) > 48 {
		line = line[:48]
	}
	return strings.TrimSpace(line)
}

func styleLineRange(line string, start, end int, style lipgloss.Style) string {
	if start < 0 || end <= start || start >= len(line) {
		return line
	}
	end = min(end, len(line))
	return line[:start] + style.Render(line[start:end]) + line[end:]
}

func styleDisplayRangePlain(line string, start, end int, style lipgloss.Style) string {
	plain := md.StripANSI(line)
	startByte, endByte, ok := displayColumnRangeBytes(plain, start, end)
	if !ok {
		return plain
	}
	return plain[:startByte] + style.Render(plain[startByte:endByte]) + plain[endByte:]
}

func displayColumnSlice(line string, start, end int) string {
	startByte, endByte, ok := displayColumnRangeBytes(line, start, end)
	if !ok {
		return ""
	}
	return line[startByte:endByte]
}

func displayColumnRangeBytes(line string, start, end int) (int, int, bool) {
	if start < 0 || end <= start {
		return 0, 0, false
	}
	width := lipgloss.Width(line)
	start = clamp(start, 0, width)
	end = clamp(end, 0, width)
	if end <= start {
		return 0, 0, false
	}
	return displayColumnByteIndex(line, start), displayColumnByteIndex(line, end), true
}

func selectionLineStartColumn(line string) int {
	return min(2, leadingWhitespaceWidth(line))
}

func leadingWhitespaceWidth(line string) int {
	width := 0
	for _, r := range line {
		if r != ' ' && r != '\t' {
			return width
		}
		width += lipgloss.Width(string(r))
	}
	return width
}

func displayColumnByteIndex(line string, column int) int {
	if column <= 0 {
		return 0
	}
	current := 0
	for i, r := range line {
		next := current + lipgloss.Width(string(r))
		if next > column {
			return i
		}
		if next == column {
			return i + utf8.RuneLen(r)
		}
		current = next
	}
	return len(line)
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

func styleANSIVisibleRange(line string, start, end int, style lipgloss.Style) string {
	startByte, endByte, ok := ansiVisibleRangeBytes(line, start, end)
	if !ok {
		return line
	}
	return line[:startByte] + style.Render(line[startByte:endByte]) + line[endByte:]
}

func styleANSIVisibleRangePlain(line string, start, end int, style lipgloss.Style) string {
	startByte, endByte, ok := ansiVisibleRangeBytes(line, start, end)
	if !ok {
		return line
	}
	return line[:startByte] + style.Render(md.StripANSI(line[startByte:endByte])) + line[endByte:]
}

func wrapDisplayHard(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	if s == "" {
		return []string{""}
	}

	lines := make([]string, 0)
	var current strings.Builder
	currentWidth := 0
	for _, r := range s {
		rWidth := lipgloss.Width(string(r))
		if currentWidth > 0 && currentWidth+rWidth > width {
			lines = append(lines, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rWidth
	}
	lines = append(lines, current.String())
	return lines
}

func wrapANSIHard(line string, width int) []string {
	if width <= 0 || lipgloss.Width(md.StripANSI(line)) <= width {
		return []string{line}
	}
	totalWidth := lipgloss.Width(md.StripANSI(line))
	lines := make([]string, 0, (totalWidth/width)+1)
	for start := 0; start < totalWidth; start += width {
		end := min(totalWidth, start+width)
		lines = append(lines, ansiVisibleColumnSlice(line, start, end))
	}
	return lines
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

func ansiVisibleRangeBytes(line string, start, end int) (int, int, bool) {
	if start < 0 || end <= start {
		return 0, 0, false
	}

	visible := 0
	startByte := -1
	endByte := -1
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

		if visible >= start && startByte < 0 {
			startByte = i
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		if size <= 0 {
			size = 1
		}
		width := lipgloss.Width(string(r))
		if startByte < 0 && visible+width > start {
			startByte = i
		}
		visible += width
		i += size
		if visible >= end {
			endByte = i
			break
		}
	}

	if startByte < 0 {
		return 0, 0, false
	}
	if endByte < 0 {
		endByte = len(line)
	}
	return startByte, endByte, true
}

func clamp(n, low, high int) int {
	if high < low {
		return low
	}
	return min(max(n, low), high)
}

func hasScheme(url string) bool {
	return strings.Contains(url, "://") || strings.HasPrefix(url, "mailto:")
}

func plural(count int, singular string) string {
	if count == 1 {
		return singular
	}
	return singular + "s"
}

func slug(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	dash := false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func helpItems() []helpItem {
	return []helpItem{
		{Key: "j / k", Name: "scroll", Description: "move down or up"},
		{Key: "arrows", Name: "scroll", Description: "move down, up, left, or right"},
		{Key: "g / G", Name: "jump", Description: "go to top or bottom"},
		{Key: "o", Name: "outline", Description: "toggle the heading pane"},
		{Key: "enter", Name: "toggle", Description: "toggle a focused checkbox"},
		{Key: "space", Name: "toggle", Description: "toggle a focused checkbox"},
		{Key: "/", Name: "search", Description: "search rendered text"},
		{Key: "n / N", Name: "search", Description: "next or previous match"},
		{Key: "tab", Name: "checkboxes", Description: "focus the next checkbox"},
		{Key: "shift+tab", Name: "checkboxes", Description: "focus the previous checkbox"},
		{Key: "y", Name: "copy", Description: "copy selected text, focused URL, or current-line URL"},
		{Key: "mouse wheel", Name: "scroll", Description: "scroll the active text pane"},
		{Key: "drag", Name: "select", Description: "select rendered text for copying"},
		{Key: "click", Name: "actions", Description: "toggle a checkbox, copy a URL, or confirm an internal jump"},
		{Key: "?", Name: "help", Description: "open this searchable guide"},
		{Key: "esc", Name: "close", Description: "close help/search or clear focus"},
		{Key: "q", Name: "quit", Description: "exit mdpoke"},
	}
}

var (
	clipboardWrite = clipboard.WriteAll

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Background(lipgloss.Color("235"))

	guideStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Background(lipgloss.Color("236"))

	confirmButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235")).
				Background(lipgloss.Color("151")).
				Bold(true)

	cancelButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235")).
				Background(lipgloss.Color("245")).
				Bold(true)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("81"))

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("81")).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	linkFocusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(lipgloss.Color("117")).
			Bold(true)

	bareAutoLinkStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("69")).
				Underline(true)

	taskFocusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(lipgloss.Color("151")).
			Bold(true)

	jumpFocusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(lipgloss.Color("229")).
			Bold(true)

	lineSelectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235")).
				Background(lipgloss.Color("116"))

	searchMatchStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235")).
				Background(lipgloss.Color("186"))

	activeSearchMatchStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235")).
				Background(lipgloss.Color("214")).
				Bold(true)

	dividerStyle = lipgloss.NewStyle().
			Width(1).
			Foreground(lipgloss.Color("238")).
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("238"))

	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("235"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Padding(1, 2)
)
