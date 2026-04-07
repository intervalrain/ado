package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/git"
)

type prCreateStep int

const (
	prStepTitle prCreateStep = iota
	prStepDesc
	prStepTarget
	prStepReviewer
	prStepOptReviewer
	prStepAutoComplete
	prStepConfirm
	prStepDone
)

type prCreatedMsg struct {
	pr  *api.PullRequest
	url string
}

type prCreateModel struct {
	client *api.Client
	repoID string
	repoName string
	step   prCreateStep

	srcBranch string

	titleInput    textinput.Model
	descInput     textinput.Model
	targetInput   textinput.Model
	reviewerInput textinput.Model
	optRevInput   textinput.Model
	autoComplete  bool

	resultMsg string
	err       error
}

func newPRCreateModel(client *api.Client, repoID, repoName string) prCreateModel {
	srcBranch, _ := git.CurrentBranch()
	defaultTarget := git.DefaultBranch()

	ti := textinput.New()
	ti.Placeholder = "PR title..."
	ti.CharLimit = 256
	ti.Width = 60

	di := textinput.New()
	di.Placeholder = "Description (optional, Enter to skip)..."
	di.CharLimit = 512
	di.Width = 60

	tgi := textinput.New()
	tgi.SetValue(defaultTarget)
	tgi.CharLimit = 100
	tgi.Width = 40

	ri := textinput.New()
	ri.Placeholder = "Required reviewer name (Enter to skip)..."
	ri.CharLimit = 100
	ri.Width = 40

	oi := textinput.New()
	oi.Placeholder = "Optional reviewer name (Enter to skip)..."
	oi.CharLimit = 100
	oi.Width = 40

	m := prCreateModel{
		client:        client,
		repoID:        repoID,
		repoName:      repoName,
		step:          prStepTitle,
		srcBranch:     srcBranch,
		titleInput:    ti,
		descInput:     di,
		targetInput:   tgi,
		reviewerInput: ri,
		optRevInput:   oi,
	}
	m.titleInput.Focus()
	return m
}

func (m prCreateModel) update(msg tea.Msg) (prCreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case prCreatedMsg:
		m.step = prStepDone
		m.resultMsg = fmt.Sprintf("Created PR #%d: %s\n%s", msg.pr.ID, msg.pr.Title, msg.url)
		return m, nil
	case errMsg:
		m.step = prStepDone
		m.err = msg.err
		return m, nil
	}

	switch m.step {
	case prStepTitle:
		return m.updateTextStep(msg, &m.titleInput, prStepDesc, func() tea.Cmd {
			m.descInput.Focus()
			return m.descInput.Cursor.BlinkCmd()
		})
	case prStepDesc:
		return m.updateTextStep(msg, &m.descInput, prStepTarget, func() tea.Cmd {
			m.targetInput.Focus()
			return m.targetInput.Cursor.BlinkCmd()
		})
	case prStepTarget:
		return m.updateTextStep(msg, &m.targetInput, prStepReviewer, func() tea.Cmd {
			m.reviewerInput.Focus()
			return m.reviewerInput.Cursor.BlinkCmd()
		})
	case prStepReviewer:
		return m.updateTextStep(msg, &m.reviewerInput, prStepOptReviewer, func() tea.Cmd {
			m.optRevInput.Focus()
			return m.optRevInput.Cursor.BlinkCmd()
		})
	case prStepOptReviewer:
		return m.updateTextStep(msg, &m.optRevInput, prStepAutoComplete, nil)
	case prStepAutoComplete:
		return m.updateAutoComplete(msg)
	case prStepConfirm:
		return m.updateConfirm(msg)
	}
	return m, nil
}

