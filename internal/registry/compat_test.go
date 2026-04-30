package registry

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// TestAWSCompat validates that AWS SDK v2 config and S3 packages import
// and compile cleanly with the project's Go version.
func TestAWSCompat(t *testing.T) {
	// Load default config (no credentials required for compilation test).
	cfg, err := config.LoadDefaultConfig(t.Context())
	if err != nil {
		t.Skipf("AWS config load skipped (no credentials): %v", err)
	}

	// Create S3 client to verify the package links correctly.
	client := s3.NewFromConfig(cfg)
	if client == nil {
		t.Fatal("expected non-nil S3 client")
	}
}
