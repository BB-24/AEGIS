package kms

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

const awsWrappedKey = "wrapped"

type fakeAWSKMS struct {
	plaintext  []byte
	encryptErr error
	decryptErr error
	rotateErr  error
	healthErr  error
	rotated    bool
}

func (f *fakeAWSKMS) Encrypt(_ context.Context, in *awskms.EncryptInput, _ ...func(*awskms.Options)) (*awskms.EncryptOutput, error) {
	if f.encryptErr != nil {
		return nil, f.encryptErr
	}
	f.plaintext = append([]byte(nil), in.Plaintext...)
	return &awskms.EncryptOutput{CiphertextBlob: []byte(awsWrappedKey), KeyId: aws.String("arn:aws:kms:region:account:key/version")}, nil
}

func (f *fakeAWSKMS) Decrypt(_ context.Context, _ *awskms.DecryptInput, _ ...func(*awskms.Options)) (*awskms.DecryptOutput, error) {
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}
	return &awskms.DecryptOutput{Plaintext: append([]byte(nil), f.plaintext...)}, nil
}

func (f *fakeAWSKMS) DescribeKey(context.Context, *awskms.DescribeKeyInput, ...func(*awskms.Options)) (*awskms.DescribeKeyOutput, error) {
	if f.healthErr != nil {
		return nil, f.healthErr
	}
	return &awskms.DescribeKeyOutput{KeyMetadata: &types.KeyMetadata{}}, nil
}

func (f *fakeAWSKMS) EnableKeyRotation(context.Context, *awskms.EnableKeyRotationInput, ...func(*awskms.Options)) (*awskms.EnableKeyRotationOutput, error) {
	f.rotated = true
	return &awskms.EnableKeyRotationOutput{}, f.rotateErr
}

func TestAWSProviderWrapUnwrapAndHealth(t *testing.T) {
	client := &fakeAWSKMS{}
	provider, err := NewAWSProviderWithClient(client, "alias/aegis")
	if err != nil {
		t.Fatal(err)
	}
	dek := make([]byte, 32)
	wrapped, keyID, err := provider.WrapKey(context.Background(), dek)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wrapped, []byte(awsWrappedKey)) || keyID != "arn:aws:kms:region:account:key/version" {
		t.Fatalf("wrap result = (%q, %q)", wrapped, keyID)
	}
	decoded, err := provider.UnwrapKey(context.Background(), wrapped, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, dek) {
		t.Fatal("AWS provider roundtrip changed the DEK")
	}
	if err := provider.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := provider.RotateKey(context.Background()); err != nil || !client.rotated {
		t.Fatalf("RotateKey = %v, rotated=%v", err, client.rotated)
	}
}

func TestAWSProviderValidationAndErrors(t *testing.T) {
	if _, err := NewAWSProviderWithClient(nil, "key"); err == nil {
		t.Fatal("accepted nil client")
	}
	if _, err := NewAWSProviderWithClient(&fakeAWSKMS{}, ""); err == nil {
		t.Fatal("accepted empty key ID")
	}
	client := &fakeAWSKMS{encryptErr: errors.New("offline")}
	provider, err := NewAWSProviderWithClient(client, "key")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := provider.WrapKey(context.Background(), make([]byte, 32)); err == nil {
		t.Fatal("ignored AWS Encrypt error")
	}
	if _, _, err := provider.WrapKey(context.Background(), []byte("short")); err == nil {
		t.Fatal("accepted short DEK")
	}
	client.encryptErr = nil
	client.decryptErr = errors.New("offline")
	if _, err := provider.UnwrapKey(context.Background(), []byte("wrapped"), "key"); err == nil {
		t.Fatal("ignored AWS Decrypt error")
	}
	client.decryptErr = nil
	client.rotateErr = errors.New("denied")
	if err := provider.RotateKey(context.Background()); err == nil {
		t.Fatal("ignored AWS rotation error")
	}
	client.healthErr = errors.New("offline")
	if err := provider.HealthCheck(context.Background()); err == nil {
		t.Fatal("ignored AWS health error")
	}
}
