package common

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertBodyStorageReadersAreIndependent(t *testing.T, storage BodyStorage, payload []byte) {
	t.Helper()
	first, err := storage.NewReader()
	require.NoError(t, err)
	second, err := storage.NewReader()
	require.NoError(t, err)

	prefix := make([]byte, 5)
	_, err = io.ReadFull(first, prefix)
	require.NoError(t, err)
	assert.Equal(t, payload[:5], prefix)

	secondPayload, err := io.ReadAll(second)
	require.NoError(t, err)
	assert.Equal(t, payload, secondPayload)

	firstRemainder, err := io.ReadAll(first)
	require.NoError(t, err)
	assert.Equal(t, payload[5:], firstRemainder)

	require.NoError(t, first.Close())
	require.NoError(t, second.Close())
	storedPayload, err := storage.Bytes()
	require.NoError(t, err, "closing replay readers must not close the storage")
	assert.Equal(t, payload, storedPayload)

	require.NoError(t, storage.Close())
	_, err = storage.NewReader()
	require.ErrorIs(t, err, ErrStorageClosed)
}

func TestMemoryBodyStorageNewReaderUsesIndependentCursors(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"memory payload"}`)
	storage := newMemoryStorage(payload)
	t.Cleanup(func() { _ = storage.Close() })

	assertBodyStorageReadersAreIndependent(t, storage, payload)
}

func TestDiskBodyStorageNewReaderUsesIndependentCursors(t *testing.T) {
	originalConfig := GetDiskCacheConfig()
	SetDiskCacheConfig(DiskCacheConfig{Path: t.TempDir()})
	t.Cleanup(func() { SetDiskCacheConfig(originalConfig) })

	payload := []byte(`{"model":"test-model","input":"disk payload"}`)
	storage, err := newDiskStorage(payload, GetDiskCachePath())
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })

	assertBodyStorageReadersAreIndependent(t, storage, payload)
}

func TestNewReplayableBodyReaderKeepsStorageLifecycleWithCaller(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"hello"}`)
	storage, err := CreateBodyStorage(payload)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })

	body := NewReplayableBodyReader(storage)
	assert.EqualValues(t, len(payload), body.Size())
	_, exposesCloser := any(body).(io.Closer)
	assert.False(t, exposesCloser, "the request body must not expose the storage closer")

	req, err := http.NewRequest(http.MethodPost, "https://example.com", body)
	require.NoError(t, err)
	require.NoError(t, req.Body.Close())

	replayBody, err := body.NewReader()
	require.NoError(t, err, "closing the HTTP request body must not close the storage")
	replay, err := io.ReadAll(replayBody)
	require.NoError(t, err)
	require.NoError(t, replayBody.Close())
	assert.Equal(t, payload, replay)

	require.NoError(t, storage.Close())
	_, err = body.NewReader()
	require.ErrorIs(t, err, ErrStorageClosed)
}
