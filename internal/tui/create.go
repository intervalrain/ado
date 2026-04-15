package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cache"
)

var (
	labelStyle   = lipgloss.NewStyle().Bold(true).Width(20)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

var typeOptions = []string{"Task", "Bug", "Epic", "Issue", "User Story"}

type createStep int

const (
	stepType createStep = iota
	stepTags
	stepTitle
	stepDescription
	stepEstimate
	stepParent
	stepConfirm
	stepDone
)

type workItemCreatedMsg struct {
	id    int
	title string
	typ   string
}

type createModel struct {
	client *api.Client
	cache  *cache.Cache
	step   createStep

	// Type selection
	typeIdx int

	// Tags selection (multi-select from cache + free input)
	cachedTags  []string
	tagSelected []bool
	tagCursor   int
	tagInput    textinput.Model
	tagAdding   bool // true = typing a new tag

	// Text inputs
	titleInput  textinput.Model
	descInput   textinput.Model
	estInput    textinput.Model
	parentInput textinput.Model

	// Result
	resultMsg string
	err       error
}

func newCreateModel(client *api.Client) createModel {
	c := cache.Load()

	ti := textinput.New()
	ti.Placeholder = "Work item title..."
	ti.CharLimit = 256
	ti.Width = 60

	di := textinput.New()
	di.Placeholder = "Description (optional, press Enter to skip)..."
	di.CharLimit = 512
	di.Width = 60

	ei := textinput.New()
	ei.Placeholder = "6"
	ei.CharLimit = 10
	ei.Width = 10

	tagi := textinput.New()
	tagi.Placeholder = "New tag name..."
	tagi.CharLimit = 50
	tagi.Width = 40

	pi := textinput.New()
	pi.Placeholder = "Parent work item ID (optional, enter to skip)"
	pi.CharLimit = 10
	pi.Width = 20

	return createModel{
		client:      client,
		cache:       c,
		step:        stepType,
		typeIdx:     0,
		cachedTags:  c.Tags,
		tagSelected: make([]bool, len(c.Tags)),
		titleInput:  ti,
		descInput:   di,
		estInput:    ei,
		parentInput: pi,
		tagInput:    tagi,
	}
}

func (m createModel) selectedTags() string {
	var tags []string
	for i, sel := range m.tagSelected {
		if sel {
			tags = append(tags, m.cachedTags[i])
		}
	}
	return strings.Join(tags, "; ")
}

func (m createModel) update(msg tea.Msg) (createModel, tea.Cmd) {
	switch msg := msg.(type) {
	case workItemCreatedMsg:
		m.step = stepDone
		m.resultMsg = fmt.Sprintf("Created %s #%d: %s", msg.typ, msg.id, msg.title)
		return m, nil
	case errMsg:
		m.step = stepDone
		m.err = msg.err
		return m, nil
	}

	switch m.step {
	case stepType:
		return m.updateType(msg)
	case stepTags:
		return m.updateTags(msg)
	case stepTitle:
		return m.updateTitle(msg)
	case stepDescription:
		return m.updateDesc(msg)
	case stepEstimate:
		return m.updateEstimate(msg)
	case stepParent:
		return m.updateParent(msg)
	case stepConfirm:
		return m.updateConfirm(msg)
	}
	return m, nil
}

func (m createModel) updateType(msg tea.Msg) (createModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "k":
			if m.typeIdx > 0 {
				m.typeIdx--
			}
		case "down", "j":
			if m.typeIdx < len(typeOptions)-1 {
				m.typeIdx++
			}
		case "enter":
			m.step = stepTags
			return m, nil
		}
	}
	return m, nil
}

