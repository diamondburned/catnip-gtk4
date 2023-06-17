package catnipctl

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/noriah/catnip"
	"github.com/noriah/catnip/dsp"
	"libdb.so/catnip-gtk4/internal/catnipgtk"
)

// Instance is a singleton instance of a catnip visualizer.
// It makes it easier to start and stop the visualizer with settings.
type Instance struct {
	config    catnipgtk.Config
	display   catnipgtk.Display
	parentCtx context.Context

	wg      sync.WaitGroup
	stop    context.CancelFunc
	paused  int  // nested pause counter
	changed bool // true if changed while paused
}

// NewInstance creates a new instance of the catnip visualizer.
func NewInstance(ctx context.Context, config catnipgtk.Config, display catnipgtk.Display) *Instance {
	return &Instance{
		config:    config,
		display:   display,
		parentCtx: ctx,
	}
}

// Config returns a copy of the current configuration.
func (i *Instance) Config() *catnipgtk.Config {
	cfg := i.config
	return &cfg
}

// Context returns the context of the instance.
func (i *Instance) Context() context.Context {
	return i.parentCtx
}

// PauseUpdates pauses the catnip visualizer from updating. It returns a
// function that resumes the updates. If the config is changed while the
// visualizer is paused, it will be restarted once it is resumed.
func (i *Instance) PauseUpdates() (resume func()) {
	old := i.config
	i.paused++

	var once sync.Once
	return func() {
		once.Do(func() {
			i.paused--
			if old != i.config {
				// Mark as changed in case we're nested but can't apply the
				// changes.
				i.changed = true
			}
			if i.paused == 0 && i.stop != nil && i.changed {
				// Only restart if we're not nested and we have changes.
				i.changed = false
				i.Start()
			}
		})
	}
}

// UpdateIsPaused returns true if the Update function is paused.
func (i *Instance) UpdateIsPaused() bool {
	return i.paused != 0
}

// Update updates the catnip visualizer with the new settings and restarts it.
// If the visualizer is not running, it will not be started.
func (i *Instance) Update(f func(cfg *catnipgtk.Config)) {
	old := i.config
	f(&i.config)

	if i.paused != 0 {
		return
	}

	if catnipgtk.ConfigOnlyChangedDisplay(old, i.config) {
		i.display.SetSizes(i.config.LineWidth, i.config.GapWidth)
		i.display.SetLineCap(i.config.LineCap)
		i.display.SetDrawStyle(i.config.DrawStyle)
		return
	}

	if i.stop != nil {
		i.Start()
	}
}

func (i *Instance) convertConfig(c catnipgtk.Config) catnip.Config {
	output := i.display.AsOutput()

	return catnip.Config{
		Backend:      c.Backend,
		Device:       c.Device,
		SampleRate:   c.SampleRate,
		SampleSize:   c.SampleSize,
		ChannelCount: c.ChannelCount,
		ProcessRate:  c.ProcessRate,
		Windower:     catnipgtk.WindowFuncs[c.WindowFunc],
		Output:       output,
		SetupFunc: func() error {
			done := make(chan struct{})
			glib.IdleAdd(func() {
				i.display.SetSizes(c.LineWidth, c.GapWidth)
				i.display.SetLineCap(c.LineCap)
				i.display.SetDrawStyle(c.DrawStyle)
				i.display.SetSamplingParams(c.SampleRate, c.SampleSize)
				close(done)
			})
			<-done
			return nil
		},
		StartFunc: func(ctx context.Context) (context.Context, error) {
			return ctx, nil
		},
		CleanupFunc: func() error {
			log.Println("CleanupFunc")
			output.Discard()
			return nil
		},
		Analyzer: dsp.NewAnalyzer(dsp.AnalyzerConfig{
			SampleRate: c.SampleRate,
			SampleSize: c.SampleSize,
			SquashLow:  true,
			BinMethod:  dsp.MaxSampleValue(),
		}),
		Smoother: dsp.NewSmoother(dsp.SmootherConfig{
			SampleRate:      c.SampleRate,
			SampleSize:      c.SampleSize,
			ChannelCount:    c.ChannelCount,
			SmoothingFactor: c.SmoothingFactor,
			SmoothingMethod: c.SmoothingMethod,
		}),
	}
}

func (i *Instance) error(err error) {
	app.Error(i.parentCtx, fmt.Errorf("catnip: %w", err))
}

// Finalize kills all instances and wait for them to finish.
// No more methods should be called after this.
func (i *Instance) Finalize() {
	i.Stop()
	i.wg.Wait()
}

// Start starts the catnip visualizer. If it is already running, it will be
// restarted.
func (i *Instance) Start() {
	i.Stop()
	ctx, cancel := context.WithCancel(i.parentCtx)
	i.stop = cancel

	cfg := i.convertConfig(i.config)

	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		if err := catnip.Run(&cfg, ctx); err != nil {
			i.error(err)
		}
	}()
}

// Stop stops the catnip visualizer. It does not wait for it to finish.
func (i *Instance) Stop() {
	if i.stop != nil {
		i.stop()
	}
}
