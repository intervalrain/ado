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
	"github.com/rainhu/ado/internal/git"
)

type prView int

const (
	prViewMenu   prView = iota // Category picker: Created by me / Assigned to me / Browse repos
	prViewList                 // PR list (from category or repo)
	prViewRepos                // Repository picker (optional)
	prViewCreate               // PR creation wizard
)

type prCategory int

const (
	prCatCreatedByMe      prCategory = iota
	prCatAssignedToMe
	prCatAssignedRequired
	prCatBrowseRepos
)

var prMenuItems = []struct {
	label string
	desc  string
}{
	{label: "Created by me", desc: "PRs you created"},
	{label: "Assigned to me", desc: "PRs where you are a reviewer"},
	{label: "Assigned to me (required)", desc: "PRs where you are a required reviewer"},
	{label: "Browse by repository", desc: "Browse all repos and their PRs"},
}

type reposLoadedMsg struct {
	repos []api.Repository
}

type prsLoadedMsg struct {
	prs []api.PullRequest
}

type repoResolvedMsg struct {
	repoID   string
	repoName string
}

type prModel struct {
	client *api.Client
	cache  *cache.Cache
	view   prView

	// Terminal size for scrolling
	termHeight int

	// Category menu
	menuCur int

	// Repo picker
	repos     []api.Repository
	repoCur   int
	repoErr   error
	reposLoad bool

	// PR list
	selectedCategory prCategory
	selectedRepo     api.Repository
	prs              []api.PullRequest
	prCur            int
	prErr            error
	prsLoad          bool
	msg              string

	// PR create wizard
	createMdl   prCreateModel
	createFrom  prView // which view launched the create wizard
}

func newPRModel(client *api.Client) prModel {
	return prModel{
		client:     client,
		cache:      cache.Load(),
		view:       prViewMenu,
		termHeight: 24, // sensible default
	}
}

func (m prModel) init() tea.Cmd {
	return nil // no upfront loading
}

func (m prModel) fetchRepos() tea.Msg {
	repos, err := m.client.ListRepositories()
	if err != nil {
		return errMsg{err}
	}
	return reposLoadedMsg{repos: repos}
}

func (m prModel) fetchMyCreatedPRs() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		prs, err := client.ListMyCreatedPullRequests()
		if err != nil {
			return errMsg{err}
		}
		return prsLoadedMsg{prs: prs}
	}
}

func (m prModel) fetchMyReviewPRs() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		prs, err := client.ListMyReviewPullRequests()
		if err != nil {
			return errMsg{err}
		}
		return prsLoadedMsg{prs: prs}
	}
}

func (m prModel) fetchMyRequiredPRs() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		prs, err := client.ListMyPullRequests()
		if err != nil {
			return errMsg{err}
		}
		return prsLoadedMsg{prs: prs}
	}
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

// resolveCurrentRepo detects the git repo name and resolves its ADO repo ID.
func (m prModel) resolveCurrentRepo() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		repoName, err := git.RepoName()
		if err != nil {
			return errMsg{fmt.Errorf("detect git repo: %w", err)}
		}
		repos, err := client.ListRepositories()
		if err != nil {
			return errMsg{err}
		}
		for _, r := range repos {
			if strings.EqualFold(r.Name, repoName) {
				return repoResolvedMsg{repoID: r.ID, repoName: r.Name}
			}
		}
		return errMsg{fmt.Errorf("repo %q not found in Azure DevOps project", repoName)}
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

// visibleRange computes the start/end indices for a scrollable list
// given total items, current cursor position, and available lines.
func visibleRange(total, cursor, availableLines int) (start, end int) {
	if total <= availableLines {
		return 0, total
	}
	half := availableLines / 2
	start = cursor - half
	if start < 0 {
		start = 0
	}
	end = start + availableLines
	if end > total {
		end = total
		start = end - availableLines
	}
	return
}

