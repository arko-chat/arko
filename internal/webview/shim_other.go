//go:build !windows
// +build !windows

package webview

import (
	"unsafe"

	"github.com/arko-chat/arko/internal/webview/webview"
)

type WebView = webview.WebView

const (
	// HintNone specifies that width and height are default size
	HintNone = webview.HintNone

	// HintFixed specifies that window size can not be changed by a user
	HintFixed = webview.HintFixed

	// HintMin specifies that width and height are minimum bounds
	HintMin = webview.HintMin

	// HintMax specifies that width and height are maximum bounds
	HintMax = webview.HintMax
)

// New creates a new webview in a new window.
func New(debug bool) WebView {
	return webview.New(debug)
}

// NewWindow creates a new webview using an existing window.
func NewWindow(debug bool, window unsafe.Pointer) WebView {
	return webview.NewWindow(debug, window)
}
