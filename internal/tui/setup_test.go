// ABOUTME: Unit tests for the push setup TUI wizard bubbletea model.
// ABOUTME: Uses synthetic tea.Msg values to test state machine transitions.
package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// mustSetupModel extracts SetupModel from a tea.Model or fails the test.
func mustSetupModel(t *testing.T, model tea.Model) SetupModel {
	t.Helper()
	m, ok := model.(SetupModel)
	if !ok {
		t.Fatalf("expected SetupModel, got %T", model)
	}
	return m
}

func TestNewSetupModel_DefaultValues(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	if m.step != StepAppToken {
		t.Errorf("expected initial step StepAppToken, got %d", m.step)
	}
	if m.inputs[0].Value() != "" {
		t.Error("expected empty app token input for new config")
	}
}

func TestNewSetupModel_ExistingConfig(t *testing.T) {
	m := NewSetupModel("token123", "key456", "markdown", "/custom/path")
	if m.inputs[0].Value() != "token123" {
		t.Errorf("expected pre-filled app token, got %q", m.inputs[0].Value())
	}
	if m.inputs[1].Value() != "key456" {
		t.Errorf("expected pre-filled user key, got %q", m.inputs[1].Value())
	}
	if m.inputs[2].Value() != "markdown" {
		t.Errorf("expected pre-filled backend, got %q", m.inputs[2].Value())
	}
	if m.inputs[3].Value() != "/custom/path" {
		t.Errorf("expected pre-filled data dir, got %q", m.inputs[3].Value())
	}
}

func TestSetupModel_StepTransitions(t *testing.T) {
	m := NewSetupModel("", "", "", "")

	// Set token and advance
	m.inputs[0].SetValue("token")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, updated)
	if m.step != StepUserKey {
		t.Errorf("expected StepUserKey, got %d", m.step)
	}

	// Set user key and advance
	m.inputs[1].SetValue("key")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, updated)
	if m.step != StepBackend {
		t.Errorf("expected StepBackend, got %d", m.step)
	}

	// Default backend and advance
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, updated)
	if m.step != StepDataDir {
		t.Errorf("expected StepDataDir, got %d", m.step)
	}
	if m.inputs[2].Value() != "sqlite" {
		t.Errorf("expected default backend 'sqlite', got %q", m.inputs[2].Value())
	}

	// Default data dir and start validation
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, updated)
	if m.step != StepValidating {
		t.Errorf("expected StepValidating, got %d", m.step)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for validation + spinner")
	}
}

func TestSetupModel_EmptyAppTokenBlocked(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, updated)
	if m.step != StepAppToken {
		t.Errorf("expected to stay on StepAppToken with empty input, got %d", m.step)
	}
}

func TestSetupModel_EmptyUserKeyBlocked(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepUserKey
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, updated)
	if m.step != StepUserKey {
		t.Errorf("expected to stay on StepUserKey with empty input, got %d", m.step)
	}
}

func TestSetupModel_InvalidBackend(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepBackend
	m.inputs[2].SetValue("invalid")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, updated)
	if m.step != StepBackend {
		t.Errorf("expected to stay on StepBackend with invalid backend, got %d", m.step)
	}
}

func TestSetupModel_ValidationSuccess(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.validateFn = func(_ context.Context, _, _ string) error { return nil }
	m.step = StepValidating

	updated, _ := m.Update(validationResultMsg{err: nil})
	m = mustSetupModel(t, updated)
	if m.step != StepDone {
		t.Errorf("expected StepDone after successful validation, got %d", m.step)
	}
}

func TestSetupModel_ValidationFailure(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepValidating

	updated, _ := m.Update(validationResultMsg{err: fmt.Errorf("connection refused")})
	m = mustSetupModel(t, updated)
	if m.step != StepFailed {
		t.Errorf("expected StepFailed after validation error, got %d", m.step)
	}
}

func TestSetupModel_FailedRetry(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepFailed
	m.validationErr = fmt.Errorf("some error")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = mustSetupModel(t, updated)
	if m.step != StepValidating {
		t.Errorf("expected StepValidating after retry, got %d", m.step)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd on retry")
	}
}

