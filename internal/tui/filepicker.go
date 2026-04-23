package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dirEntry struct {
	name  string
	isDir bool
	isGit bool // directory contains a .git entry (file or dir)
}

type dirPickerModel struct {
	dir          string // current directory being browsed
	entries      []dirEntry
	filtered     []dirEntry // entries after applying filter
	cursor       int
	err          error
	height       int // max visible rows
	offset       int // scroll offset
	filter       textinput.Model
	filtering    bool              // true when filter input is focused
	multiSelect  bool              // true for multi-select mode
	selected     map[string]bool   // selected paths in multi-select mode
	onlyGitRepos bool              // if true, only .git-containing dirs are selectable
	notice       string            // transient inline message (e.g. rejection)
}

type dirSelectedMsg struct {
	path  string
	paths []string // multi-select results
}

type dirCancelledMsg struct{}

// newMultiGitRepoPicker is a multi-select picker that only accepts directories
// containing a .git entry (file or dir).
func newMultiGitRepoPicker(startDir string) dirPickerModel {
	return newDirPickerWithMode(startDir, true, true)
}

// newGitRepoPicker is a single-select picker constrained to git repos.
func newGitRepoPicker(startDir string) dirPickerModel {
	return newDirPickerWithMode(startDir, false, true)
}

func newDirPickerWithMode(startDir string, multiSelect, onlyGitRepos bool) dirPickerModel {
	if startDir == "" {
		startDir, _ = os.UserHomeDir()
	}
	// Resolve to absolute
	startDir, _ = filepath.Abs(startDir)

	fi := textinput.New()
	fi.Placeholder = "type to filter..."
	fi.CharLimit = 128
	fi.Width = 40

	m := dirPickerModel{
		dir:          startDir,
		height:       12,
		filter:       fi,
		multiSelect:  multiSelect,
		onlyGitRepos: onlyGitRepos,
		selected:     make(map[string]bool),
	}
	m.loadDir()
	return m
}

// isGitRepo reports whether dir contains a .git entry (dir or file).
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func (m *dirPickerModel) loadDir() {
	m.entries = nil
	m.cursor = 0
	m.offset = 0
	m.err = nil
	m.filter.SetValue("")
	m.filtering = false
	m.filter.Blur()

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		m.err = err
		return
	}

	// Add parent directory entry
	if m.dir != "/" {
		m.entries = append(m.entries, dirEntry{name: "..", isDir: true})
	}

	// Collect directories only
	var dirs []dirEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // skip hidden
		}
		if e.IsDir() {
			full := filepath.Join(m.dir, e.Name())
			dirs = append(dirs, dirEntry{name: e.Name(), isDir: true, isGit: isGitRepo(full)})
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].name < dirs[j].name
	})
	m.entries = append(m.entries, dirs...)
	m.applyFilter()
}

func (m *dirPickerModel) applyFilter() {
	query := strings.ToLower(m.filter.Value())
	if query == "" {
		m.filtered = m.entries
	} else {
		m.filtered = nil
		// Always keep ".." visible
		for _, e := range m.entries {
			if e.name == ".." || strings.Contains(strings.ToLower(e.name), query) {
				m.filtered = append(m.filtered, e)
			}
		}
	}
	// Reset cursor to stay in bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
}

