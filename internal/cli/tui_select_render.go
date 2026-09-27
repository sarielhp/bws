package cli

import (
	"fmt"
	"io"
	"strings"

	"bws/internal/stack"

	"github.com/fatih/color"
)

func renderStackSelector(w io.Writer, title string, items []StackChoice, selected int, termWidth int) int {
	var lines []string

	header := color.New(color.FgWhite, color.Bold).Sprint(title)
	hint := color.New(color.FgHiBlack).Sprint(" (↑/↓ or j/k to navigate, Enter to select, Esc to cancel):")
	lines = append(lines, header+hint)

	for i, item := range items {
		lines = append(lines, renderItemLine(item, i, i == selected, termWidth))
	}

	details := renderDetailPane(items[selected], termWidth)
	lines = append(lines, details...)

	for _, line := range lines {
		fmt.Fprintf(w, "\r\x1b[2K%s\n", line)
	}
	return len(lines)
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

	var formattedName string
	if isSelected {
		formattedName = color.New(color.FgHiWhite, color.Bold).Sprintf("%-16s", name)
	} else {
		formattedName = color.New(color.FgWhite).Sprintf("%-16s", name)
	}

	catFormatted := color.New(color.FgHiBlack).Sprintf("%-23s", cat)
	badgeFormatted := ""
	if item.Badge != "" {
		badgeFormatted = " " + color.New(color.FgHiGreen, color.Bold).Sprintf("(%s)", item.Badge)
	}

	line := indicator + numStr + formattedName + " " + catFormatted + badgeFormatted
	return line
}

func renderDetailPane(item StackChoice, termWidth int) []string {
	sepLen := termWidth - 2
	if sepLen < 40 {
		sepLen = 76
	} else if sepLen > 80 {
		sepLen = 80
	}

	sepLine := color.New(color.FgCyan).Sprint(strings.Repeat("─", sepLen))
	paneHeader := color.New(color.FgCyan).Sprint("─── Stack Details ") +
		color.New(color.FgCyan).Sprint(strings.Repeat("─", max(0, sepLen-18)))

	lbl := color.New(color.FgYellow, color.Bold).SprintFunc()
	dim := color.New(color.FgHiBlack).SprintFunc()

	var pane []string
	pane = append(pane, paneHeader)

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

	pane = append(pane, fmt.Sprintf("  %s %s %s", lbl("Stack:      "), color.New(color.FgHiWhite, color.Bold).Sprint(title), dim(fmt.Sprintf("(%s)", stk.Name))))
	pane = append(pane, fmt.Sprintf("  %s %s %s", lbl("Category:   "), stk.Category, dim(fmt.Sprintf("[source: %s]", stk.Source))))
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
