package bridge

// NativeBridge is implemented by the native side (Swift/Kotlin).
// gomobile exposes this as an interface that native code can satisfy.
//
// Rules for gomobile compatibility:
//   - methods may only use primitive types, strings, []byte, or other
//     gomobile-bound types as parameters and return values
//   - no variadic parameters
//   - errors are returned as a second return value
type NativeBridge interface {
	// GetDeviceID returns a stable unique device identifier.
	GetDeviceID() (string, error)

	// ShowNotification fires a local push notification.
	ShowNotification(title string, body string) error

	// AuthenticateBiometric prompts Face ID / fingerprint.
	// Returns true if the user authenticated successfully.
	AuthenticateBiometric(reason string) (bool, error)

	// ShareText opens the native share sheet with the given text.
	ShareText(text string) error

	// GetSafeAreaInsets returns top,right,bottom,left inset values
	// in points/dp so Go templates can adjust layout.
	GetSafeAreaInsets() (top int, right int, bottom int, left int, err error)

	// OpenURL opens a URL in the system browser (not the WebView).
	OpenURL(url string) error

	// OnAppBackground is called by Go when it detects the app going
	// to the background (you call this from native lifecycle hooks too).
	OnAppBackground() error

	// OnAppForeground is called by Go when the app returns to foreground.
	OnAppForeground() error
}
