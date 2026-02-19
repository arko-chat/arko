package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/arko-chat/arko/internal/config"
	"github.com/arko-chat/arko/internal/handlers"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/router"
	"github.com/arko-chat/arko/internal/service"
	"github.com/arko-chat/arko/internal/session"
	"github.com/arko-chat/arko/internal/ws"
	webview "github.com/webview/webview_go"
)

func main() {
	slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	cfg, err := config.Load()
	if err != nil {
		slogger.Error("failed to generate config", "err", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.CryptoDBPath, 0700); err != nil {
		slogger.Error("failed to create crypto db directory", "err", err)
		os.Exit(1)
	}

	hub := ws.NewHub(slogger)
	sessionStore := session.NewStore([]byte(cfg.SessionSecret))

	mgr := matrix.NewManager(
		hub,
		slogger,
		cfg.CryptoDBPath,
		[]byte(cfg.PickleKey),
	)

	mgr.RestoreAllSessions()

	svc := service.NewChatService(mgr, hub)
	h := handlers.New(svc, slogger)
	mux := router.New(h, sessionStore, mgr)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}

	addr := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	slogger.Info("server starting", "addr", addr)

	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Fatal(err)
		}
	}()

	w := webview.New(true)
	defer w.Destroy()
	w.SetTitle("Arko")
	w.SetSize(1040, 768, webview.HintMin)
	w.SetSize(1280, 800, webview.HintMax)
	w.Navigate(addr)
	w.Run()

	slogger.Info("window closed, shutting down")
	mgr.Shutdown()
}
