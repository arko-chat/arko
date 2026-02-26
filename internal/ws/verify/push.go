package verifyws

import "fmt"

func RedirectFragment(path string) []byte {
	return []byte(fmt.Sprintf(
		`<div id="ws-verify-redirect" hx-swap-oob="true"><script>window.location.href="%s";</script></div>`,
		path,
	))
}
