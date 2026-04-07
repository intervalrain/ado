package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cache"
)

type prView int

const (
	prViewRepos prView = iota
	prViewList
	prViewCreate
)

type reposLoadedMsg struct {
	repos []api.Repository
}

type prsLoadedMsg struct {
	prs []api.PullRequest
}

type prModel struct {
	client *api.Client
	cache  *cache.Cache
	view   prView

	// Repo picker
	repos     []api.Repository
	repoCur   int
	repoErr   error
	reposLoad bool

	// PR list
	selectedRepo api.Repository
	prs          []api.PullRequest
	prCur        int
	prErr        error
	prsLoad      bool
	msg          string

	// PR create wizard
	createMdl prCreateModel
}

func newPRModel(client *api.Client) prModel {
	return prModel{
		client: client,
		cache:  cache.Load(),
		view:   prViewRepos,
	}
}

func (m prModel) init() tea.Cmd {
	return m.fetchRepos
}

func (m prModel) fetchRepos() tea.Msg {
	repos, err := m.client.ListRepositories()
	if err != nil {
		return errMsg{err}
	}
	return reposLoadedMsg{repos: repos}
}

func (m prModel) fetchPRs(repoID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		prs, err := client.ListPullRequests(repoID)
		if err != nil {
			return errMsg{err}
		}
		return prsLoadedMsg{prs: prs}
	}
}

// combinedRepos returns favorites first, then non-favorites, with the favorite count.
func (m prModel) combinedRepos() (list []api.Repository, favCount int) {
	for _, r := range m.repos {
		if m.cache.IsFavRepo(r.ID) {
			list = append(list, r)
		}
	}
	favCount = len(list)
	for _, r := range m.repos {
		if !m.cache.IsFavRepo(r.ID) {
			list = append(list, r)
		}
	}
	return
}

func (m prModel) update(msg tea.Msg) (prModel, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		if m.view == prViewRepos {
			m.repoErr = msg.err
		} else {
			m.prErr = msg.err
		}
		return m, nil
	case reposLoadedMsg:
		m.repos = msg.repos
		m.reposLoad = true
		return m, nil
	case prsLoadedMsg:
		m.prs = msg.prs
		m.prsLoad = true
		m.prCur = 0
		return m, nil
	}

	switch m.view {
	case prViewRepos:
		return m.updateRepos(msg)
	case prViewList:
		return m.updatePRList(msg)
	case prViewCreate:
		return m.updateCreate(msg)
	}
	return m, nil
}

func (m prModel) updateRepos(msg tea.Msg) (prModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		list, _ := m.combinedRepos()
		switch keyMsg.String() {
		case "up", "k":
			if m.repoCur > 0 {
				m.repoCur--
			}
		case "down", "j":
			if m.repoCur < len(list)-1 {
				m.repoCur++
			}
		case "f":
			if len(list) > 0 {
				repo := list[m.repoCur]
				if m.cache.IsFavRepo(repo.ID) {
					m.cache.RemoveFavRepo(repo.ID)
				} else {
					m.cache.AddFavRepo(repo.ID, repo.Name)
				}
				_ = m.cache.Save()
			}
		case "enter":
			if len(list) > 0 {
				m.selectedRepo = list[m.repoCur]
				m.view = prViewList
				m.prsLoad = false
				m.prs = nil
				m.prErr = nil
				m.prCur = 0
				return m, m.fetchPRs(m.selectedRepo.ID)
			}
		}
	}
	return m, nil
}

func (m prModel) updatePRList(msg tea.Msg) (prModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.view = prViewRepos
			return m, nil
		case "up", "k":
			if m.prCur > 0 {
				m.prCur--
			}
		case "down", "j":
			if m.prCur < len(m.prs)-1 {
				m.prCur++
			}
		case "enter":
			if len(m.prs) > 0 {
				pr := m.prs[m.prCur]
				url := m.client.PRWebURL(m.selectedRepo.Name, pr.ID)
				openBrowser(url)
				m.msg = fmt.Sprintf("Opened PR #%d in browser", pr.ID)
			}
		case "r":
			m.prsLoad = false
			m.prs = nil
			m.prErr = nil
			m.msg = "Refreshing..."
			return m, m.fetchPRs(m.selectedRepo.ID)
		case "n":
			m.createMdl = newPRCreateModel(m.client, m.selectedRepo.ID, m.selectedRepo.Name)
			m.view = prViewCreate
			return m, m.createMdl.titleInput.Cursor.BlinkCmd()
		}
	}
	return m, nil
}

