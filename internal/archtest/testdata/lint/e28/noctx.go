package e28

import "net/http"

// Fetch calls HTTP without the caller's context.
func Fetch(url string) (*http.Response, error) {
	return http.Get(url)
}