func (m prCreateModel) updateTextStep(msg tea.Msg, input *textinput.Model, nextStep prCreateStep, onNext func() tea.Cmd) (prCreateModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			// Title is required
			if m.step == prStepTitle && strings.TrimSpace(input.Value()) == "" {
				return m, nil
			}
			// Target branch is required
			if m.step == prStepTarget && strings.TrimSpace(input.Value()) == "" {
				return m, nil
			}
			input.Blur()
			m.step = nextStep
			if onNext != nil {
				return m, onNext()
			}
			return m, nil
		case "esc":
			input.Blur()
			if m.step == prStepTitle {
				return m, nil // parent handles esc at first step
			}
			m.step--
			return m, m.focusCurrentStep()
		default:
			var cmd tea.Cmd
			*input, cmd = input.Update(msg)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	return m, cmd
}

func (m prCreateModel) updateAutoComplete(msg tea.Msg) (prCreateModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "y", "Y":
			m.autoComplete = true
			m.step = prStepConfirm
		case "n", "N", "enter":
			m.step = prStepConfirm
		case "esc":
			m.step = prStepOptReviewer
			m.optRevInput.Focus()
			return m, m.optRevInput.Cursor.BlinkCmd()
		}
	}
	return m, nil
}

func (m prCreateModel) updateConfirm(msg tea.Msg) (prCreateModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "y", "Y", "enter":
			return m, m.submit()
		case "n", "N", "esc":
			m.step = prStepTitle
			m.titleInput.Focus()
			return m, m.titleInput.Cursor.BlinkCmd()
		}
	}
	return m, nil
}

func (m prCreateModel) focusCurrentStep() tea.Cmd {
	switch m.step {
	case prStepTitle:
		m.titleInput.Focus()
		return m.titleInput.Cursor.BlinkCmd()
	case prStepDesc:
		m.descInput.Focus()
		return m.descInput.Cursor.BlinkCmd()
	case prStepTarget:
		m.targetInput.Focus()
		return m.targetInput.Cursor.BlinkCmd()
	case prStepReviewer:
		m.reviewerInput.Focus()
		return m.reviewerInput.Cursor.BlinkCmd()
	case prStepOptReviewer:
		m.optRevInput.Focus()
		return m.optRevInput.Cursor.BlinkCmd()
	}
	return nil
}

func (m prCreateModel) submit() tea.Cmd {
	client := m.client
	repoID := m.repoID
	repoName := m.repoName
	title := m.titleInput.Value()
	desc := m.descInput.Value()
	srcBranch := m.srcBranch
	targetBranch := m.targetInput.Value()
	reviewer := strings.TrimSpace(m.reviewerInput.Value())
	optReviewer := strings.TrimSpace(m.optRevInput.Value())
	autoComplete := m.autoComplete
	assignee := client.Config().Assignee

	return func() tea.Msg {
		pr, err := client.CreatePullRequest(api.CreatePullRequestInput{
			RepoID:       repoID,
			Title:        title,
			Description:  desc,
			SourceBranch: srcBranch,
			TargetBranch: targetBranch,
		})
		if err != nil {
			return errMsg{fmt.Errorf("create PR: %w", err)}
		}

		if reviewer != "" {
			identity, err := client.SearchIdentity(reviewer)
			if err == nil {
				_ = client.AddPRReviewer(repoID, pr.ID, identity.ID, true)
			}
		}

		if optReviewer != "" {
			identity, err := client.SearchIdentity(optReviewer)
			if err == nil {
				_ = client.AddPRReviewer(repoID, pr.ID, identity.ID, false)
			}
		}

		if autoComplete && assignee != "" {
			identity, err := client.SearchIdentity(assignee)
			if err == nil {
				_ = client.SetAutoComplete(repoID, pr.ID, identity.ID)
			}
		}

		url := client.PRWebURL(repoName, pr.ID)
		return prCreatedMsg{pr: pr, url: url}
	}
}

