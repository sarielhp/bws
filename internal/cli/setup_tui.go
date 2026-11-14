package cli

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const (
	setupHelp      = "←→ group · ↑↓ move · space toggle · / search · enter save & start · q quit"
	setupHelpTyped = "type to search · ↑↓ move · space toggle · enter done · esc clear"
	sideBySideMin  = 100 // terminal width at which details sit beside the list
)

var (
	stTitle    = lipgloss.NewStyle().Bold(true)
	stTab      = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("245"))
	stTabOn    = lipgloss.NewStyle().Padding(0, 1).Bold(true).Reverse(true)
	stDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	stCursor   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	stChecked  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	stNote     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	stBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	stHeading  = lipgloss.NewStyle().Bold(true).Underline(true)
	stSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// setupTUI is the bubbletea model for the setup screen.
type setupTUI struct {
	s         *setupState
	title     string
	tabs      []int // indexes into setupTabs that have items
	tab       int
	cursor    int
	offset    int
	filter    string
	filtering bool
	width     int
	height    int
	done      bool
	save      bool
}

func newSetupTUI(title string, s *setupState) *setupTUI {
	m := &setupTUI{s: s, title: title, width: 80, height: 24}
	for i, t := range setupTabs {
		for _, it := range s.items {
			if it.category == t.category {
				m.tabs = append(m.tabs, i)
				break
			}
		}
	}
	return m
}

// runSetupTUI runs the screen on the terminal. It returns ErrSetupCancelled
// when the user quits without saving.
func runSetupTUI(title string, s *setupState) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return fmt.Errorf("bws setup needs a terminal; use --with to choose without one")
	}
	m := newSetupTUI(title, s)
	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(os.Stderr)).Run(); err != nil {
		return err
	}
	if !m.save {
		return ErrSetupCancelled
	}
	return nil
}

func (m *setupTUI) Init() tea.Cmd { return nil }

func (m *setupTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if m.filtering {
			m.updateFilter(msg)
		} else {
			m.updateNormal(msg)
		}
	}
	m.clampCursor()
	if m.done {
		return m, tea.Quit
	}
	return m, nil
}

func (m *setupTUI) updateNormal(msg tea.KeyMsg) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.done = true
	case "esc":
		if m.filter != "" {
			m.filter = ""
		} else {
			m.done = true
		}
	case "enter":
		m.done, m.save = true, true
	case "left", "h", "shift+tab":
		m.switchTab(-1)
	case "right", "l", "tab":
		m.switchTab(1)
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.visible()) - 1
	case " ", "x":
		m.toggleCurrent()
	case "/":
		m.filtering = true
	}
}

func (m *setupTUI) updateFilter(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.done = true
	case tea.KeyEsc:
		m.filter, m.filtering = "", false
	case tea.KeyEnter:
		m.filtering = false
	case tea.KeyBackspace:
		if r := []rune(m.filter); len(r) > 0 {
			m.filter = string(r[:len(r)-1])
		}
	case tea.KeyUp:
		m.cursor--
	case tea.KeyDown:
		m.cursor++
	case tea.KeySpace:
		m.toggleCurrent()
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		m.cursor = 0
	}
}

func (m *setupTUI) switchTab(delta int) {
	if m.filter != "" || len(m.tabs) == 0 {
		return
	}
	m.tab = (m.tab + delta + len(m.tabs)) % len(m.tabs)
	m.cursor, m.offset = 0, 0
}

func (m *setupTUI) toggleCurrent() {
	if it := m.current(); it != nil {
		m.s.toggle(it.key)
	}
}

// visible returns indexes of the items shown: the current tab, or every
// match when a search is active (name matches before description matches).
func (m *setupTUI) visible() []int {
	var byName, byDesc []int
	needle := strings.ToLower(m.filter)
	for i, it := range m.s.items {
		switch {
		case needle == "":
			if len(m.tabs) > 0 && it.category == setupTabs[m.tabs[m.tab]].category {
				byName = append(byName, i)
			}
		case strings.Contains(strings.ToLower(it.key+" "+it.label), needle):
			byName = append(byName, i)
		case strings.Contains(strings.ToLower(it.desc), needle):
			byDesc = append(byDesc, i)
		}
	}
	return append(byName, byDesc...)
}

func (m *setupTUI) current() *setupItem {
	vis := m.visible()
	if m.cursor < 0 || m.cursor >= len(vis) {
		return nil
	}
	return &m.s.items[vis[m.cursor]]
}

func (m *setupTUI) clampCursor() {
	n := len(m.visible())
	m.cursor = max(0, min(m.cursor, n-1))
	rows := m.listRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	m.offset = max(0, min(m.offset, max(0, n-rows)))
}

// listRows is the number of list lines that fit.
func (m *setupTUI) listRows() int {
	avail := m.height - 7 // title, tabs, two footer lines, borders, spacing
	if m.width < sideBySideMin {
		avail = avail * 3 / 5
	}
	return max(3, avail)
}
