package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"bws/internal/stack"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
)

var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiEscapeRegex.ReplaceAllString(s, "")
}

func visualWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

func truncateANSI(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if visualWidth(s) <= maxWidth {
		return s
	}

	limit := maxWidth - 1
	var b strings.Builder
	curWidth := 0
	inEsc := false

	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			b.WriteRune(r)
			continue
		}
		if inEsc {
			b.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		rw := runewidth.RuneWidth(r)
		if curWidth+rw > limit {
			break
		}
		b.WriteRune(r)
		curWidth += rw
	}
	b.WriteString("…\x1b[0m")
	return b.String()
}

func renderStackSelector(w io.Writer, title string, items []StackChoice, selected int, termWidth, termHeight int) int {
	maxWidth := max(10, termWidth-1)
	if termHeight <= 0 {
		termHeight = 24
	}

	var lines []string

	headerTitle := title
	if termWidth < 50 && strings.HasPrefix(title, "Recommended Stacks") {
		headerTitle = "Recommended Stacks"
	}
	lines = append(lines, renderHeaderLine(headerTitle, termWidth))

	for i, item := range items {
		lines = append(lines, renderItemLine(item, i, i == selected, termWidth))
	}

	availRows := termHeight - len(lines) - 1
	if availRows >= 3 && termWidth >= 35 {
		details := renderDetailPane(items[selected], termWidth)
		if len(details) > availRows {
			details = trimDetailPane(details, availRows)
		}
		lines = append(lines, details...)
	}

	for _, line := range lines {
		fmt.Fprintf(w, "\r\x1b[2K%s\n", truncateANSI(line, maxWidth))
	}
	return len(lines)
}

func trimDetailPane(details []string, maxRows int) []string {
	if len(details) <= maxRows || maxRows < 2 {
		return details[:min(len(details), maxRows)]
	}
	trimmed := append([]string{}, details[:maxRows-1]...)
	trimmed = append(trimmed, details[len(details)-1])
	return trimmed
}

func renderHeaderLine(title string, termWidth int) string {
	header := color.New(color.FgWhite, color.Bold).Sprint(title)
	var hintText string
	switch {
	case termWidth >= 96:
		hintText = " (↑/↓ or j/k to navigate, Enter to select, Esc to cancel):"
	case termWidth >= 75:
		hintText = " (↑/↓ to navigate, Enter to select, Esc):"
	case termWidth >= 55:
		hintText = " (↑/↓, Enter to select, Esc):"
	case termWidth >= 40:
		hintText = " (↑/↓, Enter):"
	default:
		hintText = ":"
	}
	hint := color.New(color.FgHiBlack).Sprint(hintText)
	return header + hint
}

func renderItemLine(item StackChoice, idx int, isSelected bool, termWidth int) string {
	indicator := "   "
	if isSelected {
		indicator = color.New(color.FgHiCyan, color.Bold).Sprint(" ▸ ")
	}

	numStr := fmt.Sprintf("%2d. ", idx+1)
	name := item.Label
	cat := ""
	if item.Stack != nil {
		name = item.Stack.Name
		cat = fmt.Sprintf("[%s]", item.Stack.Category)
	}

	nameWidth := 16
	if termWidth < 60 {
		nameWidth = 14
	}
	if termWidth < 45 {
		nameWidth = 12
	}

	var formattedName string
	if isSelected {
		formattedName = color.New(color.FgHiWhite, color.Bold).Sprintf("%-*s", nameWidth, name)
	} else {
		formattedName = color.New(color.FgWhite).Sprintf("%-*s", nameWidth, name)
	}

	badgeFormatted := ""
	if item.Badge != "" {
		badgeFormatted = color.New(color.FgHiGreen, color.Bold).Sprintf("(%s)", item.Badge)
	}

	catFormatted := ""
	if cat != "" && termWidth >= 65 {
		catFormatted = color.New(color.FgHiBlack).Sprintf("%-23s", cat)
	}

	var parts []string
	parts = append(parts, indicator+numStr+formattedName)
	if catFormatted != "" {
		parts = append(parts, catFormatted)
	}
	if badgeFormatted != "" {
		parts = append(parts, badgeFormatted)
	}

	return strings.Join(parts, " ")
}

