package catnipgtk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/noriah/catnip/dsp"
	"github.com/noriah/catnip/dsp/window"
)

// ConfigDir is the directory where the configuration is saved.
var ConfigDir, _ = getConfigDir()
var configFile = filepath.Join(ConfigDir, "config.json")

func getConfigDir() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	cfgDir = filepath.Join(cfgDir, "catnip-gtk4")
	return cfgDir, nil
}

// EnsureConfigDir ensures that the configuration directory exists.
func EnsureConfigDir() error {
	if ConfigDir == "" {
		_, err := getConfigDir()
		if err != nil {
			return err
		}
		return errors.New("catnipgtk: ConfigDir is empty")
	}

	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return fmt.Errorf("catnipgtk: failed to create config directory: %w", err)
	}

	return nil
}

// Config is the configuration for the catnip instance.
type Config struct {
	Backend         string              `json:"backend"`
	Device          string              `json:"device"`
	SampleRate      float64             `json:"sampleRate"`
	SampleSize      int                 `json:"sampleSize"`
	ChannelCount    int                 `json:"channelCount"`
	ProcessRate     int                 `json:"processRate"`
	WindowFunc      WindowFunc          `json:"windowFunc"`
	SmoothingFactor float64             `json:"smoothingFactor"`
	SmoothingMethod dsp.SmoothingMethod `json:"smoothingMethod"`
	DrawStyle       DrawStyle           `json:"drawStyle"`
	LineWidth       float64             `json:"lineWidth"`
	GapWidth        float64             `json:"gapWidth"`
	LineCap         cairo.LineCap       `json:"lineCap"`
	WindowControls  bool                `json:"windowControls"`
}

// WindowFunc is the window function to use for the FFT.
type WindowFunc string

const (
	WindowRectangle WindowFunc = "Rectangle"
	WindowLanczos   WindowFunc = "Lanczos"
	WindowHamming   WindowFunc = "Hamming"
	WindowHann      WindowFunc = "Hann"
	WindowBartlett  WindowFunc = "Bartlett"
	WindowBlackman  WindowFunc = "Blackman"
)

var WindowFuncs = map[WindowFunc]window.Function{
	WindowRectangle: window.Rectangle(),
	WindowLanczos:   window.Lanczos(),
	WindowHamming:   window.Hamming(),
	WindowHann:      window.Hann(),
	WindowBartlett:  window.Bartlett(),
	WindowBlackman:  window.Blackman(),
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Backend:         "pipewire",
		Device:          "",
		SampleRate:      44100,
		SampleSize:      1024,
		ChannelCount:    2,
		WindowFunc:      WindowLanczos,
		SmoothingFactor: 0.6415,
		SmoothingMethod: dsp.SmoothSimpleAverage,
		DrawStyle:       DrawBottomBars,
		LineWidth:       3,
		GapWidth:        3,
		LineCap:         cairo.LineCapRound,
	}
}

// RestoreConfig restores the configuration from the config file. If the
// configuration file does not exist, it returns an error.
func RestoreConfig() (Config, error) {
	if err := EnsureConfigDir(); err != nil {
		return Config{}, err
	}

	f, err := os.Open(configFile)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	var config Config
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return Config{}, fmt.Errorf("catnipgtk: failed to decode config: %w", err)
	}
	return config, nil
}

// SaveAsync saves the configuration asynchronously.
func (c Config) SaveAsync(done func(err error)) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		done(err)
		return
	}

	go func() {
		if err := EnsureConfigDir(); err != nil {
			glib.IdleAdd(func() { done(err) })
			return
		}

		if err := os.WriteFile(configFile, b, 0644); err != nil {
			glib.IdleAdd(func() { done(err) })
			return
		}
	}()
}

// ConfigOnlyChangedDisplay returns whether the only changed fields are
// display-related. If this returns true, then the catnip instance can be
// reused.
func ConfigOnlyChangedDisplay(old, new Config) bool {
	zero := func(cfg *Config) {
		cfg.GapWidth = 0
		cfg.LineWidth = 0
		cfg.DrawStyle = 0
		cfg.LineCap = 0
	}

	zero(&old)
	zero(&new)
	return old == new
}