func (m createModel) updateTags(msg tea.Msg) (createModel, tea.Cmd) {
	// If adding a new tag via text input
	if m.tagAdding {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				newTag := strings.TrimSpace(m.tagInput.Value())
				if newTag != "" {
					m.cachedTags = append(m.cachedTags, newTag)
					m.tagSelected = append(m.tagSelected, true)
					m.cache.AddTags([]string{newTag})
					_ = m.cache.Save()
				}
				m.tagInput.SetValue("")
				m.tagInput.Blur()
				m.tagAdding = false
				return m, nil
			case "esc":
				m.tagInput.SetValue("")
				m.tagInput.Blur()
				m.tagAdding = false
				return m, nil
			default:
				var cmd tea.Cmd
				m.tagInput, cmd = m.tagInput.Update(msg)
				return m, cmd
			}
		}
		var cmd tea.Cmd
		m.tagInput, cmd = m.tagInput.Update(msg)
		return m, cmd
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "k":
			if m.tagCursor > 0 {
				m.tagCursor--
			}
		case "down", "j":
			if m.tagCursor < len(m.cachedTags)-1 {
				m.tagCursor++
			}
		case " ":
			if len(m.cachedTags) > 0 {
				m.tagSelected[m.tagCursor] = !m.tagSelected[m.tagCursor]
			}
		case "a":
			m.tagAdding = true
			m.tagInput.Focus()
			return m, m.tagInput.Cursor.BlinkCmd()
		case "enter":
			m.step = stepTitle
			m.titleInput.Focus()
			return m, m.titleInput.Cursor.BlinkCmd()
		case "esc":
			m.step = stepType
			return m, nil
		}
	}
	return m, nil
}

func (m createModel) updateTitle(msg tea.Msg) (createModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if strings.TrimSpace(m.titleInput.Value()) == "" {
				return m, nil
			}
			m.titleInput.Blur()
			m.step = stepDescription
			m.descInput.Focus()
			return m, m.descInput.Cursor.BlinkCmd()
		case "esc":
			m.titleInput.Blur()
			m.step = stepTags
			return m, nil
		default:
			var cmd tea.Cmd
			m.titleInput, cmd = m.titleInput.Update(msg)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.titleInput, cmd = m.titleInput.Update(msg)
	return m, cmd
}

func (m createModel) updateDesc(msg tea.Msg) (createModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			m.descInput.Blur()
			m.step = stepEstimate
			m.estInput.Focus()
			return m, m.estInput.Cursor.BlinkCmd()
		case "esc":
			m.descInput.Blur()
			m.step = stepTitle
			m.titleInput.Focus()
			return m, m.titleInput.Cursor.BlinkCmd()
		default:
			var cmd tea.Cmd
			m.descInput, cmd = m.descInput.Update(msg)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.descInput, cmd = m.descInput.Update(msg)
	return m, cmd
}

func (m createModel) updateEstimate(msg tea.Msg) (createModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			m.estInput.Blur()
			m.step = stepParent
			m.parentInput.Focus()
			return m, m.parentInput.Cursor.BlinkCmd()
		case "esc":
			m.estInput.Blur()
			m.step = stepDescription
			m.descInput.Focus()
			return m, m.descInput.Cursor.BlinkCmd()
		default:
			var cmd tea.Cmd
			m.estInput, cmd = m.estInput.Update(msg)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.estInput, cmd = m.estInput.Update(msg)
	return m, cmd
}

func (m createModel) updateParent(msg tea.Msg) (createModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			v := strings.TrimSpace(m.parentInput.Value())
			if v != "" {
				if _, err := strconv.Atoi(v); err != nil {
					return m, nil
				}
			}
			m.parentInput.Blur()
			m.step = stepConfirm
			return m, nil
		case "esc":
			m.parentInput.Blur()
			m.step = stepEstimate
			m.estInput.Focus()
			return m, m.estInput.Cursor.BlinkCmd()
		default:
			var cmd tea.Cmd
			m.parentInput, cmd = m.parentInput.Update(msg)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.parentInput, cmd = m.parentInput.Update(msg)
	return m, cmd
}

func (m createModel) updateConfirm(msg tea.Msg) (createModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "y", "Y", "enter":
			return m, m.submitWorkItem()
		case "n", "N", "esc":
			m.step = stepType
			return m, nil
		}
	}
	return m, nil
}