func (m prModel) update(msg tea.Msg) (prModel, tea.Cmd) {
	// Handle terminal resize
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.termHeight = sizeMsg.Height
		return m, nil
	}

	switch msg := msg.(type) {
	case errMsg:
		if m.view == prViewCreate {
			// Let create model handle its own errors
			break
		}
		if m.view == prViewMenu || m.view == prViewList {
			m.msg = fmt.Sprintf("Error: %v", msg.err)
		} else if m.view == prViewRepos {
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
	case repoResolvedMsg:
		m.createMdl = newPRCreateModel(m.client, m.cache, msg.repoID, msg.repoName)
		m.createFrom = m.view
		m.view = prViewCreate
		m.msg = ""
		return m, m.createMdl.titleInput.Cursor.BlinkCmd()
	}

	switch m.view {
	case prViewMenu:
		return m.updateMenu(msg)
	case prViewRepos:
		return m.updateRepos(msg)
	case prViewList:
		return m.updatePRList(msg)
	case prViewCreate:
		return m.updateCreate(msg)
	}
	return m, nil
}

func (m prModel) updateMenu(msg tea.Msg) (prModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "k":
			if m.menuCur > 0 {
				m.menuCur--
			}
		case "down", "j":
			if m.menuCur < len(prMenuItems)-1 {
				m.menuCur++
			}
		case "enter":
			switch prCategory(m.menuCur) {
			case prCatCreatedByMe:
				m.selectedCategory = prCatCreatedByMe
				m.view = prViewList
				m.prsLoad = false
				m.prs = nil
				m.prErr = nil
				m.prCur = 0
				m.msg = ""
				return m, m.fetchMyCreatedPRs()
			case prCatAssignedToMe:
				m.selectedCategory = prCatAssignedToMe
				m.view = prViewList
				m.prsLoad = false
				m.prs = nil
				m.prErr = nil
				m.prCur = 0
				m.msg = ""
				return m, m.fetchMyReviewPRs()
			case prCatAssignedRequired:
				m.selectedCategory = prCatAssignedRequired
				m.view = prViewList
				m.prsLoad = false
				m.prs = nil
				m.prErr = nil
				m.prCur = 0
				m.msg = ""
				return m, m.fetchMyRequiredPRs()
			case prCatBrowseRepos:
				m.view = prViewRepos
				m.repoCur = 0
				if !m.reposLoad {
					m.repoErr = nil
					return m, m.fetchRepos
				}
			}
		case "n":
			m.msg = "Detecting current repository..."
			return m, m.resolveCurrentRepo()
		}
	}
	return m, nil
}

func (m prModel) updateRepos(msg tea.Msg) (prModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		list, _ := m.combinedRepos()
		switch keyMsg.String() {
		case "esc":
			m.view = prViewMenu
			return m, nil
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
				m.selectedCategory = prCatBrowseRepos
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
			if m.selectedCategory == prCatBrowseRepos {
				m.view = prViewRepos
			} else {
				m.view = prViewMenu
			}
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
				repoName := m.prRepoName(pr)
				url := m.client.PRWebURL(repoName, pr.ID)
				openBrowser(url)
				m.msg = fmt.Sprintf("Opened PR #%d in browser", pr.ID)
			}
		case "r":
			m.prsLoad = false
			m.prs = nil
			m.prErr = nil
			m.msg = "Refreshing..."
			return m, m.refreshPRs()
		case "n":
			if m.selectedCategory == prCatBrowseRepos {
				m.createMdl = newPRCreateModel(m.client, m.cache, m.selectedRepo.ID, m.selectedRepo.Name)
				m.createFrom = prViewList
				m.view = prViewCreate
				return m, m.createMdl.titleInput.Cursor.BlinkCmd()
			}
			// For non-browse categories, auto-detect repo from git
			m.msg = "Detecting current repository..."
			return m, m.resolveCurrentRepo()
		}
	}
	return m, nil
}

func (m prModel) refreshPRs() tea.Cmd {
	switch m.selectedCategory {
	case prCatCreatedByMe:
		return m.fetchMyCreatedPRs()
	case prCatAssignedToMe:
		return m.fetchMyReviewPRs()
	case prCatAssignedRequired:
		return m.fetchMyRequiredPRs()
	default:
		return m.fetchPRs(m.selectedRepo.ID)
	}
}

func (m prModel) prRepoName(pr api.PullRequest) string {
	if m.selectedCategory == prCatBrowseRepos {
		return m.selectedRepo.Name
	}
	return pr.Repository.Name
}

