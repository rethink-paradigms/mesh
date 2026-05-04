package registry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type mockS3Client struct {
	objects          map[string]*mockObject
	multipartUploads map[string]*mockMultipartUpload
}

type mockObject struct {
	body     []byte
	metadata map[string]string
}

type mockMultipartUpload struct {
	key     string
	parts   map[int32][]byte
	partNum int32
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{
		objects:          make(map[string]*mockObject),
		multipartUploads: make(map[string]*mockMultipartUpload),
	}
}

func (m *mockS3Client) CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	uploadID := "upload-" + aws.ToString(params.Key)
	m.multipartUploads[uploadID] = &mockMultipartUpload{
		key:   aws.ToString(params.Key),
		parts: make(map[int32][]byte),
	}
	return &s3.CreateMultipartUploadOutput{UploadId: aws.String(uploadID)}, nil
}

func (m *mockS3Client) UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	upload, ok := m.multipartUploads[aws.ToString(params.UploadId)]
	if !ok {
		return nil, fmt.Errorf("upload not found")
	}
	buf, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}
	upload.parts[aws.ToInt32(params.PartNumber)] = buf
	return &s3.UploadPartOutput{ETag: aws.String(fmt.Sprintf("etag-%d", aws.ToInt32(params.PartNumber)))}, nil
}

func (m *mockS3Client) CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	upload, ok := m.multipartUploads[aws.ToString(params.UploadId)]
	if !ok {
		return nil, fmt.Errorf("upload not found")
	}

	var body []byte
	for i := int32(1); i <= int32(len(upload.parts)); i++ {
		body = append(body, upload.parts[i]...)
	}

	m.objects[upload.key] = &mockObject{body: body}
	delete(m.multipartUploads, aws.ToString(params.UploadId))
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (m *mockS3Client) AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	delete(m.multipartUploads, aws.ToString(params.UploadId))
	return &s3.AbortMultipartUploadOutput{}, nil
}

func (m *mockS3Client) CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	obj, ok := m.objects[aws.ToString(params.Key)]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}
	obj.metadata = params.Metadata
	return &s3.CopyObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	obj, ok := m.objects[aws.ToString(params.Key)]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}
	return &s3.GetObjectOutput{
		Body:     io.NopCloser(bytes.NewReader(obj.body)),
		Metadata: obj.metadata,
	}, nil
}

func (m *mockS3Client) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	return &s3.HeadBucketOutput{}, nil
}

type s3ClientInterface interface {
	CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

type testPlugin struct {
	*S3RegistryPlugin
	mock *mockS3Client
}

func newTestPlugin() *testPlugin {
	mock := newMockS3Client()
	return &testPlugin{
		S3RegistryPlugin: &S3RegistryPlugin{
			cfg:    RegistryConfig{Bucket: "test-bucket", Region: "us-east-1"},
			client: (*s3.Client)(nil),
		},
		mock: mock,
	}
}

func (tp *testPlugin) Push(ctx context.Context, key string, r io.Reader, size int64, sha256 string) error {
	if key == "" {
		return fmt.Errorf("registry: key is required")
	}

	createOut, err := tp.mock.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(tp.cfg.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String("application/octet-stream"),
		Metadata:    map[string]string{sha256MetadataKey: sha256},
	})
	if err != nil {
		return fmt.Errorf("registry: create multipart upload: %w", err)
	}

	uploadID := aws.ToString(createOut.UploadId)
	var completedParts []types.CompletedPart
	partNum := int32(1)

	const partSize = 5 * 1024 * 1024
	buf := make([]byte, partSize)

	for {
		n, err := io.ReadFull(r, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			_, _ = tp.mock.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(tp.cfg.Bucket),
				Key:      aws.String(key),
				UploadId: aws.String(uploadID),
			})
			return fmt.Errorf("registry: read stream: %w", err)
		}
		if n == 0 {
			break
		}

		uploadOut, uploadErr := tp.mock.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(tp.cfg.Bucket),
			Key:        aws.String(key),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(partNum),
			Body:       bytes.NewReader(buf[:n]),
		})
		if uploadErr != nil {
			_, _ = tp.mock.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(tp.cfg.Bucket),
				Key:      aws.String(key),
				UploadId: aws.String(uploadID),
			})
			return fmt.Errorf("registry: upload part %d: %w", partNum, uploadErr)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       uploadOut.ETag,
			PartNumber: aws.Int32(partNum),
		})
		partNum++

		if err == io.EOF {
			break
		}
	}

	_, err = tp.mock.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(tp.cfg.Bucket),
		Key:    aws.String(key),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return fmt.Errorf("registry: complete multipart upload: %w", err)
	}

	_, err = tp.mock.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(tp.cfg.Bucket),
		Key:        aws.String(key),
		CopySource: aws.String(tp.cfg.Bucket + "/" + key),
		Metadata: map[string]string{
			sha256MetadataKey: sha256,
		},
		MetadataDirective: types.MetadataDirectiveReplace,
	})
	if err != nil {
		return fmt.Errorf("registry: set sha256 metadata: %w", err)
	}

	return nil
}

