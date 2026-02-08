// ABOUTME: Interactive TUI wizard for configuring push credentials and storage.
// ABOUTME: 4-step bubbletea model with async Pushover API validation.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Step represents the current wizard step.
type Step int

const (
	StepAppToken Step = iota
	StepUserKey
	StepBackend
	StepDataDir
	StepValidating
	StepDone
	StepFailed
)

// validationResultMsg carries the result of an async validation attempt.
type validationResultMsg struct {
	err error
}

// ValidateFn is the function signature for credential validation.
type ValidateFn func(ctx context.Context, appToken, userKey string) error

// cancelHolder shares a cancel function across bubbletea model copies.
type cancelHolder struct {
	cancel context.CancelFunc
}

// SetupModel is the bubbletea model for the setup wizard.
type SetupModel struct {
	step          Step
	inputs        [4]textinput.Model
	spinner       spinner.Model
	validateFn    ValidateFn
	cancelCtx     *cancelHolder
	validationErr error
	quitting      bool
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	brandStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	stepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// defaultDataDir returns the default XDG data directory for push.
func defaultDataDir() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "push")
}

// NewSetupModel creates a new setup wizard model, pre-filling with existing config values.
func NewSetupModel(appToken, userKey, backend, dataDir string) SetupModel {
	tokenInput := textinput.New()
	tokenInput.Placeholder = "your-app-token"
	tokenInput.EchoMode = textinput.EchoPassword
	tokenInput.Focus()
	tokenInput.Width = 50
	if appToken != "" {
		tokenInput.SetValue(appToken)
	}

	keyInput := textinput.New()
	keyInput.Placeholder = "your-user-key"
	keyInput.EchoMode = textinput.EchoPassword
	keyInput.Width = 50
	if userKey != "" {
		keyInput.SetValue(userKey)
	}

	backendInput := textinput.New()
	backendInput.Placeholder = "sqlite"
	backendInput.Width = 50
	if backend != "" {
		backendInput.SetValue(backend)
	}

	dataDirInput := textinput.New()
	dataDirInput.Placeholder = defaultDataDir()
	dataDirInput.Width = 50
	if dataDir != "" {
		dataDirInput.SetValue(dataDir)
	}

	s := spinner.New()
	s.Spinner = spinner.Dot

	return SetupModel{
		step:       StepAppToken,
		inputs:     [4]textinput.Model{tokenInput, keyInput, backendInput, dataDirInput},
		spinner:    s,
		validateFn: ValidateCredentials,
		cancelCtx:  &cancelHolder{},
	}
}

// Init implements tea.Model.
func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			m.quitting = true
			if m.cancelCtx.cancel != nil {
				m.cancelCtx.cancel()
			}
			return m, tea.Quit
		}

		switch m.step {
		case StepAppToken, StepUserKey, StepBackend, StepDataDir:
			return m.updateInput(msg)
		case StepFailed:
			return m.updateFailed(msg)
		}

	case validationResultMsg:
		m.cancelCtx.cancel = nil
		if msg.err == nil {
			m.step = StepDone
			return m, tea.Quit
		}
		m.validationErr = msg.err
		m.step = StepFailed
		return m, nil

	case spinner.TickMsg:
		if m.step == StepValidating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	default:
		// Forward other messages (e.g. cursor blink) to the active input
		switch m.step {
		case StepAppToken, StepUserKey, StepBackend, StepDataDir:
			idx := int(m.step)
			var cmd tea.Cmd
			m.inputs[idx], cmd = m.inputs[idx].Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m SetupModel) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		return m.handleEnter()
	}

	idx := int(m.step)
	var cmd tea.Cmd
	m.inputs[idx], cmd = m.inputs[idx].Update(msg)
	return m, cmd
}

func (m SetupModel) handleEnter() (tea.Model, tea.Cmd) {
	idx := int(m.step)

	// Don't advance on empty required fields
	if m.step == StepAppToken && strings.TrimSpace(m.inputs[0].Value()) == "" {
		return m, nil
	}
	if m.step == StepUserKey && strings.TrimSpace(m.inputs[1].Value()) == "" {
		return m, nil
	}

	// Validate and default backend
	if m.step == StepBackend {
		val := strings.TrimSpace(m.inputs[2].Value())
		if val == "" {
			val = "sqlite"
		}
		val = strings.ToLower(val)
		if val != "sqlite" && val != "markdown" {
			return m, nil
		}
		m.inputs[2].SetValue(val)
	}

	// Default data dir
	if m.step == StepDataDir {
		val := strings.TrimSpace(m.inputs[3].Value())
		if val == "" {
			m.inputs[3].SetValue(defaultDataDir())
		}
	}

	m.inputs[idx].Blur()

	switch m.step {
	case StepAppToken:
		m.step = StepUserKey
		m.inputs[1].Focus()
		return m, textinput.Blink
	case StepUserKey:
		m.step = StepBackend
		m.inputs[2].Focus()
		return m, textinput.Blink
	case StepBackend:
		m.step = StepDataDir
		m.inputs[3].Focus()
		return m, textinput.Blink
	case StepDataDir:
		m.step = StepValidating
		return m, tea.Batch(m.startValidation(), m.spinner.Tick)
	}

	return m, nil
}

