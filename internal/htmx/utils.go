package htmx

import (
	"net/http"
)

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
