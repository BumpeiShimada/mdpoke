package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/lipgloss"
)

const customBlockIndent = "  "

type customBlock struct {
	placeholder string
	rendered    string
}

func prepareCustomBlocks(markdown string, width int) (string, []customBlock) {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	prepared := make([]string, 0, len(lines))
	blocks := make([]customBlock, 0)

	for i := 0; i < len(lines); i++ {
		if marker, language, ok := codeFenceStart(lines[i]); ok {
			codeLines := make([]string, 0)
			i++
			for i < len(lines) {
				if codeFenceEnd(lines[i], marker) {
					break
				}
				codeLines = append(codeLines, lines[i])
				i++
			}
			placeholder := customBlockPlaceholder(len(blocks))
			prepared = append(prepared, "", placeholder, "")
			blocks = append(blocks, customBlock{
				placeholder: placeholder,
				rendered:    renderCodeBlock(codeLines, language, width),
			})
			continue
		}

		if i+1 < len(lines) && isTableHeaderLine(lines[i]) && isTableSeparatorLine(lines[i+1]) {
			tableLines := []string{lines[i], lines[i+1]}
			i += 2
			for i < len(lines) && strings.TrimSpace(lines[i]) != "" && strings.Contains(lines[i], "|") {
				tableLines = append(tableLines, lines[i])
				i++
			}
			i--
			placeholder := customBlockPlaceholder(len(blocks))
			prepared = append(prepared, "", placeholder, "")
			blocks = append(blocks, customBlock{
				placeholder: placeholder,
				rendered:    renderTableBlock(tableLines, width),
			})
			continue
		}

		prepared = append(prepared, lines[i])
	}

	return strings.Join(prepared, "\n"), blocks
}

func replaceCustomBlocks(rendered string, blocks []customBlock) string {
	if len(blocks) == 0 {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	replaced := make([]string, 0, len(lines))
	for _, line := range lines {
		plain := StripANSI(line)
		found := false
		for _, block := range blocks {
			if strings.Contains(plain, block.placeholder) {
				replaced = append(replaced, strings.Split(strings.TrimRight(block.rendered, "\n"), "\n")...)
				found = true
				break
			}
		}
		if !found {
			replaced = append(replaced, line)
		}
	}

	return strings.Join(replaced, "\n")
}

func customBlockPlaceholder(index int) string {
	return fmt.Sprintf("@@MDPOKE_CUSTOM_BLOCK_%04d@@", index)
}

func codeFenceStart(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	for _, markerRune := range []byte{'`', '~'} {
		marker := strings.Repeat(string(markerRune), 3)
		if !strings.HasPrefix(trimmed, marker) {
			continue
		}

		count := 0
		for count < len(trimmed) && trimmed[count] == markerRune {
			count++
		}
		fence := strings.Repeat(string(markerRune), count)
		info := strings.TrimSpace(trimmed[count:])
		language := ""
		if info != "" {
			language = strings.Fields(info)[0]
		}
		return fence, language, true
	}
	return "", "", false
}

func codeFenceEnd(line, marker string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), marker)
}

func renderCodeBlock(codeLines []string, language string, width int) string {
	innerMax := max(10, width-lipgloss.Width(customBlockIndent)-2)
	contentMax := max(8, innerMax-2)
	wrapped := make([]string, 0, len(codeLines))
	for _, line := range codeLines {
		wrapped = append(wrapped, wrapDisplayHard(strings.ReplaceAll(line, "\t", "    "), contentMax)...)
	}
	if len(wrapped) == 0 {
		wrapped = []string{""}
	}

	longest := 0
	for _, line := range wrapped {
		longest = max(longest, lipgloss.Width(line))
	}
	title := "code"
	if language != "" {
		title = language
	}
	innerWidth := min(innerMax, max(longest, codeBlockTitleWidth(title)))

	highlighted := highlightCode(strings.Join(wrapped, "\n"), language)
	highlightedLines := strings.Split(strings.TrimRight(highlighted, "\n"), "\n")
	if len(highlightedLines) == 0 {
		highlightedLines = []string{""}
	}

	out := make([]string, 0, len(highlightedLines)+2)
	out = append(out, customBlockIndent+topRule(title, innerWidth))
	for _, line := range highlightedLines {
		out = append(out, customBlockIndent+line)
	}
	out = append(out, customBlockIndent+bottomRule(innerWidth))
	return strings.Join(out, "\n")
}

func highlightCode(source, language string) string {
	if language == "" {
		language = "plaintext"
	}

	var buf bytes.Buffer
	if err := quick.Highlight(&buf, source, language, "terminal16m", "catppuccin-mocha"); err == nil {
		return buf.String()
	}

	buf.Reset()
	if err := quick.Highlight(&buf, source, "plaintext", "terminal16m", "catppuccin-mocha"); err == nil {
		return buf.String()
	}

	return source
}

