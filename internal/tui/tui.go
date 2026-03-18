package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxLines = 1000

var (
	restartFunc func()
	restartMu   sync.Mutex
)

// SetRestartFunc sets the function to call when Shift-R is pressed.
func SetRestartFunc(fn func()) {
	restartMu.Lock()
	restartFunc = fn
	restartMu.Unlock()
}

// RestartMsg signals the TUI that a restart was requested.
type RestartMsg struct{}

// OutputMsg carries a line of output from the subprocess.
type OutputMsg string

// RestartBannerMsg tells the TUI to show a restart banner.
type RestartBannerMsg struct{}

// ProcessExitedMsg signals the subprocess exited.
type ProcessExitedMsg struct{ Err error }

// Model is the bubbletea model for the devup TUI.
type Model struct {
	hostname  string
	port      int
	proxyPort int
	secure    bool
	logPath   string
	cmd       []string
	lines     []string
	width     int
	height    int
	quitting  bool
}

// New creates a new TUI model.
func New(hostname string, port, proxyPort int, secure bool, logPath string, cmd []string) Model {
	return Model{
		hostname:  hostname,
		port:      port,
		proxyPort: proxyPort,
		secure:    secure,
		logPath:   logPath,
		cmd:       cmd,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "R": // Shift-R
			restartMu.Lock()
			fn := restartFunc
			restartMu.Unlock()
			if fn != nil {
				go fn()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case RestartBannerMsg:
		m.lines = append(m.lines, "\x00RESTART")
		if len(m.lines) > maxLines {
			m.lines = m.lines[len(m.lines)-maxLines:]
		}

	case OutputMsg:
		m.lines = append(m.lines, string(msg))
		if len(m.lines) > maxLines {
			m.lines = m.lines[len(m.lines)-maxLines:]
		}

	case ProcessExitedMsg:
		if msg.Err != nil {
			m.lines = append(m.lines, fmt.Sprintf("[process exited: %v]", msg.Err))
		} else {
			m.lines = append(m.lines, "[process exited]")
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	scheme := "http"
	if m.secure {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d", scheme, m.hostname, m.proxyPort)
	headerText := fmt.Sprintf(" %s  %s | Shift-R: restart | Ctrl-C: quit", url, strings.Join(m.cmd, " "))

	// Pad to full width
	width := m.width
	if width < len(headerText) {
		width = len(headerText)
	}
	for len(headerText) < width {
		headerText += " "
	}

	urlEnd := len(url) + 2 // +2 for leading space and one char past

	// Render each character with gradient background
	runes := []rune(headerText)
	for i, ch := range runes {
		t := float64(i) / float64(max(len(runes)-1, 1))
		// Gradient: #5f00d7 → #1a1a2e
		r := int(0x5f + t*float64(0x1a-0x5f))
		g := int(0x00 + t*float64(0x1a-0x00))
		bl := int(0xd7 + t*float64(0x2e-0xd7))
		bg := fmt.Sprintf("#%02x%02x%02x", r, g, bl)

		fg := "#ffffff"
		if i > 0 && i <= urlEnd {
			fg = "#a0d8b0"
		}

		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(fg)).
			Background(lipgloss.Color(bg))
		b.WriteString(style.Render(string(ch)))
	}
	b.WriteString("\n")

	// Second status line: log file path (reverse gradient)
	displayPath := m.logPath
	if home, err := os.UserHomeDir(); err == nil {
		displayPath = strings.Replace(displayPath, home, "~", 1)
	}
	logLine := fmt.Sprintf(" log %s", displayPath)
	for len(logLine) < width {
		logLine += " "
	}
	logRunes := []rune(logLine)
	for i, ch := range logRunes {
		t := float64(i) / float64(max(len(logRunes)-1, 1))
		// Reverse gradient: #1a1a2e → #5f00d7
		r := int(0x1a + t*float64(0x5f-0x1a))
		g := int(0x1a + t*float64(0x00-0x1a))
		bl := int(0x2e + t*float64(0xd7-0x2e))
		bg := fmt.Sprintf("#%02x%02x%02x", r, g, bl)

		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cccccc")).
			Background(lipgloss.Color(bg))
		b.WriteString(style.Render(string(ch)))
	}
	b.WriteString("\n")

	visibleLines := m.height - 4
	if visibleLines < 1 {
		visibleLines = 10
	}

	start := 0
	if len(m.lines) > visibleLines {
		start = len(m.lines) - visibleLines
	}

	for i := start; i < len(m.lines); i++ {
		if m.lines[i] == "\x00RESTART" {
			b.WriteString(renderRestartBanner(m.width))
		} else {
			b.WriteString(m.lines[i])
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderRestartBanner(width int) string {
	text := " ↻ restarting "
	if width < len(text) {
		width = len(text)
	}

	// Center the text with padding
	pad := (width - len(text)) / 2
	line := strings.Repeat(" ", pad) + text + strings.Repeat(" ", width-pad-len(text))

	var b strings.Builder
	runes := []rune(line)
	for i, ch := range runes {
		t := float64(i) / float64(max(len(runes)-1, 1))
		// Gradient: #ff6b35 (orange) → #d7263d (red) → #ff6b35 (orange)
		// Use a sine wave for a symmetric gradient
		mid := 0.5 - 0.5*cosApprox(2*3.14159*t)
		r := int(0xff - mid*float64(0xff-0xd7))
		g := int(0x6b - mid*float64(0x6b-0x26))
		bl := int(0x35 + mid*float64(0x3d-0x35))
		bg := fmt.Sprintf("#%02x%02x%02x", r, g, bl)

		style := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color(bg))
		b.WriteString(style.Render(string(ch)))
	}
	return b.String()
}

func cosApprox(x float64) float64 {
	// Simple cosine approximation (good enough for color gradients)
	x2 := x * x
	return 1 - x2/2 + x2*x2/24
}
