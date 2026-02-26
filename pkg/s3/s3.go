// Package s3 provides S3 storage operations for snapshots.
package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Client wraps S3 operations for snapshot storage.
type Client struct {
	client     *s3.Client
	bucket     string
	prefix     string
	region     string
	endpoint   string // For S3-compatible services (MinIO, etc.)
	accessKey  string
	secretKey  string
}

// Config holds S3 configuration.
type Config struct {
	Bucket    string
	Prefix    string // Optional: path prefix in bucket
	Region    string
	Endpoint  string // Optional: for S3-compatible services
	AccessKey string
	SecretKey string
}

// NewClient creates a new S3 client for snapshot operations.
func NewClient(cfg Config) (*Client, error) {
	client := &Client{
		bucket:    cfg.Bucket,
		prefix:    strings.TrimPrefix(cfg.Prefix, "/"),
		region:    cfg.Region,
		endpoint:  cfg.Endpoint,
		accessKey: cfg.AccessKey,
		secretKey: cfg.SecretKey,
	}

	if err := client.init(); err != nil {
		return nil, err
	}

	return client, nil
}

// init initializes the S3 client.
func (c *Client) init() error {
	ctx := context.Background()

	// Build config options
	var opts []func(*config.LoadOptions) error

	// Set region
	if c.region != "" {
		opts = append(opts, config.WithRegion(c.region))
	}

	// Set credentials if provided
	if c.accessKey != "" && c.secretKey != "" {
		creds := credentials.NewStaticCredentialsProvider(c.accessKey, c.secretKey, "")
		opts = append(opts, config.WithCredentialsProvider(creds))
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	s3Opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = true // Required for MinIO and some S3-compatible services
		},
	}

	// Set custom endpoint if provided (for MinIO, etc.)
	if c.endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(c.endpoint)
		})
	}

	c.client = s3.NewFromConfig(awsCfg, s3Opts...)

	// Verify bucket exists
	if err := c.verifyBucket(ctx); err != nil {
		return err
	}

	return nil
}

// verifyBucket checks if the bucket exists and is accessible.
func (c *Client) verifyBucket(ctx context.Context) error {
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(c.bucket),
	})
	if err != nil {
		return fmt.Errorf("cannot access S3 bucket %s: %w", c.bucket, err)
	}
	return nil
}

// UploadSnapshot uploads a snapshot directory to S3.
func (c *Client) UploadSnapshot(snapshotName string, snapshotDir string) error {
	ctx := context.Background()

	// Walk through snapshot directory
	return filepath.Walk(snapshotDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate S3 key
		relPath, err := filepath.Rel(snapshotDir, path)
		if err != nil {
			return err
		}

		key := c.buildKey(snapshotName, relPath)

		// Upload file
		if err := c.uploadFile(ctx, path, key); err != nil {
			return fmt.Errorf("failed to upload %s: %w", relPath, err)
		}

		return nil
	})
}

// uploadFile uploads a single file to S3.
func (c *Client) uploadFile(ctx context.Context, localPath, key string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Determine content type
	contentType := getContentType(localPath)

	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})

	return err
}

// DownloadSnapshot downloads a snapshot from S3 to local directory.
func (c *Client) DownloadSnapshot(snapshotName string, destDir string) error {
	ctx := context.Background()

	prefix := c.buildKey(snapshotName, "")

	// List objects with prefix
	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			if err := c.downloadObject(ctx, *obj.Key, destDir, prefix); err != nil {
				return err
			}
		}
	}

	return nil
}

// downloadObject downloads a single S3 object.
func (c *Client) downloadObject(ctx context.Context, key, destDir, prefix string) error {
	// Calculate relative path
	relPath := strings.TrimPrefix(key, prefix)
	relPath = strings.TrimPrefix(relPath, "/")

	localPath := filepath.Join(destDir, relPath)

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	// Download object
	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", key, err)
	}
	defer resp.Body.Close()

	// Write to file
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// ListSnapshots lists all snapshots stored in S3.
func (c *Client) ListSnapshots() ([]string, error) {
	ctx := context.Background()

	prefix := c.prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Use a map to track unique snapshot names
	snapshots := make(map[string]bool)

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(c.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		// CommonPrefixes contains "subdirectories"
		for _, cp := range page.CommonPrefixes {
			snapshotName := strings.TrimSuffix(strings.TrimPrefix(*cp.Prefix, prefix), "/")
			if snapshotName != "" {
				snapshots[snapshotName] = true
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(snapshots))
	for name := range snapshots {
		result = append(result, name)
	}

	return result, nil
}

// DeleteSnapshot deletes a snapshot from S3.
func (c *Client) DeleteSnapshot(snapshotName string) error {
	ctx := context.Background()

	prefix := c.buildKey(snapshotName, "")

	// List all objects with prefix
	var objectsToDelete []types.ObjectIdentifier

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
	}

	if len(objectsToDelete) == 0 {
		return fmt.Errorf("snapshot %s not found", snapshotName)
	}

	// Delete objects in batches (max 1000 per request)
	const batchSize = 1000
	for i := 0; i < len(objectsToDelete); i += batchSize {
		end := i + batchSize
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}

		batch := objectsToDelete[i:end]
		_, err := c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(c.bucket),
			Delete: &types.Delete{
				Objects: batch,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to delete objects: %w", err)
		}
	}

	return nil
}

// SnapshotExists checks if a snapshot exists in S3.
func (c *Client) SnapshotExists(snapshotName string) (bool, error) {
	ctx := context.Background()

	prefix := c.buildKey(snapshotName, "")

	resp, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, err
	}

	return len(resp.Contents) > 0, nil
}

// buildKey builds the S3 key for a file.
func (c *Client) buildKey(snapshotName, filePath string) string {
	parts := []string{c.prefix, snapshotName, filePath}
	
	// Remove empty parts
	var cleanParts []string
	for _, p := range parts {
		if p != "" {
			cleanParts = append(cleanParts, p)
		}
	}

	key := strings.Join(cleanParts, "/")
	return strings.ReplaceAll(key, "\\", "/") // Normalize Windows paths
}

// getContentType determines the content type based on file extension.
func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return "application/yaml"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// ConfigFromEnv creates S3 config from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Bucket:    os.Getenv("DEPLOY_CLUSTER_S3_BUCKET"),
		Prefix:    os.Getenv("DEPLOY_CLUSTER_S3_PREFIX"),
		Region:    os.Getenv("AWS_REGION"),
		Endpoint:  os.Getenv("DEPLOY_CLUSTER_S3_ENDPOINT"),
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
}

// IsConfigured checks if S3 is configured via environment variables.
func IsConfigured() bool {
	return os.Getenv("DEPLOY_CLUSTER_S3_BUCKET") != ""
}
