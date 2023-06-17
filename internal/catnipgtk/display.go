package catnipgtk

import (
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/noriah/catnip/processor"
)

var _ = cssutil.WriteCSS(`
	.catnip-background {
		background-color: @theme_fg_color;
	}
`)

const ScalingWindow = 1.5 // seconds
const PeakThreshold = 0.01
const ZeroThreshold = 5

// DrawStyle is the style of drawing.
type DrawStyle int

const (
	// DrawBottomBars draws vertical bars from the bottom.
	DrawBottomBars DrawStyle = iota
	// DrawLines draws lines across the display.
	DrawLines
)

// Display is a display of audio data.
type Display interface {
	gtk.Widgetter
	AsOutput() DiscardableOutput
	// SetSizes sets the sizes of the bars and spaces in the display.
	SetSizes(bar, space float64)
	// SetDrawStyle sets the style of drawing.
	SetDrawStyle(style DrawStyle)
	// SetLineCap sets the line cap of the display.
	SetLineCap(lineCap cairo.LineCap)
	// SetSamplingParams sets the sampling rate and size.
	SetSamplingParams(rate float64, size int)
}

// DiscardableOutput extends processor.Output with a Discard method.
type DiscardableOutput interface {
	processor.Output
	// Discard prevents the output from being used again.
	Discard()
}

// WrapDiscardableOutput wraps an output in a DiscardableOutput.
func WrapDiscardableOutput(output processor.Output) DiscardableOutput {
	return &discardableOutput{output: output}
}

type discardableOutput struct {
	output    processor.Output
	discarded uint32
}

func (d *discardableOutput) Bins(nchannels int) int {
	if atomic.LoadUint32(&d.discarded) != 0 {
		return 0
	}
	return d.output.Bins(nchannels)
}

func (d *discardableOutput) Write(bins [][]float64, nchannels int) error {
	if atomic.LoadUint32(&d.discarded) != 0 {
		return nil
	}
	return d.output.Write(bins, nchannels)
}

func (d *discardableOutput) Discard() {
	atomic.StoreUint32(&d.discarded, 1)
}

func calculateBar(value, height float64) float64 {
	bar := min(value, height)
	return height - bar
}

func max[T ~int | ~float64](i, j T) T {
	if i > j {
		return i
	}
	return j
}

func min[T ~int | ~float64](i, j T) T {
	if i < j {
		return i
	}
	return j
}
