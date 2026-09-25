package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/ids"
	"github.com/profora/myankafe-finance/backend/internal/objectstore"
)

type storageProbeResult struct {
	OK         bool    `json:"ok"`
	DurationMS float64 `json:"duration_ms"`
}

func probeAttachmentStore(ctx context.Context, store objectstore.Store) (storageProbeResult, error) {
	if store == nil || !store.Configured() {
		return storageProbeResult{}, errors.New("attachment storage is not configured")
	}

	probeID, err := ids.ULID()
	if err != nil {
		return storageProbeResult{}, err
	}
	key := "system/probes/" + probeID + ".txt"
	payload := []byte("myankafe-finance-r2-probe:" + probeID)

	start := time.Now()
	if err := store.Put(ctx, key, "text/plain", payload); err != nil {
		return storageProbeResult{}, err
	}
	cleanup := func() {
		_ = store.Delete(context.Background(), key)
	}

	obj, err := store.Open(ctx, key)
	if err != nil {
		cleanup()
		return storageProbeResult{}, err
	}
	body, readErr := io.ReadAll(io.LimitReader(obj.Body, int64(len(payload))+1))
	closeErr := obj.Body.Close()
	if readErr != nil {
		cleanup()
		return storageProbeResult{}, readErr
	}
	if closeErr != nil {
		cleanup()
		return storageProbeResult{}, closeErr
	}
	if !bytes.Equal(body, payload) {
		cleanup()
		return storageProbeResult{}, errors.New("attachment storage probe read-back mismatch")
	}
	if err := store.Delete(ctx, key); err != nil {
		return storageProbeResult{}, err
	}

	return storageProbeResult{
		OK:         true,
		DurationMS: float64(time.Since(start).Microseconds()) / 1000,
	}, nil
}

func (s *Server) storageProbe(w http.ResponseWriter, r *http.Request) {
	user, err := s.principal(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, user)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if !isOwner {
		fail(w, http.StatusForbidden, errors.New("OWNER access required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	result, err := probeAttachmentStore(ctx, s.AttachmentStore)
	if err != nil {
		_ = s.Store.Audit(
			r.Context(),
			user,
			nil,
			"R2_STORAGE_PROBE",
			"OBJECT_STORAGE",
			nil,
			"FAILED",
			map[string]any{"error": err.Error()},
		)
		fail(w, http.StatusServiceUnavailable, err)
		return
	}

	_ = s.Store.Audit(
		r.Context(),
		user,
		nil,
		"R2_STORAGE_PROBE",
		"OBJECT_STORAGE",
		nil,
		"SUCCESS",
		map[string]any{"duration_ms": result.DurationMS},
	)
	write(w, http.StatusOK, result)
}

func (s *Server) systemStatus(w http.ResponseWriter, r *http.Request) {
	user, err := s.principal(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, err)
		return
	}
	isOwner, err := s.ownerAnywhere(r, user)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if !isOwner {
		fail(w, http.StatusForbidden, errors.New("OWNER access required"))
		return
	}

	metricsTokenConfigured := s.Config.MetricsBearerToken != ""
	write(w, http.StatusOK, map[string]any{
		"app_env":                       s.Config.AppEnv,
		"database":                      "ok",
		"attachment_storage_configured": s.AttachmentStore != nil && s.AttachmentStore.Configured(),
		"metrics_available":             s.Config.AppEnv != "production" || metricsTokenConfigured,
		"metrics_protected":             metricsTokenConfigured,
	})
}
