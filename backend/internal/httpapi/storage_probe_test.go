package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/profora/myankafe-finance/backend/internal/objectstore"
)

type memoryObjectStore struct {
	configured bool
	objects    map[string][]byte
	failPut    bool
	failOpen   bool
	failDelete bool
}

func (m *memoryObjectStore) Configured() bool { return m.configured }

func (m *memoryObjectStore) Put(_ context.Context, key, _ string, body []byte) error {
	if m.failPut {
		return errors.New("put failed")
	}
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	m.objects[key] = append([]byte(nil), body...)
	return nil
}

func (m *memoryObjectStore) Open(_ context.Context, key string) (objectstore.ReadObject, error) {
	if m.failOpen {
		return objectstore.ReadObject{}, errors.New("open failed")
	}
	body, ok := m.objects[key]
	if !ok {
		return objectstore.ReadObject{}, errors.New("not found")
	}
	return objectstore.ReadObject{
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentType:   "text/plain",
		ContentLength: int64(len(body)),
	}, nil
}

func (m *memoryObjectStore) Delete(_ context.Context, key string) error {
	if m.failDelete {
		return errors.New("delete failed")
	}
	delete(m.objects, key)
	return nil
}

func TestProbeAttachmentStoreSuccess(t *testing.T) {
	store := &memoryObjectStore{configured: true, objects: map[string][]byte{}}

	result, err := probeAttachmentStore(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatal("probe should report success")
	}
	if len(store.objects) != 0 {
		t.Fatalf("probe object was not removed: %+v", store.objects)
	}
}

func TestProbeAttachmentStoreRejectsUnconfigured(t *testing.T) {
	_, err := probeAttachmentStore(context.Background(), &memoryObjectStore{})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err=%v", err)
	}
}

func TestProbeAttachmentStorePropagatesDeleteFailure(t *testing.T) {
	store := &memoryObjectStore{
		configured: true,
		objects:    map[string][]byte{},
		failDelete: true,
	}

	_, err := probeAttachmentStore(context.Background(), store)
	if err == nil || !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("err=%v", err)
	}
}
