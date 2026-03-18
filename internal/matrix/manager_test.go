package matrix

import (
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestNewManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mgr := NewManager(logger, t.TempDir())
	if mgr == nil {
		t.Fatal("expected manager, got nil")
	}

	if mgr.GetContext() == nil {
		t.Error("expected context, got nil")
	}

	mgr.Shutdown()
}

func TestManager_HasClient_NonexistentUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mgr := NewManager(logger, t.TempDir())
	defer mgr.Shutdown()

	if mgr.HasClient("@definitely_does_not_exist_12345:example.com") {
		t.Error("expected no client for nonexistent user")
	}
}

func TestManager_GetClient_NonexistentUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mgr := NewManager(logger, t.TempDir())
	defer mgr.Shutdown()

	client, err := mgr.GetClient("@definitely_does_not_exist_12345:example.com")
	if !errors.Is(err, ErrNoClient) {
		t.Errorf("expected ErrNoClient, got %v", err)
	}
	if client != nil {
		t.Error("expected nil client")
	}
}

func TestManager_GetMatrixSession_NonexistentUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mgr := NewManager(logger, t.TempDir())
	defer mgr.Shutdown()

	sess := mgr.GetMatrixSession("@definitely_does_not_exist_12345:example.com")
	if sess != nil {
		t.Error("expected nil session for nonexistent user")
	}
}

func TestManager_GetVerificationState_NonexistentUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mgr := NewManager(logger, t.TempDir())
	defer mgr.Shutdown()

	state := mgr.GetVerificationState("@definitely_does_not_exist_12345:example.com")
	if state != nil {
		t.Error("expected nil verification state")
	}
}

func TestManager_Shutdown_Idempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mgr := NewManager(logger, t.TempDir())

	mgr.Shutdown()
	mgr.Shutdown()
}
