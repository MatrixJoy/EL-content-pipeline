package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/oldj/english-learning/content-pipeline/internal/domain"
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

func (s *Objects) Get(ctx context.Context, key string) ([]byte, string, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("get %s: %w", key, err)
	}
	defer object.Close()
	stat, err := object.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat %s: %w", key, err)
	}
	data, err := io.ReadAll(object)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", key, err)
	}
	return data, stat.ContentType, nil
}

func (s *Objects) Put(ctx context.Context, key, mediaType string, data []byte) (domain.Object, error) {
	sum := sha256.Sum256(data)
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: mediaType})
	if err != nil {
		return domain.Object{}, fmt.Errorf("put %s: %w", key, err)
	}
	return domain.Object{Key: key, SHA256: hex.EncodeToString(sum[:]), ByteSize: int64(len(data)), MediaType: mediaType}, nil
}

func (s *Objects) ListKeys(ctx context.Context, prefix, suffix string) ([]string, error) {
	keys := []string{}
	for object := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		if suffix == "" || strings.HasSuffix(object.Key, suffix) {
			keys = append(keys, object.Key)
		}
	}
	return keys, nil
}
