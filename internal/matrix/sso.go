package matrix

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/toqueteos/webbrowser"
	"maunium.net/go/mautrix"
)

func (m *Manager) GetSSOToken(ctx context.Context, client *mautrix.Client) (string, error) {
	var wg sync.WaitGroup
	ssoCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	wg.Go(func() {
		<-ssoCtx.Done()
		listener.Close()
	})

	addr := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	ssoBaseUrl := client.BuildClientURL("v3", "login", "sso", "redirect")
	ssoUrl, err := url.Parse(ssoBaseUrl)
	if err != nil {
		return "", err
	}
	q := ssoUrl.Query()
	q.Add("redirectUrl", addr)
	ssoUrl.RawQuery = q.Encode()

	token := ""

	wg.Go(func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			token = r.URL.Query().Get("loginToken")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			htmlResponse := `
<!DOCTYPE html>
<html>
<head>
    <title>Closing Tab</title>
    <script type="text/javascript">
        function closeWindow() {
            window.close();
        }
        window.onload = closeWindow;
    </script>
</head>
<body>
    <p>If the tab does not close automatically, it is likely due to browser security restrictions.</p>
    <p>You can manually close this tab.</p>
    <button onclick="closeWindow()">Close Window</button>
</body>
</html>`
			w.Write([]byte(htmlResponse))
			cancel()
		})
		httpActive := make(chan struct{})
		go func() {
			if err := http.Serve(listener, mux); err != nil {
				close(httpActive)
			}
		}()
		_ = webbrowser.Open(ssoUrl.String())
		<-httpActive
	})

	wg.Wait()

	return token, nil
}
