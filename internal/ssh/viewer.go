package ssh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
)

const pageSize = 20

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#7D56F4"))
)

type LogViewerModel struct {
	viewport    viewport.Model
	logs        []types.StoredLog
	cursor      *db.LastCursor
	selectedIdx int
	repo        db.LogRepository
	clientId    string
	instances   *[]string
	ready       bool
	loading     bool
	err         error
}

var _ tea.Model = &LogViewerModel{}

func InitialModel(repo db.LogRepository, clientId string, instances *[]string) LogViewerModel {
	return LogViewerModel{
		logs:      make([]types.StoredLog, 0),
		repo:      repo,
		clientId:  clientId,
		instances: instances,
	}
}

func (m LogViewerModel) Init() tea.Cmd {
	return m.fetchLogs
}

func (m LogViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			// Initialize viewport
			m.viewport = viewport.New(msg.Width, msg.Height-6) // Account for title and help text
			m.viewport.SetContent(m.logsToString())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				m.viewport.SetContent(m.logsToString())

				// If we're near the top and not loading, fetch more logs
				if m.selectedIdx < 3 && !m.loading {
					cmds = append(cmds, m.fetchLogs)
				}
			}
		case "down", "j":
			if m.selectedIdx < len(m.logs)-1 {
				m.selectedIdx++
				m.viewport.SetContent(m.logsToString())
			}
		case "g":
			m.selectedIdx = 0
			m.viewport.SetContent(m.logsToString())
			m.viewport.GotoTop()
		case "G":
			m.selectedIdx = len(m.logs) - 1
			m.viewport.SetContent(m.logsToString())
			m.viewport.GotoBottom()
		}

	case logsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		// Prepend new logs to existing ones
		m.logs = append(msg.logs, m.logs...)

		// Update cursor for next fetch
		if len(msg.logs) > 0 {
			lastLog := msg.logs[0]
			m.cursor = &db.LastCursor{
				Timestamp:      lastLog.Timestamp,
				SequenceNumber: int(lastLog.SequenceNumber),
				IsBackward:     true,
			}
		}

		// Update viewport content
		if m.ready {
			m.viewport.SetContent(m.logsToString())
		}
	}

	// Handle viewport updates
	if m.ready {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m LogViewerModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Title
	title := titleStyle.Render("Log Viewer")
	b.WriteString(title + "\n\n")

	// Loading indicator
	if m.loading {
		b.WriteString("Loading...\n")
	}

	// Viewport
	b.WriteString(m.viewport.View() + "\n")

	// Help
	help := "↑/k: up • ↓/j: down • g: top • G: bottom • q: quit"
	b.WriteString(help)

	return b.String()
}

func (m LogViewerModel) logsToString() string {
	var b strings.Builder

	for i, log := range m.logs {
		logLine := fmt.Sprintf("[%s] %s: %s",
			log.Timestamp.Format(time.RFC3339),
			log.Client.ClientId,
			log.LogLine,
		)

		if i == m.selectedIdx {
			b.WriteString(selectedStyle.Render(logLine))
		} else {
			b.WriteString(logStyle.Render(logLine))
		}
		b.WriteString("\n")
	}

	return b.String()
}

type logsMsg struct {
	logs []types.StoredLog
	err  error
}

func (m LogViewerModel) fetchLogs() tea.Msg {
	m.loading = true
	logs, err := m.repo.GetLogs(context.Background(), m.clientId, m.instances, pageSize, nil, m.cursor)
	return logsMsg{logs: logs, err: err}
}