func (m SetupModel) updateFailed(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes {
		switch msg.Runes[0] {
		case 'r':
			m.step = StepValidating
			m.validationErr = nil
			return m, tea.Batch(m.startValidation(), m.spinner.Tick)
		case 's':
			m.step = StepDone
			return m, tea.Quit
		case 'q':
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m SetupModel) startValidation() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelCtx.cancel = cancel
	appToken := m.inputs[0].Value()
	userKey := m.inputs[1].Value()
	fn := m.validateFn
	return func() tea.Msg {
		return validationResultMsg{err: fn(ctx, appToken, userKey)}
	}
}

// View implements tea.Model.
func (m SetupModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(brandStyle.Render("   PUSH CANNON ULTRA"))
	b.WriteString(titleStyle.Render(" - Setup"))
	b.WriteString("\n\n")
	b.WriteString("Configure Pushover credentials and storage.\n\n")

	switch m.step {
	case StepAppToken:
		b.WriteString(stepStyle.Render("Step 1 of 4: App Token"))
		b.WriteString("\n")
		b.WriteString(m.inputs[0].View())
		b.WriteString("\n")

	case StepUserKey:
		b.WriteString(fmt.Sprintf("  App Token: %s\n\n", strings.Repeat("*", len(m.inputs[0].Value()))))
		b.WriteString(stepStyle.Render("Step 2 of 4: User Key"))
		b.WriteString("\n")
		b.WriteString(m.inputs[1].View())
		b.WriteString("\n")

	case StepBackend:
		b.WriteString(fmt.Sprintf("  App Token: %s\n", strings.Repeat("*", len(m.inputs[0].Value()))))
		b.WriteString(fmt.Sprintf("  User Key:  %s\n\n", strings.Repeat("*", len(m.inputs[1].Value()))))
		b.WriteString(stepStyle.Render("Step 3 of 4: Storage Backend"))
		b.WriteString("\n")
		b.WriteString(promptStyle.Render("(sqlite or markdown, press Enter for default)"))
		b.WriteString("\n")
		b.WriteString(m.inputs[2].View())
		b.WriteString("\n")

	case StepDataDir:
		b.WriteString(fmt.Sprintf("  App Token: %s\n", strings.Repeat("*", len(m.inputs[0].Value()))))
		b.WriteString(fmt.Sprintf("  User Key:  %s\n", strings.Repeat("*", len(m.inputs[1].Value()))))
		b.WriteString(fmt.Sprintf("  Backend:   %s\n\n", m.inputs[2].Value()))
		b.WriteString(stepStyle.Render("Step 4 of 4: Data Directory"))
		b.WriteString("\n")
		b.WriteString(promptStyle.Render(fmt.Sprintf("(press Enter for default: %s)", defaultDataDir())))
		b.WriteString("\n")
		b.WriteString(m.inputs[3].View())
		b.WriteString("\n")

	case StepValidating:
		b.WriteString(fmt.Sprintf("  App Token: %s\n", strings.Repeat("*", len(m.inputs[0].Value()))))
		b.WriteString(fmt.Sprintf("  User Key:  %s\n", strings.Repeat("*", len(m.inputs[1].Value()))))
		b.WriteString(fmt.Sprintf("  Backend:   %s\n", m.inputs[2].Value()))
		b.WriteString(fmt.Sprintf("  Data Dir:  %s\n\n", m.inputs[3].Value()))
		b.WriteString(m.spinner.View())
		b.WriteString(" Validating credentials...")
		b.WriteString("\n")

	case StepDone:
		b.WriteString(successStyle.Render("Connected!"))
		b.WriteString("\n")

	case StepFailed:
		errMsg := "unknown error"
		if m.validationErr != nil {
			errMsg = m.validationErr.Error()
		}
		b.WriteString(errorStyle.Render(fmt.Sprintf("Validation failed: %s", errMsg)))
		b.WriteString("\n\n")
		b.WriteString(promptStyle.Render("[r]etry  [s]ave anyway  [q]uit"))
		b.WriteString("\n")
	}

	return b.String()
}

// Result returns the entered values.
func (m SetupModel) Result() (appToken, userKey, backend, dataDir string) {
	return m.inputs[0].Value(), m.inputs[1].Value(), m.inputs[2].Value(), m.inputs[3].Value()
}

// ShouldSave returns true if the wizard completed and the user did not cancel.
func (m SetupModel) ShouldSave() bool {
	return m.step == StepDone && !m.quitting
}
