package ssh

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

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
	logs        []types.StoredLog
	cursor      *db.LastCursor
	selectedIdx int
	repo        db.LogRepository
	clientId    string
	instances   *[]string
	height      int
	width       int
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
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				// If we're at the top of the visible list and have more logs to fetch
				if m.selectedIdx == 0 && !m.loading {
					return m, m.fetchLogs
				}
			}
			return m, nil
		case "down":
			if m.selectedIdx < len(m.logs)-1 {
				m.selectedIdx++
			}
			return m, nil
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

		return m, nil
	}

	return m, nil
}

func (m LogViewerModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var b strings.Builder

	// Title
	title := titleStyle.Render("Log Viewer")
	b.WriteString(title + "\n\n")

	// Loading indicator
	if m.loading {
		b.WriteString("Loading...\n")
	}

	// Logs
	visibleLogs := m.logs
	if len(visibleLogs) > m.height-4 { // Account for title and padding
		outer := math.Max(math.Min(float64(m.height-4), float64(len(visibleLogs))), 0)
		visibleLogs = visibleLogs[:int64(outer)]
	}

	for i, log := range visibleLogs {
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

	// Help
	help := "\nup/down: navigate • q: quit"
	b.WriteString(help)

	return b.String()
}

type logsMsg struct {
	logs []types.StoredLog
	err  error
}

func (m LogViewerModel) fetchLogs() tea.Msg {
	logs, err := m.repo.GetLogs(context.Background(), m.clientId, m.instances, pageSize, nil, m.cursor)
	return logsMsg{logs: logs, err: err}
}