func (m dirPickerModel) Update(msg tea.Msg) (dirPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When filtering is active, handle special keys first
		if m.filtering {
			switch msg.String() {
			case "esc":
				// If filter has text, clear it; otherwise exit filter mode
				if m.filter.Value() != "" {
					m.filter.SetValue("")
					m.applyFilter()
				} else {
					m.filtering = false
					m.filter.Blur()
				}
				return m, nil
			case "enter":
				// Accept filter and switch to navigation
				m.filtering = false
				m.filter.Blur()
				return m, nil
			case "up", "down":
				// Allow navigation while filtering
				m.filtering = false
				m.filter.Blur()
				return m.handleNav(msg.String())
			default:
				var cmd tea.Cmd
				m.filter, cmd = m.filter.Update(msg)
				m.applyFilter()
				return m, cmd
			}
		}

		// Normal navigation mode
		switch msg.String() {
		case "up":
			return m.handleNav("up")
		case "down":
			return m.handleNav("down")
		case " ":
			if m.multiSelect {
				// Toggle selection of current directory or the entry under cursor.
				path := m.dir
				if len(m.filtered) > 0 {
					e := m.filtered[m.cursor]
					if e.name != ".." {
						path = filepath.Join(m.dir, e.name)
					}
				}
				if m.onlyGitRepos && !isGitRepo(path) {
					m.notice = "✗ not a git repo — only directories containing .git can be selected"
					return m, nil
				}
				m.notice = ""
				if m.selected[path] {
					delete(m.selected, path)
				} else {
					m.selected[path] = true
				}
				return m, nil
			}
		case "enter":
			if m.multiSelect {
				// Submit all selected paths
				if len(m.selected) > 0 {
					paths := make([]string, 0, len(m.selected))
					for p := range m.selected {
						paths = append(paths, p)
					}
					sort.Strings(paths)
					return m, func() tea.Msg {
						return dirSelectedMsg{paths: paths}
					}
				}
				return m, nil
			}
			// Single-select: enter opens directory
			fallthrough
		case "right":
			if len(m.filtered) == 0 {
				break
			}
			e := m.filtered[m.cursor]
			if e.isDir {
				var target string
				if e.name == ".." {
					target = filepath.Dir(m.dir)
				} else {
					target = filepath.Join(m.dir, e.name)
				}
				m.dir = target
				m.loadDir()
			}
		case "left":
			// Go to parent
			if m.dir != "/" {
				m.dir = filepath.Dir(m.dir)
				m.loadDir()
			}
		case "tab":
			if !m.multiSelect {
				if m.onlyGitRepos && !isGitRepo(m.dir) {
					m.notice = "✗ not a git repo — enter a directory containing .git"
					return m, nil
				}
				m.notice = ""
				return m, func() tea.Msg {
					return dirSelectedMsg{path: m.dir}
				}
			}
		case "esc":
			return m, func() tea.Msg {
				return dirCancelledMsg{}
			}
		default:
			// Any printable key starts filtering
			r := msg.Runes
			if len(r) > 0 {
				m.filtering = true
				m.filter.Focus()
				// Forward this keystroke into the filter input
				var cmd tea.Cmd
				m.filter, cmd = m.filter.Update(msg)
				m.applyFilter()
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m dirPickerModel) handleNav(dir string) (dirPickerModel, tea.Cmd) {
	m.notice = ""
	switch dir {
	case "up":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			if m.cursor >= m.offset+m.height {
				m.offset = m.cursor - m.height + 1
			}
		}
	}
	return m, nil
}

func (m dirPickerModel) View() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	checkedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	filterLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	gitBadgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	noticeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	header := "  Browse: " + m.dir
	if m.multiSelect && len(m.selected) > 0 {
		header += fmt.Sprintf("  (%d selected)", len(m.selected))
	}
	b.WriteString(headerStyle.Render(header) + "\n")

	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(
			"  Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	// Filter input line
	if m.filtering {
		b.WriteString(filterLabelStyle.Render("  Filter: ") + m.filter.View() + "\n")
	} else if m.filter.Value() != "" {
		b.WriteString(filterLabelStyle.Render("  Filter: "+m.filter.Value()) +
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
				fmt.Sprintf("  (%d/%d)", len(m.filtered), len(m.entries))) + "\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString("    (no matching directories)\n")
	}

	// Visible window
	end := min(m.offset+m.height, len(m.filtered))

	for i := m.offset; i < end; i++ {
		e := m.filtered[i]
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		name := e.name
		if e.isDir && e.name != ".." {
			name += "/"
		}

		// In multi-select mode, show checkbox
		checkbox := ""
		if m.multiSelect && e.name != ".." {
			fullPath := filepath.Join(m.dir, e.name)
			if m.selected[fullPath] {
				checkbox = "[x] "
			} else {
				checkbox = "[ ] "
			}
		}

		line := "    " + cursor + checkbox + name
		var rendered string
		if i == m.cursor {
			if m.multiSelect && checkbox != "" && m.selected[filepath.Join(m.dir, e.name)] {
				rendered = checkedStyle.Render(line)
			} else {
				rendered = cursorStyle.Render(line)
			}
		} else if m.multiSelect && checkbox != "" && m.selected[filepath.Join(m.dir, e.name)] {
			rendered = checkedStyle.Render(line)
		} else {
			rendered = dirStyle.Render(line)
		}
		if e.isGit && e.name != ".." {
			rendered += "  " + gitBadgeStyle.Render("git")
		}
		b.WriteString(rendered + "\n")
	}

	// Scroll indicators
	if m.offset > 0 {
		b.WriteString("    " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ...more above") + "\n")
	}
	if end < len(m.filtered) {
		b.WriteString("    " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ...more below") + "\n")
	}

	if m.notice != "" {
		b.WriteString("\n  " + noticeStyle.Render(m.notice))
	}
	gitHint := ""
	if m.onlyGitRepos {
		gitHint = "  (only git repos)"
	}
	if m.multiSelect {
		b.WriteString(helpStyle.Render("\n  ↑↓: navigate  ←→: open  space: select  enter: submit  type: filter  esc: cancel" + gitHint))
	} else {
		b.WriteString(helpStyle.Render("\n  ↑↓: navigate  ←→/enter: open  tab: select  type: filter  esc: cancel" + gitHint))
	}
	return b.String()
}

