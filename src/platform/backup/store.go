package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Object adalah satu file backup di penyimpanan.
type Object struct {
	Key     string
	Size    int64
	Created time.Time
}

// Store: penyimpanan PRIVAT untuk file backup (bukan bucket foto publik).
type Store interface {
	Put(ctx context.Context, key string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	List(ctx context.Context) ([]Object, error) // terbaru dulu
	Delete(ctx context.Context, key string) error
}

func validKey(key string) bool {
	return key != "" && !strings.ContainsAny(key, `/\`) && !strings.HasPrefix(key, ".")
}

// ---------- R2 (S3-compatible) ----------

// R2Store menyimpan backup di bucket R2 terpisah tanpa akses publik.
type R2Store struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewR2Store(endpoint, bucket, accessKey, secretKey string) *R2Store {
	client := s3.New(s3.Options{
		Region: "auto", BaseEndpoint: aws.String(endpoint),
		Credentials:                credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &R2Store{client: client, bucket: bucket, prefix: "db/"}
}

func (s *R2Store) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	if !validKey(key) {
		return fmt.Errorf("backup: key tidak valid %q", key)
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.prefix + key), Body: body,
		ContentLength: aws.Int64(size), ContentType: aws.String("application/octet-stream"),
	})
	return err
}

func (s *R2Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if !validKey(key) {
		return nil, fmt.Errorf("backup: key tidak valid %q", key)
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.prefix + key)})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *R2Store) List(ctx context.Context) ([]Object, error) {
	var out []Object
	p := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(s.prefix)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range page.Contents {
			out = append(out, Object{Key: strings.TrimPrefix(aws.ToString(o.Key), s.prefix), Size: aws.ToInt64(o.Size), Created: aws.ToTime(o.LastModified)})
		}
	}
	sortNewest(out)
	return out, nil
}

func (s *R2Store) Delete(ctx context.Context, key string) error {
	if !validKey(key) {
		return fmt.Errorf("backup: key tidak valid %q", key)
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.prefix + key)})
	return err
}

// ---------- Folder lokal (development / test) ----------

type DirStore struct{ Dir string }

func (d DirStore) path(key string) (string, error) {
	if !validKey(key) {
		return "", fmt.Errorf("backup: key tidak valid %q", key)
	}
	return filepath.Join(d.Dir, key), nil
}

func (d DirStore) Put(_ context.Context, key string, body io.Reader, _ int64) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d.Dir, 0o750); err != nil {
		return err
	}
	f, err := os.Create(p) //nolint:gosec // G304: key divalidasi
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (d DirStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := d.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p) //nolint:gosec // G304: key divalidasi
}

func (d DirStore) List(context.Context) ([]Object, error) {
	entries, err := os.ReadDir(d.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Object
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() {
			out = append(out, Object{Key: e.Name(), Size: info.Size(), Created: info.ModTime()})
		}
	}
	sortNewest(out)
	return out, nil
}

func (d DirStore) Delete(_ context.Context, key string) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func sortNewest(os []Object) {
	sort.Slice(os, func(i, j int) bool { return os[i].Created.After(os[j].Created) })
}
