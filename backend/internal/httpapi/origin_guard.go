package httpapi

import (
	"errors"
	"net/http"
	"strings"
)

func (s *Server) originGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" && origin != s.Config.CORSOrigin {
				fail(w, http.StatusForbidden, errors.New("origin not allowed"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
