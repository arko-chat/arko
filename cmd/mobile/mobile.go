package mobile

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/arko-chat/arko/internal/bridge"
	"github.com/arko-chat/arko/internal/handlers"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/router"
	"github.com/arko-chat/arko/internal/service"
	"github.com/arko-chat/arko/internal/ws"
)

var (
	mu       sync.Mutex
	stopFunc func()
)

func RegisterBridge(b bridge.NativeBridge) {
	bridge.Register(b)
}

func Start(dataDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if stopFunc != nil {
		return "", fmt.Errorf("server already running")
	}

	if _, err := bridge.Safe(); err != nil {
		return "", fmt.Errorf("call RegisterBridge before Start: %w", err)
	}

	slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	cryptoDBPath := dataDir + "/cryptodb"
	if err := os.MkdirAll(cryptoDBPath, 0700); err != nil {
		return "", fmt.Errorf("failed to create crypto db directory: %w", err)
	}

	mgr := matrix.NewManager(slogger, cryptoDBPath)
	wsHub := ws.NewHub(slogger)
	svc := service.New(mgr, wsHub)
	h := handlers.New(wsHub, svc, slogger)
	mux := router.New(h, mgr)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to listen: %w", err)
	}

	addr := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	slogger.Info("mobile server starting", "addr", addr)

	srv := &http.Server{Handler: mux}

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			slogger.Error("server error", "err", err)
		}
	}()

	stopFunc = func() {
		srv.Close()
		listener.Close()
		mgr.Shutdown()
	}

	return addr, nil
}

func Stop() {
	mu.Lock()
	defer mu.Unlock()

	if stopFunc != nil {
		stopFunc()
		stopFunc = nil
	}
}
