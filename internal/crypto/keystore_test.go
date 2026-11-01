package crypto

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/aegis/aegis/internal/crypto/kms"
)

const mockKeyID = "mock-key"

type mockKMS struct {
	wrapped []byte
	keyID   string
	plain   []byte
	err     error
}

func (m *mockKMS) WrapKey(_ context.Context, plaintext []byte) ([]byte, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	m.plain = append(m.plain[:0], plaintext...)
	m.wrapped = append([]byte("wrapped:"), plaintext...)
	m.keyID = mockKeyID
	return append([]byte(nil), m.wrapped...), m.keyID, nil
}

func (m *mockKMS) UnwrapKey(_ context.Context, ciphertext []byte, keyID string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	if keyID != m.keyID || len(ciphertext) < len("wrapped:") {
		return nil, errors.New("invalid wrapped key")
	}
	return append([]byte(nil), ciphertext[len("wrapped:"):]...), nil
}

func (*mockKMS) GetKeyID() string { return mockKeyID }

func (*mockKMS) RotateKey(context.Context) error { return kms.ErrRotationUnsupported }

func (*mockKMS) HealthCheck(context.Context) error { return nil }

func TestProviderKeyManagerWrapUnwrapAndRotate(t *testing.T) {
	provider := &mockKMS{}
	manager, err := NewKeyManager(provider)
	if err != nil {
		t.Fatal(err)
	}
	dek := make([]byte, DEKSize)
	for i := range dek {
		dek[i] = byte(i)
	}
	wrapped, keyID, err := manager.WrapKey(context.Background(), dek)
	if err != nil {
		t.Fatal(err)
	}
	if keyID != mockKeyID {
		t.Fatalf("key ID = %q", keyID)
	}
	unwrapped, err := manager.UnwrapKey(context.Background(), wrapped, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unwrapped, dek) {
		t.Fatal("unwrapped key differs from original")
	}
	if err := manager.RotateKEK(context.Background()); !errors.Is(err, ErrRotationUnsupported) {
		t.Fatalf("RotateKEK error = %v, want ErrRotationUnsupported", err)
	}
}

func TestProviderKeyManagerValidationAndErrors(t *testing.T) {
	if _, err := NewKeyManager(nil); err == nil {
		t.Fatal("accepted nil KMS provider")
	}
	manager, err := NewKeyManager(&mockKMS{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.WrapKey(context.Background(), []byte("short")); err == nil {
		t.Fatal("wrapped an invalid DEK size")
	}
	if _, err := manager.UnwrapKey(context.Background(), nil, "key"); err == nil {
		t.Fatal("unwrapped empty ciphertext")
	}
	if _, err := manager.UnwrapKey(context.Background(), []byte("wrapped"), ""); err == nil {
		t.Fatal("unwrapped with empty key ID")
	}

	provider := &mockKMS{err: errors.New("KMS unavailable")}
	manager, err = NewKeyManager(provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.WrapKey(context.Background(), make([]byte, DEKSize)); err == nil {
		t.Fatal("ignored KMS wrap failure")
	}
	if _, err := manager.UnwrapKey(context.Background(), []byte("wrapped"), "key"); err == nil {
		t.Fatal("ignored KMS unwrap failure")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := manager.WrapKey(ctx, make([]byte, DEKSize)); !errors.Is(err, context.Canceled) {
		t.Fatalf("WrapKey error = %v, want context.Canceled", err)
	}
}

func TestProviderKeyManagerRejectsInvalidProviderOutputs(t *testing.T) {
	empty := &emptyOutputKMS{}
	manager, err := NewKeyManager(empty)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.WrapKey(context.Background(), make([]byte, DEKSize)); err == nil {
		t.Fatal("accepted empty wrapped key and key ID")
	}
	if _, err := manager.UnwrapKey(context.Background(), []byte("wrapped"), "key"); err == nil {
		t.Fatal("accepted invalid unwrapped DEK size")
	}
}

func TestProviderKeyManagerRotatorErrors(t *testing.T) {
	provider := &rotatingMockKMS{mockKMS: mockKMS{}, err: errors.New("rotation failed")}
	manager, err := NewKeyManager(provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RotateKEK(context.Background()); err == nil {
		t.Fatal("ignored rotation failure")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := manager.RotateKEK(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("RotateKEK error = %v, want context.Canceled", err)
	}
}

type rotatingMockKMS struct {
	mockKMS
	err error
}

func (m *rotatingMockKMS) RotateKey(context.Context) error { return m.err }

type emptyOutputKMS struct{}

func (*emptyOutputKMS) WrapKey(context.Context, []byte) ([]byte, string, error) {
	return nil, "", nil
}

func (*emptyOutputKMS) UnwrapKey(context.Context, []byte, string) ([]byte, error) {
	return []byte("wrong-sized-key"), nil
}

func (*emptyOutputKMS) GetKeyID() string { return "test" }

func (*emptyOutputKMS) RotateKey(context.Context) error { return nil }

func (*emptyOutputKMS) HealthCheck(context.Context) error { return nil }
