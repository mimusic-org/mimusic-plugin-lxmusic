//go:build wasip1

package handlers

import (
	"net/http"
	"strings"
)

func isTVRequest(req *http.Request) bool {
	return strings.Contains(req.URL.Path, "/lxmusic/api/tv/")
}