func TestSetupModel_FailedSaveAnyway(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepFailed

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = mustSetupModel(t, updated)
	if m.step != StepDone {
		t.Errorf("expected StepDone after save anyway, got %d", m.step)
	}
}

func TestSetupModel_FailedQuit(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepFailed

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = mustSetupModel(t, updated)
	if cmd == nil {
		t.Error("expected quit cmd")
	}
	if !m.quitting {
		t.Error("expected quitting to be true after 'q'")
	}
	if m.ShouldSave() {
		t.Error("expected ShouldSave false after quit")
	}
}

func TestSetupModel_QuitOnCtrlC(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = mustSetupModel(t, updated)
	if cmd == nil {
		t.Error("expected quit cmd on ctrl+c")
	}
	if !m.quitting {
		t.Error("expected quitting to be true")
	}
}

func TestSetupModel_QuitOnEsc(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = mustSetupModel(t, updated)
	if cmd == nil {
		t.Error("expected quit cmd on escape")
	}
	if !m.quitting {
		t.Error("expected quitting to be true")
	}
}

func TestSetupModel_Result(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.inputs[0].SetValue("token-123")
	m.inputs[1].SetValue("key-456")
	m.inputs[2].SetValue("sqlite")
	m.inputs[3].SetValue("/data/push")
	m.step = StepDone

	appToken, userKey, backend, dataDir := m.Result()
	if appToken != "token-123" {
		t.Errorf("expected app token, got %q", appToken)
	}
	if userKey != "key-456" {
		t.Errorf("expected user key, got %q", userKey)
	}
	if backend != "sqlite" {
		t.Errorf("expected backend, got %q", backend)
	}
	if dataDir != "/data/push" {
		t.Errorf("expected data dir, got %q", dataDir)
	}
}

func TestSetupModel_ShouldSave(t *testing.T) {
	t.Run("done means save", func(t *testing.T) {
		m := NewSetupModel("", "", "", "")
		m.step = StepDone
		if !m.ShouldSave() {
			t.Error("expected ShouldSave true when done")
		}
	})

	t.Run("quit means no save", func(t *testing.T) {
		m := NewSetupModel("", "", "", "")
		m.quitting = true
		if m.ShouldSave() {
			t.Error("expected ShouldSave false when quitting")
		}
	})

	t.Run("save anyway means save", func(t *testing.T) {
		m := NewSetupModel("", "", "", "")
		m.step = StepFailed
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		m = mustSetupModel(t, updated)
		if !m.ShouldSave() {
			t.Error("expected ShouldSave true after save anyway")
		}
	})
}

func TestSetupModel_ViewContainsBranding(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	view := m.View()
	if !strings.Contains(view, "PUSH CANNON ULTRA") {
		t.Error("expected view to contain PUSH CANNON ULTRA branding")
	}
}

func TestSetupModel_ViewShowsCurrentStep(t *testing.T) {
	m := NewSetupModel("", "", "", "")

	m.step = StepAppToken
	if !strings.Contains(m.View(), "App Token") {
		t.Error("expected StepAppToken view to mention App Token")
	}

	m.step = StepUserKey
	if !strings.Contains(m.View(), "User Key") {
		t.Error("expected StepUserKey view to mention User Key")
	}

	m.step = StepBackend
	if !strings.Contains(m.View(), "Backend") {
		t.Error("expected StepBackend view to mention Backend")
	}

	m.step = StepDataDir
	if !strings.Contains(m.View(), "Data Directory") {
		t.Error("expected StepDataDir view to mention Data Directory")
	}
}

func TestSetupModel_ViewValidating(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepValidating
	view := m.View()
	if !strings.Contains(view, "Validating credentials") {
		t.Error("expected StepValidating view to mention Validating credentials")
	}
}

func TestSetupModel_ViewDone(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepDone
	view := m.View()
	if !strings.Contains(view, "Connected") {
		t.Error("expected StepDone view to mention Connected")
	}
}