func (tp *testPlugin) Pull(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if key == "" {
		return nil, "", fmt.Errorf("registry: key is required")
	}

	out, err := tp.mock.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(tp.cfg.Bucket),
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

func (tp *testPlugin) Verify(ctx context.Context, key, expectedSHA256 string) error {
	out, err := tp.mock.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(tp.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("registry: get object %q: %w", key, err)
	}
	defer out.Body.Close()

	sha256Meta := ""
	if out.Metadata != nil {
		sha256Meta = out.Metadata[sha256MetadataKey]
	}

	if sha256Meta != expectedSHA256 {
		return fmt.Errorf("registry: sha256 mismatch: stored=%s, expected=%s", sha256Meta, expectedSHA256)
	}

	return nil
}

func TestPushSucceeds(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	content := []byte("hello world")
	err := p.Push(ctx, "test-key", bytes.NewReader(content), int64(len(content)), "abc123")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}
}

func TestPushAndPullContent(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	content := []byte("hello world this is a test snapshot")
	key := "snapshot-123"
	sha256 := "dummy-sha256-abc123"

	err := p.Push(ctx, key, bytes.NewReader(content), int64(len(content)), sha256)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	body, pulledSHA, err := p.Pull(ctx, key)
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
	defer body.Close()

	pulled, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Read pulled content failed: %v", err)
	}

	if !bytes.Equal(pulled, content) {
		t.Fatalf("pulled content mismatch: got %q, want %q", pulled, content)
	}
	if pulledSHA != sha256 {
		t.Fatalf("pulled sha256 = %q, want %q", pulledSHA, sha256)
	}
}

func TestVerifyPassesOnMatchingHash(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	content := []byte("verify me")
	key := "verify-test"
	sha256 := "expected-sha256-xyz"

	err := p.Push(ctx, key, bytes.NewReader(content), int64(len(content)), sha256)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if err := p.Verify(ctx, key, sha256); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
}

func TestVerifyFailsOnMismatch(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	content := []byte("original content")
	key := "tampered-test"
	sha256 := "expected-sha256-abc"

	err := p.Push(ctx, key, bytes.NewReader(content), int64(len(content)), sha256)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	err = p.Verify(ctx, key, "wrong-sha256")
	if err == nil {
		t.Fatal("expected Verify to fail on mismatched sha256, got nil")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch error, got: %v", err)
	}
}

func TestPushEmptyKey(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	err := p.Push(ctx, "", bytes.NewReader([]byte("data")), 4, "sha")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestPullEmptyKey(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	_, _, err := p.Pull(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestPullMissingObject(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	_, _, err := p.Pull(ctx, "nonexistent-key")
	if err == nil {
		t.Fatal("expected error for missing object")
	}
}

func TestVerifyMissingObject(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	err := p.Verify(ctx, "nonexistent-key", "sha256")
	if err == nil {
		t.Fatal("expected error for missing object")
	}
}

func TestPushLargeContentMultipart(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	content := make([]byte, 6*1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	key := "large-snapshot"
	sha256 := "large-sha256-xyz"

	err := p.Push(ctx, key, bytes.NewReader(content), int64(len(content)), sha256)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	body, pulledSHA, err := p.Pull(ctx, key)
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
	defer body.Close()

	pulled, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Read pulled content failed: %v", err)
	}

	if !bytes.Equal(pulled, content) {
		t.Fatalf("large content mismatch: length got=%d, want=%d", len(pulled), len(content))
	}
	if pulledSHA != sha256 {
		t.Fatalf("pulled sha256 = %q, want %q", pulledSHA, sha256)
	}

	if err := p.Verify(ctx, key, sha256); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
}

func TestPluginInfo(t *testing.T) {
	ctx := context.Background()
	p := newTestPlugin()

	meta, err := p.PluginInfo(ctx)
	if err != nil {
		t.Fatalf("PluginInfo failed: %v", err)
	}

	if meta.Name != "s3-registry" {
		t.Fatalf("expected name s3-registry, got %s", meta.Name)
	}
	if meta.Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %s", meta.Version)
	}
}

func TestNewS3RegistryPluginMissingBucket(t *testing.T) {
	_, err := NewS3RegistryPlugin(RegistryConfig{Region: "us-east-1"})
	if err == nil {
		t.Fatal("expected error for missing bucket")
	}
}
