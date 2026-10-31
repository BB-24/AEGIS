package kms

import (
	"context"
	"errors"
	"testing"
)

type memoryRotationStore struct {
	records map[string]WrappedDEKRecord
}

func (s *memoryRotationStore) ForEachWrappedDEK(ctx context.Context, visit func(WrappedDEKRecord) error) error {
	for _, record := range s.records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := visit(record); err != nil {
			return err
		}
	}
	return nil
}

func (s *memoryRotationStore) UpdateWrappedDEK(_ context.Context, id string, wrapped []byte, keyID string) error {
	record, ok := s.records[id]
	if !ok {
		return errors.New("record not found")
	}
	record.Wrapped = append([]byte(nil), wrapped...)
	record.KeyID = keyID
	s.records[id] = record
	return nil
}

func TestRotateAndRewrapUpdatesStoredKeys(t *testing.T) {
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	wrapped, keyID, err := provider.WrapKey(context.Background(), []byte("data key"))
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryRotationStore{records: map[string]WrappedDEKRecord{
		"chunk-1": {ID: "chunk-1", Wrapped: wrapped, KeyID: keyID},
	}}
	if err := RotateAndRewrap(context.Background(), provider, store); err != nil {
		t.Fatal(err)
	}
	record := store.records["chunk-1"]
	if record.KeyID != provider.GetKeyID() || record.KeyID == keyID {
		t.Fatalf("record key ID = %q; active=%q old=%q", record.KeyID, provider.GetKeyID(), keyID)
	}
	decoded, err := provider.UnwrapKey(context.Background(), record.Wrapped, record.KeyID)
	if err != nil || string(decoded) != "data key" {
		t.Fatalf("rewrapped DEK = (%q, %v)", decoded, err)
	}
}

func TestRotateAndRewrapValidatesAndPropagatesErrors(t *testing.T) {
	if err := RotateAndRewrap(context.Background(), nil, &memoryRotationStore{}); err == nil {
		t.Fatal("accepted nil provider")
	}
	provider, err := NewLocalProvider()
	if err != nil {
		t.Fatal(err)
	}
	if err := RotateAndRewrap(context.Background(), provider, nil); err == nil {
		t.Fatal("accepted nil store")
	}
	store := &memoryRotationStore{records: map[string]WrappedDEKRecord{
		"invalid": {ID: "invalid"},
	}}
	if err := RotateAndRewrap(context.Background(), provider, store); err == nil {
		t.Fatal("ignored incomplete rotation record")
	}
}
