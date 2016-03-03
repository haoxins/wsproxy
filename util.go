package wsproxy

import "net/http"
import "strings"

func IsWebSocketRequest(r *http.Request) bool {
	hasHeader := func(key, val string) bool {
		arr := strings.Split(r.Header.Get(key), ",")
		for _, v := range arr {
			if val == strings.ToLower(strings.TrimSpace(v)) {
				return true
			}
		}
		return false
	}

	if !hasHeader("Connection", "upgrade") {
		return false
	}
	if !hasHeader("Upgrade", "websocket") {
		return false
	}

	return true
}
