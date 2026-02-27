package urlpreviews

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/arko-chat/arko/internal/models"
	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

var (
	hostLimiters   = make(map[string]*rate.Limiter)
	hostLimitersMu sync.Mutex

	client = &http.Client{
		Timeout: 10 * time.Second,
	}
)

func limiterForHost(host string) *rate.Limiter {
	hostLimitersMu.Lock()
	defer hostLimitersMu.Unlock()

	if l, ok := hostLimiters[host]; ok {
		return l
	}

	l := rate.NewLimiter(rate.Every(2*time.Second), 1)
	hostLimiters[host] = l
	return l
}

func FetchEmbed(rawURL string) (*models.Embed, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := limiterForHost(parsed.Host).Wait(ctx); err != nil {
		return nil, err
	}

	return fetchHTMLEmbed(rawURL)
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) " +
	"Chrome/122.0.0.0 Safari/537.36"

func fetchHTMLEmbed(rawURL string) (*models.Embed, error) {
	req, err := newRequest(rawURL)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited by %s", req.URL.Host)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"unexpected status %d from %s",
			res.StatusCode,
			req.URL.Host,
		)
	}

	body, err := readBody(res)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, err
	}

	embed := &models.Embed{URL: rawURL}

	titleNode := doc.Find("html > head > title")
	if titleNode.Length() == 0 {
		return nil, errors.New("title not found")
	}
	embed.Title = titleNode.Text()

	doc.Find("html > head > link").Each(func(_ int, s *goquery.Selection) {
		for _, node := range s.Nodes {
			for _, attr := range node.Attr {
				if strings.ToLower(attr.Key) == "rel" && attr.Val == "icon" {
					embed.ImageURL = resolveFavicon(rawURL, node)
				}
			}
		}
	})

	doc.Find("html > head > meta").Each(func(_ int, s *goquery.Selection) {
		for _, node := range s.Nodes {
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "property":
					applyMetaProperty(
						strings.ToLower(attr.Val),
						node,
						embed,
					)
				case "name":
					if strings.ToLower(attr.Val) == "description" &&
						embed.Description == "" {
						embed.Description = metaContent(node)
					}
				}
			}
		}
	})

	return embed, nil
}

func newRequest(rawURL string) (*http.Request, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	return req, nil
}

func readBody(res *http.Response) (io.Reader, error) {
	if res.Header.Get("Content-Encoding") == "gzip" {
		return gzip.NewReader(res.Body)
	}
	return res.Body, nil
}

func resolveFavicon(rawURL string, node *html.Node) string {
	var link string
	for _, attr := range node.Attr {
		if strings.ToLower(attr.Key) == "href" {
			link = attr.Val
			break
		}
	}

	if link == "" {
		return ""
	}

	if strings.HasPrefix(link, "http://") ||
		strings.HasPrefix(link, "https://") {
		return link
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return (&url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   link,
	}).String()
}

func metaContent(node *html.Node) string {
	for _, attr := range node.Attr {
		if strings.ToLower(attr.Key) == "content" {
			return attr.Val
		}
	}
	return ""
}

func applyMetaProperty(
	nodeType string,
	node *html.Node,
	embed *models.Embed,
) {
	if !strings.HasPrefix(nodeType, "og:") {
		return
	}

	parts := strings.SplitN(nodeType, ":", 2)
	if len(parts) != 2 {
		return
	}

	content := metaContent(node)

	switch parts[1] {
	case "title":
		embed.Title = content
	case "description":
		embed.Description = content
	case "image":
		embed.ImageURL = content
	case "site_name":
		embed.SiteName = content
	}
}