func (m createModel) submitWorkItem() tea.Cmd {
	client := m.client
	wiType := typeOptions[m.typeIdx]
	title := m.titleInput.Value()
	desc := m.descInput.Value()
	estStr := m.estInput.Value()
	tags := m.selectedTags()
	parentStr := strings.TrimSpace(m.parentInput.Value())

	return func() tea.Msg {
		ops := []api.PatchOp{
			{Op: "add", Path: "/fields/System.Title", Value: title},
		}

		if tags != "" {
			ops = append(ops, api.PatchOp{Op: "add", Path: "/fields/System.Tags", Value: tags})
		}

		if desc != "" {
			ops = append(ops, api.PatchOp{Op: "add", Path: "/fields/System.Description", Value: desc})
		}

		est := 6.0
		if estStr != "" {
			v, err := strconv.ParseFloat(estStr, 64)
			if err != nil {
				return errMsg{fmt.Errorf("invalid estimate: %s", estStr)}
			}
			est = v
		}
		ops = append(ops,
			api.PatchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate", Value: est},
			api.PatchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.RemainingWork", Value: est},
		)

		if iterPath, err := client.GetCurrentIteration(); err == nil {
			ops = append(ops, api.PatchOp{Op: "add", Path: "/fields/System.IterationPath", Value: iterPath})
		}

		if client.Config().Assignee != "" {
			ops = append(ops, api.PatchOp{Op: "add", Path: "/fields/System.AssignedTo", Value: client.Config().Assignee})
		}

		if parentStr != "" {
			parentID, err := strconv.Atoi(parentStr)
			if err != nil {
				return errMsg{fmt.Errorf("invalid parent ID: %s", parentStr)}
			}
			parentURL := fmt.Sprintf("%s/%s/_apis/wit/workItems/%d",
				client.BaseURL(), client.Project(), parentID)
			ops = append(ops, api.PatchOp{
				Op:   "add",
				Path: "/relations/-",
				Value: map[string]string{
					"rel": "System.LinkTypes.Hierarchy-Reverse",
					"url": parentURL,
				},
			})
		}

		wi, err := client.CreateWorkItem(wiType, ops)
		if err != nil {
			return errMsg{err}
		}
		return workItemCreatedMsg{id: wi.ID, title: wi.Fields.Title, typ: wiType}
	}
}

