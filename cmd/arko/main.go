package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/arko-chat/arko/internal/config"
	"github.com/arko-chat/arko/internal/handlers"
	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/router"
	"github.com/arko-chat/arko/internal/service"
	"github.com/arko-chat/arko/internal/ws"
	webview "github.com/webview/webview_go"
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

	w := webview.New(true)
	defer w.Destroy()
	w.SetTitle("Arko")
	w.SetSize(1040, 768, webview.HintMin)
	w.Navigate(addr)
	w.Init(`
    document.addEventListener("click", function(e) {
        const a = e.target.closest("a");
        if (!a || !a.href) return;
        const url = a.href;
        if (url.startsWith("http://127.0.0.1") || url.startsWith("/")) return;
        e.preventDefault();
        openExternal(url);
    });
`)

	w.Bind("openExternal", func(url string) error {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		return cmd.Start()
	})
	w.Run()

	slogger.Info("window closed, shutting down")
	mgr.Shutdown()
}