func (m prModel) updateCreate(msg tea.Msg) (prModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if m.createMdl.step == prStepTitle || m.createMdl.step == prStepDone {
				m.view = prViewList
				return m, nil
			}
		case "enter":
			if m.createMdl.step == prStepDone && m.createMdl.err == nil {
				lines := strings.Split(m.createMdl.resultMsg, "\n")
				if len(lines) > 1 {
					openBrowser(lines[len(lines)-1])
				}
				m.view = prViewList
				m.prsLoad = false
				m.prs = nil
				m.msg = "Refreshing..."
				return m, m.fetchPRs(m.selectedRepo.ID)
			}
		case "ctrl+c":
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.createMdl, cmd = m.createMdl.update(msg)
	return m, cmd
}

func (m prModel) viewStr() string {
	switch m.view {
	case prViewList:
		return m.viewPRList()
	case prViewCreate:
		return m.createMdl.view()
	default:
		return m.viewRepos()
	}
}

func (m prModel) viewRepos() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Repositories"))
	b.WriteString("\n\n")

	if m.repoErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.repoErr)))
		b.WriteString(helpStyle.Render("\n\n  esc: back"))
		return b.String()
	}
	if !m.reposLoad {
		b.WriteString("  Loading repositories...\n")
		return b.String()
	}

	list, favCount := m.combinedRepos()
	if len(list) == 0 {
		b.WriteString(itemStyle.Render("  (no repositories found)"))
		b.WriteString("\n")
	} else {
		if favCount > 0 {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("  ★ Favorites"))
			b.WriteString("\n")
		}
		for i, repo := range list {
			if i == favCount && favCount > 0 {
				b.WriteString("\n")
				b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("241")).Render("  All Repositories"))
				b.WriteString("\n")
			}
			star := "  "
			if m.cache.IsFavRepo(repo.ID) {
				star = "★ "
			}
			line := star + repo.Name
			if i == m.repoCur {
				b.WriteString(selectedStyle.Render("> " + line))
			} else {
				b.WriteString(itemStyle.Render("  " + line))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter: view PRs  f: toggle favorite  esc: back"))
	return b.String()
}

func (m prModel) viewPRList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Pull Requests — %s", m.selectedRepo.Name)))
	b.WriteString("\n\n")

	if m.prErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.prErr)))
		b.WriteString(helpStyle.Render("\n\n  esc: back"))
		return b.String()
	}
	if !m.prsLoad {
		b.WriteString("  Loading pull requests...\n")
		return b.String()
	}

	if len(m.prs) == 0 {
		b.WriteString(itemStyle.Render("  (no active pull requests)"))
		b.WriteString("\n")
	} else {
		for i, pr := range m.prs {
			src := trimRef(pr.SourceRefName)
			tgt := trimRef(pr.TargetRefName)

			reviewSummary := buildReviewSummary(pr.Reviewers)

			line := fmt.Sprintf("%-50s  %s → %s  @%s  %s",
				truncate(pr.Title, 50),
				src, tgt,
				pr.CreatedBy.DisplayName,
				reviewSummary,
			)

			if i == m.prCur {
				b.WriteString(selectedStyle.Render("> " + line))
			} else {
				b.WriteString(itemStyle.Render("  " + line))
			}
			b.WriteString("\n")
		}
	}

	if m.msg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("\n  " + m.msg))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter: open in browser  n: new PR  r: refresh  esc: back to repos"))

	// Detail view of selected PR
	if len(m.prs) > 0 {
		pr := m.prs[m.prCur]
		b.WriteString("\n\n")
		b.WriteString(labelStyle.Render("  Reviewers:"))
		b.WriteString("\n")
		if len(pr.Reviewers) == 0 {
			b.WriteString(itemStyle.Render("    (none)"))
			b.WriteString("\n")
		} else {
			for _, r := range pr.Reviewers {
				icon := voteIcon(r.Vote)
				fmt.Fprintf(&b, "    %s %s (%s, %s)\n",
					icon, r.DisplayName, r.RequiredLabel(), r.VoteLabel())
			}
		}
	}

	return b.String()
}

func trimRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func buildReviewSummary(reviewers []api.PRReviewer) string {
	if len(reviewers) == 0 {
		return "no reviewers"
	}
	approved := 0
	waiting := 0
	rejected := 0
	noVote := 0
	for _, r := range reviewers {
		switch {
		case r.Vote >= 5:
			approved++
		case r.Vote == -5:
			waiting++
		case r.Vote == -10:
			rejected++
		default:
			noVote++
		}
	}
	var parts []string
	if approved > 0 {
		parts = append(parts, fmt.Sprintf("%d approved", approved))
	}
	if waiting > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting", waiting))
	}
	if rejected > 0 {
		parts = append(parts, fmt.Sprintf("%d rejected", rejected))
	}
	if noVote > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", noVote))
	}
	return strings.Join(parts, ", ")
}

func voteIcon(vote int) string {
	switch {
	case vote >= 5:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")
	case vote == -5:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("⏳")
	case vote == -10:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("○")
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("cmd", "/c", "start", url)
	}
	_ = cmd.Start()
}