func TestSetupModel_ViewFailed(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepFailed
	m.validationErr = fmt.Errorf("timeout")
	view := m.View()
	if !strings.Contains(view, "Validation failed") {
		t.Error("expected StepFailed view to mention Validation failed")
	}
	if !strings.Contains(view, "timeout") {
		t.Error("expected StepFailed view to show error message")
	}
	if !strings.Contains(view, "[r]etry") {
		t.Error("expected StepFailed view to show retry option")
	}
}

func TestSetupModel_ViewFailedNilError(t *testing.T) {
	m := NewSetupModel("", "", "", "")
	m.step = StepFailed
	view := m.View()
	if strings.Contains(view, "<nil>") {
		t.Error("expected nil error to be rendered gracefully")
	}
	if !strings.Contains(view, "unknown error") {
		t.Error("expected nil error to show 'unknown error' fallback")
	}
}

func TestSetupModel_CtrlCDuringValidation(t *testing.T) {
	cancelled := false
	m := NewSetupModel("", "", "", "")
	m.validateFn = func(ctx context.Context, _, _ string) error {
		<-ctx.Done()
		cancelled = true
		return ctx.Err()
	}
	m.inputs[0].SetValue("token")
	m.inputs[1].SetValue("key")
	m.inputs[2].SetValue("sqlite")
	m.inputs[3].SetValue("/data")
	m.step = StepDataDir

	// Press Enter to start validation
	updated, batchCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, updated)
	if m.step != StepValidating {
		t.Fatalf("expected StepValidating, got %d", m.step)
	}

	batchMsg, ok := batchCmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("expected batch command from validation start")
	}
	done := make(chan tea.Msg)
	go func() {
		done <- batchMsg[0]()
	}()

	// Press Ctrl+C
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = mustSetupModel(t, updated)
	if !m.quitting {
		t.Error("expected quitting to be true")
	}

	<-done
	if !cancelled {
		t.Error("expected validation context to be cancelled")
	}
}

func TestSetupModel_FullPrefilledFlow(t *testing.T) {
	m := NewSetupModel("token", "key", "sqlite", "/data")
	m.validateFn = func(_ context.Context, _, _ string) error { return nil }

	// Enter on pre-filled app token
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, u)
	if m.step != StepUserKey {
		t.Fatalf("expected StepUserKey, got %d", m.step)
	}

	// Enter on pre-filled user key
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, u)
	if m.step != StepBackend {
		t.Fatalf("expected StepBackend, got %d", m.step)
	}

	// Enter on pre-filled backend
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, u)
	if m.step != StepDataDir {
		t.Fatalf("expected StepDataDir, got %d", m.step)
	}

	// Enter on pre-filled data dir -> start validation
	u, batchCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustSetupModel(t, u)
	if m.step != StepValidating {
		t.Fatalf("expected StepValidating, got %d", m.step)
	}

	batchMsg, ok := batchCmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("expected batch command from validation start")
	}
	resultMsg := batchMsg[0]()
	u, _ = m.Update(resultMsg)
	m = mustSetupModel(t, u)
	if m.step != StepDone {
		t.Errorf("expected StepDone, got %d", m.step)
	}
	if !m.ShouldSave() {
		t.Error("expected ShouldSave true")
	}
}

func TestSetupModel_FullFlowWithTeaProgram(t *testing.T) {
	m := NewSetupModel("token", "key", "sqlite", "/data")
	m.validateFn = func(_ context.Context, _, _ string) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithoutRenderer())

	go func() {
		p.Send(tea.KeyMsg{Type: tea.KeyEnter}) // App Token
		p.Send(tea.KeyMsg{Type: tea.KeyEnter}) // User Key
		p.Send(tea.KeyMsg{Type: tea.KeyEnter}) // Backend
		p.Send(tea.KeyMsg{Type: tea.KeyEnter}) // Data Dir -> validates -> done
	}()

	result, err := p.Run()
	if err != nil {
		t.Fatalf("tea.Program error: %v", err)
	}

	final, ok := result.(SetupModel)
	if !ok {
		t.Fatalf("expected SetupModel, got %T", result)
	}
	if !final.ShouldSave() {
		t.Errorf("expected ShouldSave=true, got false (step=%d, quitting=%v)", final.step, final.quitting)
	}
}
