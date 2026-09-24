package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/profora/myankafe-finance/backend/internal/auth"
)

type captureWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{header: make(http.Header)}
}

func (w *captureWriter) Header() http.Header {
	return w.header
}

func (w *captureWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
}

func (w *captureWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func copyCaptured(dst http.ResponseWriter, src *captureWriter) {
	for k, values := range src.header {
		for _, v := range values {
			dst.Header().Add(k, v)
		}
	}
	if src.status == 0 {
		src.status = http.StatusOK
	}
	dst.WriteHeader(src.status)
	_, _ = dst.Write(src.body.Bytes())
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (s *Server) idempotency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isMutation(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		if len(key) > 200 {
			fail(w, http.StatusBadRequest, errString("Idempotency-Key must be 200 characters or fewer"))
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		sum := sha256.Sum256(body)
		requestHash := hex.EncodeToString(sum[:])

		p, ok := auth.From(r.Context())
		if !ok {
			fail(w, http.StatusUnauthorized, errString("unauthorized"))
			return
		}

		scope := p.PublicID + ":" + r.Method + ":" + r.URL.RequestURI()
		claimed, existing, err := s.Store.ClaimIdempotency(r.Context(), scope, key, requestHash)
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}

		if !claimed {
			if existing.RequestHash != requestHash {
				fail(w, http.StatusConflict, errString("Idempotency-Key was already used with a different request"))
				return
			}
			if existing.ResponseStatus == nil {
				fail(w, http.StatusConflict, errString("request with this Idempotency-Key is already in progress"))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Idempotency-Replayed", "true")
			w.WriteHeader(*existing.ResponseStatus)
			if existing.ResponseBody != nil {
				_, _ = w.Write([]byte(*existing.ResponseBody))
			}
			return
		}

		rec := newCaptureWriter()
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}

		if rec.status >= http.StatusInternalServerError {
			_ = s.Store.ReleaseIdempotency(r.Context(), scope, key)
			copyCaptured(w, rec)
			return
		}

		if err := s.Store.CompleteIdempotency(r.Context(), scope, key, rec.status, rec.body.String()); err != nil {
			_ = s.Store.ReleaseIdempotency(r.Context(), scope, key)
			fail(w, http.StatusInternalServerError, err)
			return
		}

		copyCaptured(w, rec)
	})
}
