package httpapi

import (
	"net/http"
	"strings"
)

const maxJSONRequestBytes int64 = 1 << 20

func (s *Server) bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		switch r.Method{
		case http.MethodPost,http.MethodPut,http.MethodPatch,http.MethodDelete:
			contentType:=strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
			if !strings.HasPrefix(contentType,"multipart/form-data"){
				r.Body=http.MaxBytesReader(w,r.Body,maxJSONRequestBytes)
			}
		}
		next.ServeHTTP(w,r)
	})
}
