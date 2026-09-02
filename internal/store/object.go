package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/oldj/voa-content-pipeline/internal/domain"
)

type Objects struct {
	client *minio.Client
	bucket string
}

func NewObjects(ctx context.Context, endpoint, key, secret, bucket string, tls bool) (*Objects, error) {
	c, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(key, secret, ""), Secure: tls})
	if err != nil {
		return nil, err
	}
	exists, err := c.BucketExists(ctx, bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err = c.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	return &Objects{client: c, bucket: bucket}, nil
}
func (s *Objects) Put(ctx context.Context, key, mediaType string, data []byte) (domain.Object, error) {
	sum := sha256.Sum256(data)
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: mediaType})
	if err != nil {
		return domain.Object{}, fmt.Errorf("put %s: %w", key, err)
	}
	return domain.Object{Key: key, SHA256: hex.EncodeToString(sum[:]), ByteSize: int64(len(data)), MediaType: mediaType}, nil
}
