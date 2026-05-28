package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	oairt "github.com/tmc/misc/oairt"
)

type uiEventKind int

const (
	uiStatus uiEventKind = iota
	uiAssistantDelta
	uiAssistantDone
	uiTool
	uiAudio
	uiError
)

type uiEvent struct {
	kind  uiEventKind
	text  string
	bytes int
}

func postUIEvent(ch chan<- uiEvent, ev uiEvent) {
	select {
	case ch <- ev:
	default:
	}
}

type tuiOptions struct {
	cfg    *config
	client *oairt.Client
	events <-chan uiEvent
	stdin  io.Reader
	stdout io.Writer
}

type uiEventMsg struct{ ev uiEvent }
type uiClosedMsg struct{}
type tickMsg time.Time

type tuiModel struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *oairt.Client
	events <-chan uiEvent

	width  int
	height int

	input      string
	transcript string
	logs       []string

	recording bool
	mic       *micRecorder
	micBytes  *atomic.Int64
	outBytes  int
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("24")).Padding(0, 1)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("43"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	inputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func runTUI(ctx context.Context, opts tuiOptions) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m := tuiModel{
		ctx:      ctx,
		cancel:   cancel,
		client:   opts.client,
		events:   opts.events,
		micBytes: new(atomic.Int64),
	}
	options := []tea.ProgramOption{tea.WithAltScreen()}
	if f, ok := opts.stdin.(*os.File); ok {
		options = append(options, tea.WithInput(f))
	}
	if f, ok := opts.stdout.(*os.File); ok {
		options = append(options, tea.WithOutput(f))
	}
	_, err := tea.NewProgram(m, options...).Run()
	return err
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(waitUIEvent(m.events), tick())
}

func waitUIEvent(ch <-chan uiEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return uiClosedMsg{}
		}
		return uiEventMsg{ev: ev}
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case uiEventMsg:
		m.applyEvent(msg.ev)
		return m, waitUIEvent(m.events)
	case uiClosedMsg:
		return m, tea.Quit
	case tickMsg:
		return m, tick()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *tuiModel) applyEvent(ev uiEvent) {
	switch ev.kind {
	case uiStatus:
		m.addLog("session: " + ev.text)
	case uiAssistantDelta:
		m.transcript += ev.text
	case uiAssistantDone:
		m.transcript += "\n"
	case uiTool:
		m.addLog("tool: " + ev.text)
	case uiAudio:
		m.outBytes += ev.bytes
	case uiError:
		m.addLog("error: " + ev.text)
	}
}

func (m tuiModel) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "q":
		if m.recording {
			_ = m.stopRecording(false)
		}
		m.cancel()
		return m, tea.Quit
	case "ctrl+r":
		m.toggleRecording()
		return m, nil
	case "esc":
		if m.recording {
			_ = m.stopRecording(false)
		}
		return m, nil
	case "enter":
		if m.recording {
			_ = m.stopRecording(true)
			return m, nil
		}
		text := strings.TrimSpace(m.input)
		m.input = ""
		if text == "" {
			return m, nil
		}
		if err := sendUserText(m.client, text); err != nil {
			m.addLog("send text: " + err.Error())
		} else {
			m.addLog("you: " + text)
		}
		return m, nil
	case "backspace", "ctrl+h":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	case " ":
		if m.input == "" {
			m.toggleRecording()
			return m, nil
		}
		m.input += " "
		return m, nil
	default:
		if s := key.String(); len(s) == 1 && s >= " " {
			m.input += s
		}
		return m, nil
	}
}

func (m *tuiModel) toggleRecording() {
	if m.recording {
		_ = m.stopRecording(true)
		return
	}
	m.startRecording()
}

func (m *tuiModel) startRecording() {
	if m.micBytes == nil {
		m.micBytes = new(atomic.Int64)
	}
	m.micBytes.Store(0)
	mic := newMicRecorder(24000, func(data []byte) {
		if len(data) == 0 {
			return
		}
		m.micBytes.Add(int64(len(data)))
		_ = m.client.SendAudio(data)
	})
	if err := mic.Start(m.ctx); err != nil {
		m.addLog("mic: " + err.Error())
		return
	}
	m.mic = mic
	m.recording = true
	m.addLog("mic: recording")
}

func (m *tuiModel) stopRecording(commit bool) error {
	if m.mic != nil {
		if err := m.mic.Stop(); err != nil {
			m.addLog("mic stop: " + err.Error())
		}
		m.mic = nil
	}
	m.recording = false
	if !commit {
		_ = m.client.Send(oairt.Event{Type: oairt.EventInputAudioBufferClear})
		m.addLog("mic: canceled")
		return nil
	}
	if err := m.client.Send(oairt.Event{Type: oairt.EventInputAudioBufferCommit}); err != nil {
		m.addLog("commit audio: " + err.Error())
		return err
	}
	if err := m.client.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		m.addLog("response.create: " + err.Error())
		return err
	}
	m.addLog(fmt.Sprintf("mic: sent %.1fkB", float64(m.currentMicBytes())/1024))
	return nil
}

func (m *tuiModel) addLog(s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	m.logs = append(m.logs, s)
	if len(m.logs) > 200 {
		copy(m.logs, m.logs[len(m.logs)-200:])
		m.logs = m.logs[:200]
	}
}

func (m tuiModel) View() string {
	width := m.width
	if width < 80 {
		width = 80
	}
	height := m.height
	if height < 24 {
		height = 24
	}
	inner := width - 4
	if inner < 20 {
		inner = 20
	}

	status := "connected"
	if m.recording {
		status = "recording"
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render("oairt zelda3"),
		" ",
		statusStyle.Render(status),
		" ",
		helpStyle.Render("ctrl+r/space ptt  enter send/commit  esc cancel  q quit"),
	)

	bodyHeight := height - 8
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	logHeight := bodyHeight / 3
	transcriptHeight := bodyHeight - logHeight

	transcript := panelStyle.Width(inner).Height(transcriptHeight).Render(trimToLines(m.transcript, transcriptHeight-2))
	logs := panelStyle.Width(inner).Height(logHeight).Render(m.renderLogs(logHeight - 2))
	input := inputStyle.Render("> " + m.input)
	meters := helpStyle.Render(fmt.Sprintf("mic %.1fkB  audio %.1fkB", float64(m.currentMicBytes())/1024, float64(m.outBytes)/1024))
	if m.recording {
		meters = errorStyle.Render("REC ") + meters
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, transcript, logs, input, meters)
}

func (m tuiModel) currentMicBytes() int64 {
	if m.micBytes == nil {
		return 0
	}
	return m.micBytes.Load()
}

func (m tuiModel) renderLogs(max int) string {
	if max <= 0 || len(m.logs) == 0 {
		return ""
	}
	start := len(m.logs) - max
	if start < 0 {
		start = 0
	}
	return strings.Join(m.logs[start:], "\n")
}

func trimToLines(s string, max int) string {
	if max <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= max {
		return s
	}
	return strings.Join(lines[len(lines)-max:], "\n")
}
