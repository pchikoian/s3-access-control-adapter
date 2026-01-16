package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/s3-access-control-adapter/internal/config"
)

// S3Response represents the response from S3
type S3Response struct {
	StatusCode    int
	Headers       http.Header
	Body          io.ReadCloser
	ContentLength int64
}

// S3Client wraps the AWS S3 client for proxying requests
type S3Client struct {
	client *s3.Client
	cfg    *config.AWSConfig
}

// NewS3Client creates a new S3 client
func NewS3Client(ctx context.Context, cfg *config.AWSConfig) (*S3Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	// Use static credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Opts := []func(*s3.Options){}

	// Custom endpoint for LocalStack or other S3-compatible services
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = cfg.UsePathStyle
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	return &S3Client{
		client: client,
		cfg:    cfg,
	}, nil
}

// Forward forwards an S3 request and returns the response
func (c *S3Client) Forward(ctx context.Context, req *S3Request) (*S3Response, error) {
	switch req.Action {
	case "s3:GetObject":
		return c.getObject(ctx, req)
	case "s3:PutObject":
		return c.putObject(ctx, req)
	case "s3:DeleteObject":
		return c.deleteObject(ctx, req)
	case "s3:ListBucket":
		return c.listObjects(ctx, req)
	case "s3:HeadObject":
		return c.headObject(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported action: %s", req.Action)
	}
}

func (c *S3Client) getObject(ctx context.Context, req *S3Request) (*S3Response, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
	}

	// Pass through relevant headers
	if v := req.Headers.Get("Range"); v != "" {
		input.Range = aws.String(v)
	}
	if v := req.Headers.Get("If-Match"); v != "" {
		input.IfMatch = aws.String(v)
	}
	if v := req.Headers.Get("If-None-Match"); v != "" {
		input.IfNoneMatch = aws.String(v)
	}

	output, err := c.client.GetObject(ctx, input)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)
	if output.ContentType != nil {
		headers.Set("Content-Type", *output.ContentType)
	}
	if output.ContentLength != nil {
		headers.Set("Content-Length", fmt.Sprintf("%d", *output.ContentLength))
	}
	if output.ETag != nil {
		headers.Set("ETag", *output.ETag)
	}
	if output.LastModified != nil {
		headers.Set("Last-Modified", output.LastModified.Format(http.TimeFormat))
	}
	if output.ContentEncoding != nil {
		headers.Set("Content-Encoding", *output.ContentEncoding)
	}
	if output.CacheControl != nil {
		headers.Set("Cache-Control", *output.CacheControl)
	}

	contentLength := int64(0)
	if output.ContentLength != nil {
		contentLength = *output.ContentLength
	}

	return &S3Response{
		StatusCode:    http.StatusOK,
		Headers:       headers,
		Body:          output.Body,
		ContentLength: contentLength,
	}, nil
}

func (c *S3Client) putObject(ctx context.Context, req *S3Request) (*S3Response, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
		Body:   req.Body,
	}

	if req.ContentLength > 0 {
		input.ContentLength = aws.Int64(req.ContentLength)
	}
	if v := req.Headers.Get("Content-Type"); v != "" {
		input.ContentType = aws.String(v)
	}
	if v := req.Headers.Get("Content-Encoding"); v != "" {
		input.ContentEncoding = aws.String(v)
	}
	if v := req.Headers.Get("Cache-Control"); v != "" {
		input.CacheControl = aws.String(v)
	}

	output, err := c.client.PutObject(ctx, input)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)
	if output.ETag != nil {
		headers.Set("ETag", *output.ETag)
	}

	return &S3Response{
		StatusCode: http.StatusOK,
		Headers:    headers,
	}, nil
}

func (c *S3Client) deleteObject(ctx context.Context, req *S3Request) (*S3Response, error) {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
	}

	_, err := c.client.DeleteObject(ctx, input)
	if err != nil {
		return nil, err
	}

	return &S3Response{
		StatusCode: http.StatusNoContent,
		Headers:    make(http.Header),
	}, nil
}

