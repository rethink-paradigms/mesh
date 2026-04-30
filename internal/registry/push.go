package registry

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const sha256MetadataKey = "x-amz-meta-sha256"

func (p *S3RegistryPlugin) Push(ctx context.Context, key string, r io.Reader, size int64, sha256 string) error {
	if key == "" {
		return fmt.Errorf("registry: key is required")
	}

	createOut, err := p.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(p.cfg.Bucket),
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
			_, _ = p.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(p.cfg.Bucket),
				Key:      aws.String(key),
				UploadId: aws.String(uploadID),
			})
			return fmt.Errorf("registry: read stream: %w", err)
		}
		if n == 0 {
			break
		}

		uploadOut, uploadErr := p.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(p.cfg.Bucket),
			Key:        aws.String(key),
			UploadId:   aws.String(uploadID),
			PartNumber: aws.Int32(partNum),
			Body:       bytes.NewReader(buf[:n]),
		})
		if uploadErr != nil {
			_, _ = p.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(p.cfg.Bucket),
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

	_, err = p.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(p.cfg.Bucket),
		Key:    aws.String(key),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return fmt.Errorf("registry: complete multipart upload: %w", err)
	}

	_, err = p.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(p.cfg.Bucket),
		Key:        aws.String(key),
		CopySource: aws.String(p.cfg.Bucket + "/" + key),
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