func (m createModel) view() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Create Work Item"))
	b.WriteString("\n\n")

	steps := []struct {
		label string
		done  bool
	}{
		{"Type", m.step > stepType},
		{"Tags", m.step > stepTags},
		{"Title", m.step > stepTitle},
		{"Description", m.step > stepDescription},
		{"Estimate", m.step > stepEstimate},
		{"Parent", m.step > stepParent},
		{"Confirm", m.step > stepConfirm},
	}

	// Progress indicator
	for i, s := range steps {
		marker := "○"
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		if s.done {
			marker = "●"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		} else if int(m.step) == i {
			marker = "◉"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
		}
		b.WriteString(style.Render(fmt.Sprintf("  %s %s", marker, s.label)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	switch m.step {
	case stepType:
		b.WriteString(labelStyle.Render("  Select type:"))
		b.WriteString("\n")
		for i, opt := range typeOptions {
			if i == m.typeIdx {
				b.WriteString(selectedStyle.Render("  > " + opt))
			} else {
				b.WriteString(itemStyle.Render("    " + opt))
			}
			b.WriteString("\n")
		}
		b.WriteString(helpStyle.Render("\n  ↑↓: select  enter: next  esc: back"))

	case stepTags:
		fmt.Fprintf(&b, "  Type: %s\n\n", typeOptions[m.typeIdx])
		b.WriteString(labelStyle.Render("  Tags:"))
		b.WriteString("\n")
		if len(m.cachedTags) == 0 {
			b.WriteString(itemStyle.Render("    (no cached tags)"))
			b.WriteString("\n")
		} else {
			for i, tag := range m.cachedTags {
				check := "[ ]"
				if m.tagSelected[i] {
					check = "[x]"
				}
				line := fmt.Sprintf("  %s %s", check, tag)
				if i == m.tagCursor {
					b.WriteString(selectedStyle.Render("> " + line))
				} else {
					b.WriteString(itemStyle.Render("  " + line))
				}
				b.WriteString("\n")
			}
		}
		if m.tagAdding {
			fmt.Fprintf(&b, "\n  New tag: %s\n", m.tagInput.View())
			b.WriteString(helpStyle.Render("  enter: add  esc: cancel"))
		} else {
			b.WriteString(helpStyle.Render("\n  ↑↓: navigate  space: toggle  a: add new  enter: next  esc: back"))
		}

	case stepTitle:
		fmt.Fprintf(&b, "  Type: %s\n", typeOptions[m.typeIdx])
		tags := m.selectedTags()
		if tags == "" {
			tags = "(none)"
		}
		fmt.Fprintf(&b, "  Tags: %s\n\n", tags)
		b.WriteString(labelStyle.Render("  Title:"))
		b.WriteString(" " + m.titleInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next  esc: back"))

	case stepDescription:
		fmt.Fprintf(&b, "  Type: %s\n", typeOptions[m.typeIdx])
		tags := m.selectedTags()
		if tags == "" {
			tags = "(none)"
		}
		fmt.Fprintf(&b, "  Tags: %s\n", tags)
		fmt.Fprintf(&b, "  Title: %s\n\n", m.titleInput.Value())
		b.WriteString(labelStyle.Render("  Description:"))
		b.WriteString(" " + m.descInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next (empty to skip)  esc: back"))

	case stepEstimate:
		fmt.Fprintf(&b, "  Type: %s\n", typeOptions[m.typeIdx])
		tags := m.selectedTags()
		if tags == "" {
			tags = "(none)"
		}
		fmt.Fprintf(&b, "  Tags: %s\n", tags)
		fmt.Fprintf(&b, "  Title: %s\n", m.titleInput.Value())
		desc := m.descInput.Value()
		if desc == "" {
			desc = "(none)"
		}
		fmt.Fprintf(&b, "  Description: %s\n\n", desc)
		b.WriteString(labelStyle.Render("  Estimate (hours):"))
		b.WriteString(" " + m.estInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next (default: 6)  esc: back"))

	case stepParent:
		fmt.Fprintf(&b, "  Type: %s\n", typeOptions[m.typeIdx])
		tags := m.selectedTags()
		if tags == "" {
			tags = "(none)"
		}
		fmt.Fprintf(&b, "  Tags: %s\n", tags)
		fmt.Fprintf(&b, "  Title: %s\n", m.titleInput.Value())
		desc := m.descInput.Value()
		if desc == "" {
			desc = "(none)"
		}
		fmt.Fprintf(&b, "  Description: %s\n", desc)
		est := m.estInput.Value()
		if est == "" {
			est = "6"
		}
		fmt.Fprintf(&b, "  Estimate: %s hours\n\n", est)
		b.WriteString(labelStyle.Render("  Parent ID:"))
		b.WriteString(" " + m.parentInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next (empty to skip)  esc: back"))

	case stepConfirm:
		b.WriteString("  Summary:\n\n")
		fmt.Fprintf(&b, "    Type:        %s\n", typeOptions[m.typeIdx])
		tags := m.selectedTags()
		if tags == "" {
			tags = "(none)"
		}
		fmt.Fprintf(&b, "    Tags:        %s\n", tags)
		fmt.Fprintf(&b, "    Title:       %s\n", m.titleInput.Value())
		desc := m.descInput.Value()
		if desc == "" {
			desc = "(none)"
		}
		fmt.Fprintf(&b, "    Description: %s\n", desc)
		est := m.estInput.Value()
		if est == "" {
			est = "6"
		}
		fmt.Fprintf(&b, "    Estimate:    %s hours\n", est)
		parent := strings.TrimSpace(m.parentInput.Value())
		if parent == "" {
			parent = "(none)"
		} else {
			parent = "#" + parent
		}
		fmt.Fprintf(&b, "    Parent:      %s\n", parent)
		if m.client.Config().Assignee != "" {
			fmt.Fprintf(&b, "    Assigned To: %s\n", m.client.Config().Assignee)
		}
		b.WriteString(helpStyle.Render("\n  y/enter: create  n/esc: start over"))

	case stepDone:
		if m.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		} else {
			b.WriteString(successStyle.Render("  " + m.resultMsg))
		}
		b.WriteString(helpStyle.Render("\n\n  esc: back to menu"))
	}

	return b.String()
}
