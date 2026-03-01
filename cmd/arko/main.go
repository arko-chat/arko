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
	"github.com/arko-chat/arko/internal/ws"
)

func main() {
	slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "0")
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--enable-gpu")

	cfg, err := config.Load()
	if err != nil {
		slogger.Error("failed to generate config", "err", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.CryptoDBPath, 0700); err != nil {
		slogger.Error("failed to create crypto db directory", "err", err)
		os.Exit(1)
	}

	mgr := matrix.NewManager(
		slogger,
		cfg.CryptoDBPath,
	)

	wsHub := ws.NewHub(slogger)
	svc := service.New(mgr, wsHub)
	h := handlers.New(wsHub, svc, slogger)
	mux := router.New(h, mgr)

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

	svc.WebView.InitializeWebView(addr)
	defer svc.WebView.CloseMainWindow()

	slogger.Info("window closed, shutting down")
	mgr.Shutdown()
}