func topRule(title string, innerWidth int) string {
	label := "─ " + title + " "
	labelWidth := lipgloss.Width(label)
	if labelWidth > innerWidth {
		label = "─ "
		labelWidth = lipgloss.Width(label)
	}
	return "╭" + label + strings.Repeat("─", max(0, innerWidth-labelWidth)) + "╮"
}

func codeBlockTitleWidth(title string) int {
	return lipgloss.Width("─ " + title + " ")
}

func bottomRule(innerWidth int) string {
	return "╰" + strings.Repeat("─", innerWidth) + "╯"
}

func isTableHeaderLine(line string) bool {
	return strings.Contains(line, "|") && len(splitTableLine(line)) > 0
}

func isTableSeparatorLine(line string) bool {
	cells := splitTableLine(line)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		trimmed := strings.TrimSpace(cell)
		if trimmed == "" {
			return false
		}
		trimmed = strings.TrimPrefix(trimmed, ":")
		trimmed = strings.TrimSuffix(trimmed, ":")
		if len(trimmed) < 3 {
			return false
		}
		for _, r := range trimmed {
			if r != '-' {
				return false
			}
		}
	}
	return true
}

func splitTableLine(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")

	cells := make([]string, 0)
	var cell strings.Builder
	escaped := false
	for _, r := range trimmed {
		if escaped {
			cell.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '|' {
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
			continue
		}
		cell.WriteRune(r)
	}
	if escaped {
		cell.WriteRune('\\')
	}
	cells = append(cells, strings.TrimSpace(cell.String()))
	return cells
}

func renderTableBlock(tableLines []string, width int) string {
	rows := make([][]string, 0, len(tableLines)-1)
	for index, line := range tableLines {
		if index == 1 {
			continue
		}
		rows = append(rows, splitTableLine(line))
	}
	if len(rows) == 0 {
		return ""
	}

	cols := 0
	for _, row := range rows {
		cols = max(cols, len(row))
	}
	if cols == 0 {
		return ""
	}

	for i := range rows {
		for len(rows[i]) < cols {
			rows[i] = append(rows[i], "")
		}
	}

	available := max(cols*4+cols+1, width-lipgloss.Width(customBlockIndent))
	maxColumnWidth := max(4, (available-(cols+1)-(2*cols))/cols)
	widths := tableColumnWidths(rows, maxColumnWidth, available)

	out := make([]string, 0, len(rows)*2+3)
	out = append(out, customBlockIndent+tableBorder("┌", "┬", "┐", widths))
	for rowIndex, row := range rows {
		wrappedCells := wrapTableRow(row, widths)
		for _, cellLine := range wrappedCells {
			out = append(out, customBlockIndent+renderTableRow(cellLine, widths))
		}
		if rowIndex == 0 {
			out = append(out, customBlockIndent+tableBorder("├", "┼", "┤", widths))
		}
	}
	out = append(out, customBlockIndent+tableBorder("└", "┴", "┘", widths))
	return strings.Join(out, "\n")
}

func tableColumnWidths(rows [][]string, maxColumnWidth, available int) []int {
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], lipgloss.Width(cell)+1)
		}
	}

	total := 0
	tooWide := false
	for i, width := range widths {
		if width > maxColumnWidth {
			tooWide = true
		}
		widths[i] = max(4, width)
		total += widths[i]
	}
	tableWidth := total + (len(widths) * 2) + len(widths) + 1
	if !tooWide && tableWidth <= available {
		return widths
	}

	for i := range widths {
		widths[i] = maxColumnWidth
	}
	return widths
}

func wrapTableRow(row []string, widths []int) [][]string {
	wrapped := make([][]string, len(row))
	maxLines := 1
	for i, cell := range row {
		wrapped[i] = wrapDisplayHard(cell, widths[i])
		maxLines = max(maxLines, len(wrapped[i]))
	}

	lines := make([][]string, maxLines)
	for lineIndex := range lines {
		lines[lineIndex] = make([]string, len(row))
		for cellIndex := range row {
			if lineIndex < len(wrapped[cellIndex]) {
				lines[lineIndex][cellIndex] = wrapped[cellIndex][lineIndex]
			}
		}
	}
	return lines
}

func renderTableRow(row []string, widths []int) string {
	var out strings.Builder
	out.WriteString("│")
	for i, cell := range row {
		out.WriteString(" ")
		out.WriteString(padRightDisplay(cell, widths[i]))
		out.WriteString(" │")
	}
	return out.String()
}

func tableBorder(left, middle, right string, widths []int) string {
	parts := make([]string, len(widths))
	for i, width := range widths {
		parts[i] = strings.Repeat("─", width+2)
	}
	return left + strings.Join(parts, middle) + right
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

func padRightDisplay(s string, width int) string {
	padding := max(0, width-lipgloss.Width(s))
	return s + strings.Repeat(" ", padding)
}
