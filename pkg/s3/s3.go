package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
	Endpoint       string        `mapstructure:"endpoint"`
	AccessKey      string        `mapstructure:"access_key"`
	SecretKey      string        `mapstructure:"secret_key"`
	Bucket         string        `mapstructure:"bucket"`
	Region         string        `mapstructure:"region"`
	PublicURL      string        `mapstructure:"public_url"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
}

func DefaultConfig() S3Config {
	return S3Config{
		Endpoint:       "http://seaweedfs:8333",
		Region:         "us-east-1",
		Bucket:         "media",
		RequestTimeout: 10 * time.Second,
	}
}

func (c S3Config) Validate() error {
	var errs []error
	if _, _, err := parseEndpoint(c.Endpoint); err != nil {
		errs = append(errs, fmt.Errorf("s3: %w", err))
	}
	if c.AccessKey == "" {
		errs = append(errs, errors.New("s3: access_key must not be empty"))
	}
	if c.SecretKey == "" {
		errs = append(errs, errors.New("s3: secret_key must not be empty"))
	}
	if c.Bucket == "" {
		errs = append(errs, errors.New("s3: bucket must not be empty"))
	}
	if u, err := url.Parse(c.PublicURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, fmt.Errorf("s3: public_url must be an absolute http(s) url, got %q", c.PublicURL))
	}
	if c.RequestTimeout <= 0 {
		errs = append(errs, fmt.Errorf("s3: request_timeout must be positive, got %s", c.RequestTimeout))
	}
	return errors.Join(errs...)
}

// схема в endpoint обязательна, по ней определяется, нужен ли TLS
func parseEndpoint(raw string) (host string, secure bool, err error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", false, fmt.Errorf("endpoint must be an http(s) url without path, got %q", raw)
	}
	return u.Host, u.Scheme == "https", nil
}

type Client struct {
	client         *minio.Client
	bucket         string
	requestTimeout time.Duration
}

func NewClient(sCfg S3Config) (*Client, error) {
	host, secure, err := parseEndpoint(sCfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("s3 new client error: %w", err)
	}

	cli, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(sCfg.AccessKey, sCfg.SecretKey, ""),
		Secure: secure,
		Region: sCfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 new client error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), sCfg.RequestTimeout)
	defer cancel()

	exists, err := cli.BucketExists(ctx, sCfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("s3 bucket check error: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("s3 bucket %q does not exist", sCfg.Bucket)
	}

	return &Client{client: cli, bucket: sCfg.Bucket, requestTimeout: sCfg.RequestTimeout}, nil
}

func (c *Client) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	_, err := c.client.PutObject(ctx, c.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("s3 put object %q error: %w", key, err)
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	if err := c.client.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 remove object %q error: %w", key, err)
	}
	return nil
}
