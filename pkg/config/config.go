package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
)

// Config for the environment
type Config struct {
	Debug            bool   `envconfig:"DEBUG"`
	SourceBucket     string `envconfig:"SOURCE_BUCKET"`
	ArtifactBucket   string `envconfig:"ARTIFACT_BUCKET"`
	BuildProjectArn  string `envconfig:"BUILD_PROJECT_ARN"`
	DeployProjectArn string `envconfig:"DEPLOY_PROJECT_ARN"`
}

// NewDefaultConfig reads configuration from environment variables and validates it
func NewDefaultConfig() (*Config, error) {
	cfg := new(Config)
	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse environment config")
	}

	return cfg, nil
}
