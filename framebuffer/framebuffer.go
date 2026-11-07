package framebuffer

import (
	"errors"
	"fmt"

	"github.com/BeatGlow/display/v2"
	"github.com/BeatGlow/display/v2/framebuffer/fbdev"
	"github.com/BeatGlow/display/v2/framebuffer/kms"
)

// Config configures the framebuffer output backends. A nil KMS config uses the
// KMS defaults. An empty FBDev path uses /dev/fb0.
type Config struct {
	KMS   *kms.Config
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
