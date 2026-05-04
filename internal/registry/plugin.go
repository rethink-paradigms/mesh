package registry

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	meshplugin "github.com/rethink-paradigms/mesh/internal/plugin"
)

type RegistryConfig struct {
	Type            string
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
}

type S3RegistryPlugin struct {
	cfg    RegistryConfig
	client *s3.Client
}

func NewS3RegistryPlugin(cfg RegistryConfig) (*S3RegistryPlugin, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("registry: bucket is required")
	}

	if cfg.Region == "" {
		cfg.Region = os.Getenv("AWS_REGION")
		if cfg.Region == "" {
			cfg.Region = os.Getenv("AWS_DEFAULT_REGION")
		}
	}
	if cfg.AccessKeyID == "" {
		cfg.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if cfg.SecretAccessKey == "" {
		cfg.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = os.Getenv("AWS_ENDPOINT_URL_S3")
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("registry: load AWS config: %w", err)
	}

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		awsCfg.Credentials = credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)
	}

	opts := []func(*s3.Options){}
	if cfg.Endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, opts...)

	return &S3RegistryPlugin{
		cfg:    cfg,
		client: client,
	}, nil
}

func (p *S3RegistryPlugin) PluginInfo(ctx context.Context) (meshplugin.PluginMeta, error) {
	return meshplugin.PluginMeta{
		Name:        "s3-registry",
		Version:     "1.0.0",
		Description: "S3-backed snapshot registry plugin",
		Author:      "mesh",
	}, nil
}
