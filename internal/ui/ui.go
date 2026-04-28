// Package ui implements the Bubble Tea TUI for mrv.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Message is an event sent from the agent goroutine to the TUI.
type Message struct {
	Author string // e.g. "mrv", "tool:read_file", "error"
	Text   string
	Done   bool // agent finished this turn; clear the spinner
}

// InputSubmitted is the tea.Msg produced when the user presses Enter.
type InputSubmitted struct {
	Text string
}

// AgentMsg wraps an agent Message for the Bubble Tea update loop.
type AgentMsg Message

// styles
var (
	styleUser  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleAgent = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleTool  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleError = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("8"))
)

// Model is the Bubble Tea model for the mrv TUI.
type Model struct {
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	messages  []string
	loading   bool
	agentCh   <-chan Message
	inputCh   chan<- string
	width     int
	height    int
	ready     bool
}

// New creates a new TUI Model. agentCh receives messages from the agent
// goroutine; inputCh receives user input sent to the agent goroutine.
func New(inputCh chan<- string, agentCh <-chan Message) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ta := textarea.New()
	ta.Placeholder = "Ask mrv something... (Enter to send)"
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.CharLimit = 0

	return Model{
		spinner: sp,
		textarea: ta,
		agentCh: agentCh,
		inputCh: inputCh,
	}
}

// Init starts the spinner and begins listening for agent messages.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		waitForAgentMsg(m.agentCh),
	)
}

// waitForAgentMsg returns a Cmd that blocks until the agent sends a message.
func waitForAgentMsg(ch <-chan Message) tea.Cmd {
	return func() tea.Msg {
		return AgentMsg(<-ch)
	}
}

// Update handles all Bubble Tea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			if m.loading {
				break // ignore input while agent is thinking
			}
			text := strings.TrimSpace(m.textarea.Value())
			if text == "" {
				break
			}
			m.textarea.Reset()
			m.messages = append(m.messages, styleUser.Render("you")+": "+text)
			m.loading = true
			m.updateViewport()
			// Send to agent goroutine (non-blocking; channel is buffered).
			go func() { m.inputCh <- text }()

		default:
			var taCmd tea.Cmd
			m.textarea, taCmd = m.textarea.Update(msg)
			cmds = append(cmds, taCmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 0
		footerHeight := m.textarea.Height() + 2 // textarea + border
		statusHeight := 1
		vpHeight := m.height - headerHeight - footerHeight - statusHeight
		if vpHeight < 1 {
			vpHeight = 1
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpHeight)
			m.viewport.SetContent(m.viewportContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpHeight
		}
		m.textarea.SetWidth(msg.Width)

	case spinner.TickMsg:
		var spCmd tea.Cmd
		m.spinner, spCmd = m.spinner.Update(msg)
		cmds = append(cmds, spCmd)

	case AgentMsg:
		am := Message(msg)
		if am.Done {
			m.loading = false
		} else if am.Text != "" {
			line := m.formatAgentLine(am)
			m.messages = append(m.messages, line)
			m.updateViewport()
		}
		// Always re-arm the listener.
		cmds = append(cmds, waitForAgentMsg(m.agentCh))
	}

	// Update viewport scrolling.
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// View renders the full TUI.
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var status string
	if m.loading {
		status = m.spinner.View() + " thinking..."
	} else {
		status = styleDim.Render("ready")
	}

	separator := styleBorder.Width(m.width).Render("")

	return fmt.Sprintf("%s\n%s\n%s\n%s",
		m.viewport.View(),
		status,
		separator,
		m.textarea.View(),
	)
}

func (m *Model) formatAgentLine(msg Message) string {
	switch {
	case strings.HasPrefix(msg.Author, "tool:"):
		name := strings.TrimPrefix(msg.Author, "tool:")
		return styleTool.Render("["+name+"]") + " " + msg.Text
	case msg.Author == "error":
		return styleError.Render("error") + ": " + msg.Text
	default:
		return styleAgent.Render(msg.Author) + ": " + msg.Text
	}
}

func (m *Model) updateViewport() {
	if m.ready {
		m.viewport.SetContent(m.viewportContent())
		m.viewport.GotoBottom()
	}
}

func (m *Model) viewportContent() string {
	if len(m.messages) == 0 {
		return styleDim.Render("  Welcome to mrv. Type a message to get started.")
	}
	return strings.Join(m.messages, "\n")
}