func (c *S3Client) listObjects(ctx context.Context, req *S3Request) (*S3Response, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(req.Bucket),
	}

	if prefix := req.QueryParams.Get("prefix"); prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	if delimiter := req.QueryParams.Get("delimiter"); delimiter != "" {
		input.Delimiter = aws.String(delimiter)
	}
	if maxKeys := req.QueryParams.Get("max-keys"); maxKeys != "" {
		var mk int32
		fmt.Sscanf(maxKeys, "%d", &mk)
		input.MaxKeys = aws.Int32(mk)
	}
	if continuationToken := req.QueryParams.Get("continuation-token"); continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}

	output, err := c.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, err
	}

	// Convert to XML response
	body := buildListObjectsXML(req.Bucket, output)

	headers := make(http.Header)
	headers.Set("Content-Type", "application/xml")

	return &S3Response{
		StatusCode:    http.StatusOK,
		Headers:       headers,
		Body:          io.NopCloser(body),
		ContentLength: int64(body.Len()),
	}, nil
}

func (c *S3Client) headObject(ctx context.Context, req *S3Request) (*S3Response, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
	}

	output, err := c.client.HeadObject(ctx, input)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)
	if output.ContentType != nil {
		headers.Set("Content-Type", *output.ContentType)
	}
	if output.ContentLength != nil {
		headers.Set("Content-Length", fmt.Sprintf("%d", *output.ContentLength))
	}
	if output.ETag != nil {
		headers.Set("ETag", *output.ETag)
	}
	if output.LastModified != nil {
		headers.Set("Last-Modified", output.LastModified.Format(http.TimeFormat))
	}

	return &S3Response{
		StatusCode: http.StatusOK,
		Headers:    headers,
	}, nil
}

// buildListObjectsXML builds the XML response for ListObjectsV2
func buildListObjectsXML(bucket string, output *s3.ListObjectsV2Output) *stringBuffer {
	buf := &stringBuffer{}
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
	buf.WriteString(fmt.Sprintf("<Name>%s</Name>", bucket))

	if output.Prefix != nil {
		buf.WriteString(fmt.Sprintf("<Prefix>%s</Prefix>", *output.Prefix))
	} else {
		buf.WriteString("<Prefix></Prefix>")
	}

	if output.MaxKeys != nil {
		buf.WriteString(fmt.Sprintf("<MaxKeys>%d</MaxKeys>", *output.MaxKeys))
	}

	buf.WriteString(fmt.Sprintf("<IsTruncated>%t</IsTruncated>", output.IsTruncated != nil && *output.IsTruncated))

	for _, obj := range output.Contents {
		buf.WriteString("<Contents>")
		if obj.Key != nil {
			buf.WriteString(fmt.Sprintf("<Key>%s</Key>", *obj.Key))
		}
		if obj.LastModified != nil {
			buf.WriteString(fmt.Sprintf("<LastModified>%s</LastModified>", obj.LastModified.Format("2006-01-02T15:04:05.000Z")))
		}
		if obj.ETag != nil {
			buf.WriteString(fmt.Sprintf("<ETag>%s</ETag>", *obj.ETag))
		}
		if obj.Size != nil {
			buf.WriteString(fmt.Sprintf("<Size>%d</Size>", *obj.Size))
		}
		buf.WriteString("<StorageClass>STANDARD</StorageClass>")
		buf.WriteString("</Contents>")
	}

	for _, prefix := range output.CommonPrefixes {
		buf.WriteString("<CommonPrefixes>")
		if prefix.Prefix != nil {
			buf.WriteString(fmt.Sprintf("<Prefix>%s</Prefix>", *prefix.Prefix))
		}
		buf.WriteString("</CommonPrefixes>")
	}

	buf.WriteString("</ListBucketResult>")
	return buf
}

// stringBuffer is a simple string buffer that implements io.Reader
type stringBuffer struct {
	data   []byte
	offset int
}

func (b *stringBuffer) WriteString(s string) {
	b.data = append(b.data, []byte(s)...)
}

func (b *stringBuffer) Read(p []byte) (n int, err error) {
	if b.offset >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

func (b *stringBuffer) Len() int {
	return len(b.data)
}
