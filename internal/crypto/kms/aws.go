package kms

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
)

type AWSConfig struct {
	Region string
	KeyID  string
}

type awsKMSAPI interface {
	Encrypt(context.Context, *awskms.EncryptInput, ...func(*awskms.Options)) (*awskms.EncryptOutput, error)
	Decrypt(context.Context, *awskms.DecryptInput, ...func(*awskms.Options)) (*awskms.DecryptOutput, error)
	DescribeKey(context.Context, *awskms.DescribeKeyInput, ...func(*awskms.Options)) (*awskms.DescribeKeyOutput, error)
	EnableKeyRotation(
		context.Context,
		*awskms.EnableKeyRotationInput,
		...func(*awskms.Options),
	) (*awskms.EnableKeyRotationOutput, error)
}

// AWSProvider uses AWS KMS symmetric customer-managed keys.
type AWSProvider struct {
	client awsKMSAPI
	keyID  string
}

var _ Provider = (*AWSProvider)(nil)

func NewAWSProvider(ctx context.Context, config AWSConfig) (*AWSProvider, error) {
	if strings.TrimSpace(config.Region) == "" || strings.TrimSpace(config.KeyID) == "" {
		return nil, errors.New("AWS KMS region and key ID are required")
	}
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.Region))
	if err != nil {
		return nil, fmt.Errorf("load AWS credentials and config: %w", err)
	}
	return NewAWSProviderWithClient(awskms.NewFromConfig(awsConfig), config.KeyID)
}

func NewAWSProviderWithClient(client awsKMSAPI, keyID string) (*AWSProvider, error) {
	if client == nil {
		return nil, errors.New("AWS KMS client is nil")
	}
	if strings.TrimSpace(keyID) == "" {
		return nil, errors.New("AWS KMS key ID is empty")
	}
	return &AWSProvider{client: client, keyID: keyID}, nil
}

func (p *AWSProvider) WrapKey(ctx context.Context, dek []byte) ([]byte, string, error) {
	if err := validateContext(ctx); err != nil {
		return nil, "", err
	}
	if len(dek) != 32 {
		return nil, "", fmt.Errorf("DEK must be 32 bytes, got %d", len(dek))
	}
	plaintext := append([]byte(nil), dek...)
	defer wipe(plaintext)
	out, err := p.client.Encrypt(ctx, &awskms.EncryptInput{
		KeyId:     aws.String(p.keyID),
		Plaintext: plaintext,
	})
	if err != nil {
		return nil, "", fmt.Errorf("AWS KMS Encrypt: %w", err)
	}
	if out == nil || len(out.CiphertextBlob) == 0 {
		return nil, "", errors.New("AWS KMS returned an empty ciphertext")
	}
	keyID := p.keyID
	if out.KeyId != nil && *out.KeyId != "" {
		keyID = *out.KeyId
	}
	return append([]byte(nil), out.CiphertextBlob...), keyID, nil
}

func (p *AWSProvider) UnwrapKey(ctx context.Context, wrapped []byte, keyID string) ([]byte, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	if len(wrapped) == 0 || strings.TrimSpace(keyID) == "" {
		return nil, errors.New("wrapped DEK and AWS KMS key ID are required")
	}
	out, err := p.client.Decrypt(ctx, &awskms.DecryptInput{
		CiphertextBlob: append([]byte(nil), wrapped...),
		KeyId:          aws.String(keyID),
	})
	if err != nil {
		return nil, fmt.Errorf("AWS KMS Decrypt: %w", err)
	}
	if out == nil || len(out.Plaintext) != 32 {
		if out != nil {
			wipe(out.Plaintext)
		}
		return nil, errors.New("AWS KMS returned a DEK with invalid size")
	}
	dek := append([]byte(nil), out.Plaintext...)
	wipe(out.Plaintext)
	return dek, nil
}

func (p *AWSProvider) GetKeyID() string { return p.keyID }

// RotateKey enables AWS-managed automatic rotation. AWS controls the rotation
// schedule; old ciphertext remains decryptable using the same key ARN.
func (p *AWSProvider) RotateKey(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	_, err := p.client.EnableKeyRotation(ctx, &awskms.EnableKeyRotationInput{KeyId: aws.String(p.keyID)})
	if err != nil {
		return fmt.Errorf("enable AWS KMS automatic key rotation: %w", err)
	}
	return nil
}

func (p *AWSProvider) HealthCheck(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	out, err := p.client.DescribeKey(ctx, &awskms.DescribeKeyInput{KeyId: aws.String(p.keyID)})
	if err != nil {
		return fmt.Errorf("AWS KMS health check: %w", err)
	}
	if out == nil || out.KeyMetadata == nil {
		return errors.New("AWS KMS health check returned no key metadata")
	}
	return nil
}
