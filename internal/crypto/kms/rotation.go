package kms

import (
	"context"
	"errors"
	"fmt"
)

type WrappedDEKRecord struct {
	ID      string
	Wrapped []byte
	KeyID   string
}

// RotationStore enumerates wrapped keys and atomically updates their metadata.
type RotationStore interface {
	ForEachWrappedDEK(ctx context.Context, visit func(WrappedDEKRecord) error) error
	UpdateWrappedDEK(ctx context.Context, id string, wrapped []byte, keyID string) error
}

// RotateAndRewrap rotates the provider KEK, then rewrites all stored DEK
// envelopes. The store should make each metadata update durable and atomic.
func RotateAndRewrap(ctx context.Context, provider Provider, store RotationStore) error {
	if provider == nil || store == nil {
		return errors.New("KMS provider and rotation store are required")
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := provider.RotateKey(ctx); err != nil {
		return fmt.Errorf("rotate KMS key: %w", err)
	}
	err := store.ForEachWrappedDEK(ctx, func(record WrappedDEKRecord) error {
		if record.ID == "" || len(record.Wrapped) == 0 || record.KeyID == "" {
			return errors.New("rotation store returned an incomplete wrapped DEK record")
		}
		dek, err := provider.UnwrapKey(ctx, record.Wrapped, record.KeyID)
		if err != nil {
			return fmt.Errorf("unwrap DEK %q: %w", record.ID, err)
		}
		defer wipe(dek)
		wrapped, keyID, err := provider.WrapKey(ctx, dek)
		if err != nil {
			wipe(wrapped)
			return fmt.Errorf("rewrap DEK %q: %w", record.ID, err)
		}
		defer wipe(wrapped)
		if err := store.UpdateWrappedDEK(ctx, record.ID, wrapped, keyID); err != nil {
			return fmt.Errorf("update DEK %q metadata: %w", record.ID, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("rewrap stored DEKs: %w", err)
	}
	return nil
}
