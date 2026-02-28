package mobile

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/arko-chat/arko/internal/handlers"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/router"
	"github.com/arko-chat/arko/internal/service"
	chatws "github.com/arko-chat/arko/internal/ws/chat"
	verifyws "github.com/arko-chat/arko/internal/ws/verify"
)

var (
	mu       sync.Mutex
	stopFunc func()
)

func Start(dataDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if stopFunc != nil {
		return "", fmt.Errorf("server already running")
	}

	slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	cryptoDBPath := dataDir + "/cryptodb"
	if err := os.MkdirAll(cryptoDBPath, 0700); err != nil {
		return "", fmt.Errorf("failed to create crypto db directory: %w", err)
	}

	chatHub := chatws.NewHub(slogger)
	verifyHub := verifyws.NewHub(slogger)

	mgr := matrix.NewManager(slogger, cryptoDBPath)

	svc := service.New(mgr, chatHub, verifyHub)
	h := handlers.New(svc, slogger)
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
