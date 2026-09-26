package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config menghubungkan ke Cloudflare R2 (atau layanan S3-compatible lain).
type S3Config struct {
	Endpoint        string // https://<account>.r2.cloudflarestorage.com
	Bucket          string
	PublicURL       string // basis URL publik, mis. https://media.lovoria.com
	AccessKeyID     string
	SecretAccessKey string
	// PathStyle: true untuk server S3 lokal/test; R2 memakai virtual-host (false).
	PathStyle bool
}

// S3 menyimpan objek di bucket S3-compatible (R2). Objek disajikan lewat
// PublicURL (custom domain / r2.dev di balik CDN), bukan presigned URL.
type S3 struct {
	client    *s3.Client
	bucket    string
	publicURL string
}

func NewS3(_ context.Context, c S3Config) (*S3, error) {
	if c.Endpoint == "" || c.Bucket == "" || c.PublicURL == "" {
		return nil, errors.New("storage: endpoint, bucket, dan public URL wajib diisi")
	}
	client := s3.New(s3.Options{
		Region:       "auto",
		BaseEndpoint: aws.String(c.Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(c.AccessKeyID, c.SecretAccessKey, ""),
		UsePathStyle: c.PathStyle,
		// Checksum hanya bila diwajibkan: kompatibel dengan R2 & server S3 lokal.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &S3{client: client, bucket: c.Bucket, publicURL: strings.TrimRight(c.PublicURL, "/")}, nil
}

func (s *S3) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if !validKey(key) {
		return fmt.Errorf("storage: key tidak valid %q", key)
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
		CacheControl:  aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return fmt.Errorf("storage: put %s: %w", key, err)
	}
	return nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if !validKey(key) {
		return fmt.Errorf("storage: key tidak valid %q", key)
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("storage: delete %s: %w", key, err)
	}
	return nil
}

func (s *S3) PublicURL(key string) string { return s.publicURL + "/" + key }
