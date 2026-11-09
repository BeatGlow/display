package framebuffer

import (
	"errors"
	"fmt"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/framebuffer/fbdev"
	"github.com/BeatGlow/display/framebuffer/kms"
)

// Config configures the framebuffer output backends. A nil KMS config uses the
// KMS defaults. An empty FBDev path uses /dev/fb0.
type Config struct {
	// KMS configures the DRM/KMS backend using [kms.Config].
	KMS *kms.Config
	// FBDev is the framebuffer device path, such as /dev/fb0.
	FBDev string
}

// Open opens the first available backend, preferring DRM/KMS and falling back
// to fbdev. If neither backend opens, the returned error includes both causes.
func Open(config *Config) (display.Display, error) {
	if config == nil {
		config = &Config{}
	}
	return openWith(config, kms.Open, fbdev.Open)
}

func openWith(config *Config, openKMS func(*kms.Config) (display.Display, error), openFBDev func(string) (display.Display, error)) (display.Display, error) {
	d, kmsErr := openKMS(config.KMS)
	if kmsErr == nil {
		return d, nil
	}

	d, fbdevErr := openFBDev(config.FBDev)
	if fbdevErr == nil {
		return d, nil
	}

	return nil, fmt.Errorf("framebuffer: no backend available: %w", errors.Join(
		fmt.Errorf("KMS: %w", kmsErr),
		fmt.Errorf("fbdev: %w", fbdevErr),
	))
}
