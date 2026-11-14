package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m *setupTUI) View() string {
	if m.done {
		return ""
	}
	width := max(40, m.width)
	head := []string{
		ansi.Truncate(stTitle.Render(m.title), width, "…"),
		ansi.Truncate(m.renderTabs(), width, "…"),
	}
	var body string
	if width >= sideBySideMin {
		listW := width * 11 / 20
		list := stBox.Width(listW - 2).Render(m.renderList(listW - 4))
		inner := lipgloss.Height(list) - 2
		details := stBox.Width(width - listW - 2).Height(inner).
			Render(capLines(m.renderDetails(width-listW-4), inner))
		body = lipgloss.JoinHorizontal(lipgloss.Top, list, details)
	} else {
		list := stBox.Width(width - 2).Render(m.renderList(width - 4))
		details := capLines(m.renderDetails(width-2), m.height-lipgloss.Height(list)-4)
		body = lipgloss.JoinVertical(lipgloss.Left, list, details)
	}
	help := setupHelp
	if m.filtering {
		help = setupHelpTyped
	}
	foot := []string{
		ansi.Truncate(m.renderSummary(), width, "…"),
		ansi.Truncate(stDim.Render(help), width, "…"),
	}
	return strings.Join(head, "\n") + "\n" + body + "\n" + strings.Join(foot, "\n")
}

func (m *setupTUI) renderTabs() string {
	if m.filter != "" || m.filtering {
		return stTabOn.Render("Search: " + m.filter + "▏")
	}
	var parts []string
	for i, ti := range m.tabs {
		t := setupTabs[ti]
		on, total := 0, 0
		for _, it := range m.s.items {
			if it.category == t.category {
				total++
				if it.on {
					on++
				}
			}
		}
		label := t.title
		if on > 0 {
			label = fmt.Sprintf("%s (%d)", t.title, on)
		}
		if i == m.tab {
			parts = append(parts, stTabOn.Render(label))
		} else {
			parts = append(parts, stTab.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m *setupTUI) renderList(width int) string {
	vis := m.visible()
	if len(vis) == 0 {
		return stDim.Render("nothing matches")
	}
	nameW := 4
	for _, i := range vis {
		nameW = max(nameW, len(m.s.items[i].label))
	}
	nameW = min(nameW, width/2)
	rows := m.listRows()
	var lines []string
	for row := m.offset; row < len(vis) && row < m.offset+rows; row++ {
		lines = append(lines, m.renderRow(m.s.items[vis[row]], row == m.cursor, nameW, width))
	}
	if more := len(vis) - (m.offset + rows); more > 0 {
		lines = append(lines, stDim.Render(fmt.Sprintf("  … %d more", more)))
	}
	return strings.Join(lines, "\n")
}

func (m *setupTUI) renderRow(it setupItem, isCursor bool, nameW, width int) string {
	pointer, box, name := "  ", "[ ]", fmt.Sprintf("%-*s", nameW, ansi.Truncate(it.label, nameW, "…"))
	if it.on {
		box = stChecked.Render("[x]")
	}
	if isCursor {
		pointer, name = stCursor.Render("›")+" ", stCursor.Render(name)
	}
	line := pointer + box + " " + name
	if it.note != "" {
		line += " " + stNote.Render(it.note)
	}
	if it.prof != nil {
		line += " " + stDim.Render(it.desc)
	}
	return ansi.Truncate(line, width, "…")
}

// renderDetails explains the focused item: what checking it grants.
func (m *setupTUI) renderDetails(width int) string {
	it := m.current()
	if it == nil {
		return ""
	}
	lines := []string{stHeading.Render(it.label), ""}
	lines = append(lines, wrapLines(it.desc, width)...)
	if p := it.prof; p != nil {
		lines = append(lines, "")
		lines = appendField(lines, "Needs", p.Requires, width)
		lines = appendField(lines, "Read-write", bindHosts(p.BindsRW), width)
		lines = appendField(lines, "Read-only", bindHosts(p.BindsRO), width)
		lines = appendField(lines, "Hides", p.Mask, width)
		lines = appendField(lines, "Passes env", p.PassEnv, width)
		if bin := profileBinary(p); bin != "" {
			status := "installed"
			if it.note == "not installed" {
				status = "not installed"
			}
			lines = append(lines, fmt.Sprintf("%s %s (%s)", stDim.Render("Program:"), bin, status))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *setupTUI) renderSummary() string {
	names := m.s.checked(false)
	sel := "nothing selected"
	if len(names) > 0 {
		sel = strings.Join(names, ", ")
	}
	onOff := func(key, label string) string {
		if m.s.isOn(key) {
			return label + " on"
		}
		return label + " off"
	}
	return stSelected.Render("Selected: "+sel) + stDim.Render(" · "+onOff("network", "network")+
		" · "+onOff("push", "push")+" · "+onOff("gui", "GUI"))
}

func appendField(lines []string, label string, values []string, width int) []string {
	if len(values) == 0 {
		return lines
	}
	const maxShown = 6
	shown := values
	if len(shown) > maxShown {
		shown = append(append([]string{}, shown[:maxShown]...), fmt.Sprintf("+%d more", len(values)-maxShown))
	}
	text := stDim.Render(label+":") + " " + strings.Join(shown, ", ")
	return append(lines, wrapLines(text, width)...)
}

func bindHosts(binds [][]string) []string {
	var out []string
	for _, b := range binds {
		if len(b) > 0 {
			out = append(out, b[0])
		}
	}
	return out
}

func wrapLines(s string, width int) []string {
	if s == "" {
		return nil
	}
	return strings.Split(lipgloss.NewStyle().Width(max(10, width)).Render(s), "\n")
}

// capLines keeps at most n lines of s.
func capLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if n < 1 {
		return ""
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