func (m prCreateModel) view() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Create Pull Request"))
	b.WriteString("\n\n")

	if m.srcBranch != "" {
		fmt.Fprintf(&b, "  Source: %s → Repo: %s\n\n", m.srcBranch, m.repoName)
	}

	steps := []struct {
		label string
		done  bool
	}{
		{"Title", m.step > prStepTitle},
		{"Description", m.step > prStepDesc},
		{"Target Branch", m.step > prStepTarget},
		{"Reviewer", m.step > prStepReviewer},
		{"Optional Reviewer", m.step > prStepOptReviewer},
		{"Auto-Complete", m.step > prStepAutoComplete},
		{"Confirm", m.step > prStepConfirm},
	}

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
	case prStepTitle:
		b.WriteString(labelStyle.Render("  Title:"))
		b.WriteString(" " + m.titleInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next  esc: cancel"))

	case prStepDesc:
		fmt.Fprintf(&b, "  Title: %s\n\n", m.titleInput.Value())
		b.WriteString(labelStyle.Render("  Description:"))
		b.WriteString(" " + m.descInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next (empty to skip)  esc: back"))

	case prStepTarget:
		fmt.Fprintf(&b, "  Title: %s\n\n", m.titleInput.Value())
		b.WriteString(labelStyle.Render("  Target Branch:"))
		b.WriteString(" " + m.targetInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next  esc: back"))

	case prStepReviewer:
		fmt.Fprintf(&b, "  Title: %s\n", m.titleInput.Value())
		fmt.Fprintf(&b, "  Target: %s → %s\n\n", m.srcBranch, m.targetInput.Value())
		b.WriteString(labelStyle.Render("  Required Reviewer:"))
		b.WriteString(" " + m.reviewerInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next (empty to skip)  esc: back"))

	case prStepOptReviewer:
		fmt.Fprintf(&b, "  Title: %s\n", m.titleInput.Value())
		fmt.Fprintf(&b, "  Target: %s → %s\n", m.srcBranch, m.targetInput.Value())
		rev := m.reviewerInput.Value()
		if rev == "" {
			rev = "(none)"
		}
		fmt.Fprintf(&b, "  Required Reviewer: %s\n\n", rev)
		b.WriteString(labelStyle.Render("  Optional Reviewer:"))
		b.WriteString(" " + m.optRevInput.View())
		b.WriteString(helpStyle.Render("\n\n  enter: next (empty to skip)  esc: back"))

	case prStepAutoComplete:
		b.WriteString("  Set auto-complete? (squash merge, delete source branch)\n\n")
		ac := "No"
		if m.autoComplete {
			ac = "Yes"
		}
		fmt.Fprintf(&b, "  Current: %s\n", ac)
		b.WriteString(helpStyle.Render("\n  y: yes  n/enter: no  esc: back"))

	case prStepConfirm:
		b.WriteString("  Summary:\n\n")
		fmt.Fprintf(&b, "    Title:             %s\n", m.titleInput.Value())
		desc := m.descInput.Value()
		if desc == "" {
			desc = "(none)"
		}
		fmt.Fprintf(&b, "    Description:       %s\n", desc)
		fmt.Fprintf(&b, "    Branch:            %s → %s\n", m.srcBranch, m.targetInput.Value())
		rev := m.reviewerInput.Value()
		if rev == "" {
			rev = "(none)"
		}
		fmt.Fprintf(&b, "    Required Reviewer: %s\n", rev)
		optRev := m.optRevInput.Value()
		if optRev == "" {
			optRev = "(none)"
		}
		fmt.Fprintf(&b, "    Optional Reviewer: %s\n", optRev)
		ac := "No"
		if m.autoComplete {
			ac = "Yes"
		}
		fmt.Fprintf(&b, "    Auto-Complete:     %s\n", ac)
		b.WriteString(helpStyle.Render("\n  y/enter: create  n/esc: start over"))

	case prStepDone:
		if m.err != nil {
			b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		} else {
			b.WriteString(successStyle.Render("  " + m.resultMsg))
		}
		b.WriteString(helpStyle.Render("\n\n  enter: open in browser  esc: back"))
	}

	return b.String()
}