func renderDetailPane(item StackChoice, termWidth int) []string {
	sepLen := max(10, min(80, termWidth-2))
	sepLine := color.New(color.FgCyan).Sprint(strings.Repeat("─", sepLen))

	paneHeader := "─── Stack Details "
	if sepLen < 22 {
		paneHeader = "─── Details "
	}
	if sepLen > visualWidth(paneHeader) {
		paneHeader += strings.Repeat("─", sepLen-visualWidth(paneHeader))
	}
	paneHeaderFormatted := color.New(color.FgCyan).Sprint(paneHeader)

	lbl := color.New(color.FgYellow, color.Bold).SprintFunc()
	dim := color.New(color.FgHiBlack).SprintFunc()

	var pane []string
	pane = append(pane, paneHeaderFormatted)

	if item.Stack == nil {
		pane = append(pane, fmt.Sprintf("  %s %s", lbl("Selection:  "), item.Label))
		pane = append(pane, fmt.Sprintf("  %s %s", lbl("Description:"), "Configure workspace with detected raw tool profiles"))
		pane = append(pane, sepLine)
		return pane
	}

	stk := item.Stack
	title := stk.Title
	if title == "" {
		title = stk.Name
	}

	if termWidth >= 60 {
		pane = append(pane, fmt.Sprintf("  %s %s %s", lbl("Stack:      "), color.New(color.FgHiWhite, color.Bold).Sprint(title), dim(fmt.Sprintf("(%s)", stk.Name))))
	} else {
		pane = append(pane, fmt.Sprintf("  %s %s", lbl("Stack:      "), color.New(color.FgHiWhite, color.Bold).Sprint(title)))
	}

	if termWidth >= 55 {
		pane = append(pane, fmt.Sprintf("  %s %s %s", lbl("Category:   "), stk.Category, dim(fmt.Sprintf("[source: %s]", stk.Source))))
	} else {
		pane = append(pane, fmt.Sprintf("  %s %s", lbl("Category:   "), stk.Category))
	}

	pane = append(pane, fmt.Sprintf("  %s %s", lbl("Description:"), stk.Description))

	if len(stk.Profiles) > 0 {
		var coloredProfs []string
		for _, p := range stk.Profiles {
			coloredProfs = append(coloredProfs, color.New(color.FgCyan).Sprint(p))
		}
		pane = append(pane, fmt.Sprintf("  %s %s", lbl("Profiles:   "), strings.Join(coloredProfs, ", ")))
	}

	secTags := extractSecurityHighlights(stk)
	if len(secTags) > 0 {
		pane = append(pane, fmt.Sprintf("  %s %s", lbl("Security:   "), strings.Join(secTags, " ")))
	}

	pane = append(pane, sepLine)
	return pane
}

func extractSecurityHighlights(stk *stack.Stack) []string {
	var tags []string
	tagStyle := color.New(color.FgHiBlue, color.Bold).SprintFunc()

	secProfiles := map[string]string{
		"no-sudo":      "[No-Sudo]",
		"no-ssh":       "[No-SSH]",
		"no-forge":     "[No-Forge]",
		"no-gh":        "[No-Forge]",
		"no-browser":   "[No-Browser]",
		"no-email":     "[No-Email]",
		"no-secrets":   "[No-Secrets]",
		"no-history":   "[No-History]",
		"secure-agent": "[Hardened-Agent]",
	}

	seen := make(map[string]bool)
	for _, p := range stk.Profiles {
		if tag, ok := secProfiles[p]; ok && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tagStyle(tag))
		}
	}
	if stk.Features != nil && stk.Features.NoNet != nil && *stk.Features.NoNet {
		if !seen["[Offline]"] {
			seen["[Offline]"] = true
			tags = append(tags, tagStyle("[Offline]"))
		}
	}
	return tags
}
