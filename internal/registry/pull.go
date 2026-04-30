package registry

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (p *S3RegistryPlugin) Pull(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if key == "" {
		return nil, "", fmt.Errorf("registry: key is required")
	}

	out, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("registry: get object %q: %w", key, err)
	}

	sha256Meta := ""
	if out.Metadata != nil {
		sha256Meta = out.Metadata[sha256MetadataKey]
	}

	return out.Body, sha256Meta, nil
}

func (p *S3RegistryPlugin) Verify(ctx context.Context, key, expectedSHA256 string) error {
	if key == "" {
		return fmt.Errorf("registry: key is required")
	}

	out, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("registry: head object %q: %w", key, err)
	}

	sha256Meta := ""
	if out.Metadata != nil {
		sha256Meta = out.Metadata[sha256MetadataKey]
	}

	if sha256Meta != expectedSHA256 {
		return fmt.Errorf("registry: sha256 mismatch: stored=%s, expected=%s", sha256Meta, expectedSHA256)
	}

	return nil
}