func (m prModel) updateCreate(msg tea.Msg) (prModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if m.createMdl.step == prStepTitle || m.createMdl.step == prStepDone {
				m.view = m.createFrom
				m.msg = ""
				return m, nil
			}
		case "enter":
			if m.createMdl.step == prStepDone && m.createMdl.err == nil {
				lines := strings.Split(m.createMdl.resultMsg, "\n")
				if len(lines) > 1 {
					openBrowser(lines[len(lines)-1])
				}
				if m.createFrom == prViewList {
					m.view = prViewList
					m.prsLoad = false
					m.prs = nil
					m.msg = "Refreshing..."
					return m, m.refreshPRs()
				}
				m.view = prViewMenu
				m.msg = "PR created successfully"
				return m, nil
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
	case prViewMenu:
		return m.viewMenu()
	case prViewRepos:
		return m.viewRepos()
	case prViewList:
		return m.viewPRList()
	case prViewCreate:
		return m.createMdl.view()
	default:
		return m.viewMenu()
	}
}

func (m prModel) viewMenu() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Pull Requests"))
	b.WriteString("\n\n")

	for i, item := range prMenuItems {
		desc := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  " + item.desc)
		line := item.label + desc
		if i == m.menuCur {
			b.WriteString(selectedStyle.Render("> " + line))
		} else {
			b.WriteString(itemStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	if m.msg != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("  " + m.msg))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter: select  n: new PR  esc: back"))
	return b.String()
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
		// Available lines for the repo list (reserve lines for header, help, etc.)
		availLines := m.termHeight - 8
		if availLines < 5 {
			availLines = 5
		}
		start, end := visibleRange(len(list), m.repoCur, availLines)

		if start > 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ↑ more repos above"))
			b.WriteString("\n")
		}

		if favCount > 0 && start == 0 {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("  ★ Favorites"))
			b.WriteString("\n")
		}
		for i := start; i < end; i++ {
			repo := list[i]
			if i == favCount && favCount > 0 && i >= start {
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

		if end < len(list) {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ↓ more repos below"))
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter: view PRs  f: toggle favorite  esc: back"))
	return b.String()
}

func (m prModel) prListTitle() string {
	switch m.selectedCategory {
	case prCatCreatedByMe:
		return "Pull Requests — Created by me"
	case prCatAssignedToMe:
		return "Pull Requests — Assigned to me"
	case prCatAssignedRequired:
		return "Pull Requests — Assigned to me (required)"
	default:
		return fmt.Sprintf("Pull Requests — %s", m.selectedRepo.Name)
	}
}

func (m prModel) viewPRList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(m.prListTitle()))
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
		// Reserve lines for title, help, detail view, etc.
		availLines := m.termHeight - 14
		if availLines < 5 {
			availLines = 5
		}
		start, end := visibleRange(len(m.prs), m.prCur, availLines)

		if start > 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
				fmt.Sprintf("  ↑ %d more above", start)))
			b.WriteString("\n")
		}

		showRepo := m.selectedCategory != prCatBrowseRepos
		for i := start; i < end; i++ {
			pr := m.prs[i]
			src := trimRef(pr.SourceRefName)
			tgt := trimRef(pr.TargetRefName)
			reviewSummary := buildReviewSummary(pr.Reviewers)

			var line string
			if showRepo {
				line = fmt.Sprintf("[%s] %-40s  %s → %s  @%s  %s",
					pr.Repository.Name,
					truncate(pr.Title, 40),
					src, tgt,
					pr.CreatedBy.DisplayName,
					reviewSummary,
				)
			} else {
				line = fmt.Sprintf("%-50s  %s → %s  @%s  %s",
					truncate(pr.Title, 50),
					src, tgt,
					pr.CreatedBy.DisplayName,
					reviewSummary,
				)
			}

			if i == m.prCur {
				b.WriteString(selectedStyle.Render("> " + line))
			} else {
				b.WriteString(itemStyle.Render("  " + line))
			}
			b.WriteString("\n")
		}

		if end < len(m.prs) {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
				fmt.Sprintf("  ↓ %d more below", len(m.prs)-end)))
			b.WriteString("\n")
		}
	}

	if m.msg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("\n  " + m.msg))
		b.WriteString("\n")
	}

	helpText := "  ↑↓: navigate  enter: open in browser  n: new PR  r: refresh  esc: back"
	if m.selectedCategory == prCatBrowseRepos {
		helpText = "  ↑↓: navigate  enter: open in browser  n: new PR  r: refresh  esc: back to repos"
	}
	b.WriteString(helpStyle.Render("\n" + helpText))

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
