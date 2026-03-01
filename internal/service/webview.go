package service

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/arko-chat/arko/internal/matrix"
	"github.com/arko-chat/arko/internal/webview"
	"github.com/arko-chat/arko/internal/ws"
	"github.com/puzpuzpuz/xsync/v4"
)

const BASE_TITLE = "Arko"

type WebViewService struct {
	*BaseService

	mu           sync.Mutex
	mainWindow   webview.WebView
	title        string
	childWindows *xsync.Map[string, webview.WebView]
}

func NewWebViewService(
	mgr *matrix.Manager,
	hub *ws.Hub,
) *WebViewService {
	return &WebViewService{
		BaseService:  NewBaseService(mgr, hub),
		childWindows: xsync.NewMap[string, webview.WebView](),
	}
}

func (s *WebViewService) InitializeWebView(baseUrl string) error {
	w := webview.New(true)
	w.SetTitle(BASE_TITLE)
	s.title = BASE_TITLE
	w.SetSize(1040, 768, webview.HintMin)
	w.Navigate(baseUrl)
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

	s.mu.Lock()
	s.mainWindow = w
	s.mu.Unlock()

	w.Run()
	return nil
}

func (s *WebViewService) GetTitle() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.title
}

func (s *WebViewService) SetTitle(title string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		s.title = BASE_TITLE
	} else {
		s.title = fmt.Sprintf("%s | %s", BASE_TITLE, trimmed)
	}

	if s.mainWindow != nil {
		newTitle := s.title
		s.mainWindow.Dispatch(func() {
			s.mainWindow.SetTitle(newTitle)
		})
	}
}

func (s *WebViewService) OpenChildWindow(id, title, url string, width, height int) {
	if _, exists := s.childWindows.Load(id); exists {
		return
	}

	go func() {
		w := webview.New(false)
		w.SetTitle(title)
		w.SetSize(width, height, webview.HintNone)
		w.Navigate(url)

		s.childWindows.Store(id, w)
		w.Run()

		s.childWindows.Delete(id)
	}()
}

func (s *WebViewService) CloseChildWindow(id string) {
	w, exists := s.childWindows.Load(id)

	if !exists {
		return
	}

	w.Dispatch(func() {
		w.Destroy()
	})
}

func (s *WebViewService) CloseMainWindow() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mainWindow == nil {
		return
	}

	s.mainWindow.Dispatch(func() {
		s.mainWindow.Destroy()
	})
	s.mainWindow = nil
}
