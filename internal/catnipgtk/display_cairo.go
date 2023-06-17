package catnipgtk

import (
	"math"
	"sync"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/noriah/catnip/input"

	window "github.com/noriah/catnip/util"
)

// CairoDisplay is a display of audio data using the Cairo vector graphics
// library.
type CairoDisplay struct {
	*gtk.DrawingArea

	window    *window.MovingWindow
	drawStyle DrawStyle

	background struct {
		surface *cairo.Surface
		context *cairo.Context
		width   int
		height  int
	}

	lock sync.Mutex

	binsBuffer [][]float64
	nchannels  int
	peak       float64
	scale      float64
	zeroes     int

	barWidth   float64
	spaceWidth float64
	binWidth   float64
	lineCap    cairo.LineCap

	width  int
	height int
}

var _ Display = (*CairoDisplay)(nil)

// NewCairoDisplay creates a new display.
func NewCairoDisplay(sampleRate float64, sampleSize int) *CairoDisplay {
	d := &CairoDisplay{}
	d.SetSizes(2, 3)
	d.SetLineCap(cairo.LineCapRound)
	d.SetDrawStyle(DrawBottomBars)
	d.SetSamplingParams(sampleRate, sampleSize)

	d.DrawingArea = gtk.NewDrawingArea()
	d.DrawingArea.AddCSSClass("catnip-display")
	d.DrawingArea.SetDrawFunc(d.draw)
	d.DrawingArea.AddTickCallback(func(widget gtk.Widgetter, clock gdk.FrameClocker) (ok bool) {
		base := gtk.BaseWidget(widget)
		base.QueueDraw()
		return glib.SOURCE_CONTINUE
	})

	return d
}

// SetSizes sets the sizes of the bars and spaces in the display.
func (d *CairoDisplay) SetSizes(bar, space float64) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.barWidth = bar
	d.spaceWidth = space
	d.binWidth = bar + space
}

// SetDrawStyle sets the draw style.
func (d *CairoDisplay) SetDrawStyle(style DrawStyle) {
	d.drawStyle = style
}

// SetLineCap sets the line cap.
func (d *CairoDisplay) SetLineCap(lineCap cairo.LineCap) {
	d.lineCap = lineCap
}

// SetSamplingParams sets the sampling rate and size.
func (d *CairoDisplay) SetSamplingParams(rate float64, size int) {
	windowSize := ((int(ScalingWindow * rate)) / size) * 2

	d.lock.Lock()
	defer d.lock.Unlock()

	d.window = window.NewMovingWindow(windowSize)
}

// QueueDraw queues a draw.
func (d *CairoDisplay) QueueDraw() {
	glib.IdleAdd(d.DrawingArea.QueueDraw)
}

// AsOutput returns the Display as a processor.Output.
func (d *CairoDisplay) AsOutput() DiscardableOutput {
	return WrapDiscardableOutput((*displayOutput)(d))
}

type displayOutput CairoDisplay

// Write implements processor.Output.
func (d *displayOutput) Write(bins [][]float64, nchannels int) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if len(d.binsBuffer) < len(bins) || len(d.binsBuffer[0]) < len(bins[0]) {
		d.binsBuffer = input.MakeBuffers(len(bins), len(bins[0]))
	}
	input.CopyBuffers(d.binsBuffer, bins)

	nbins := (*CairoDisplay)(d).bins(nchannels)
	var peak float64

	for i := 0; i < nchannels; i++ {
		for _, val := range bins[i][:nbins] {
			if val > peak {
				peak = val
			}
		}
	}

	d.peak = peak
	d.scale = 1.0
	d.nchannels = nchannels

	if d.peak >= PeakThreshold {
		// do some scaling if we are above the PeakThreshold
		vMean, vSD := d.window.Update(d.peak)
		if t := vMean + (2.0 * vSD); t > 1.0 {
			d.scale = t
		}

		d.zeroes = 0
	} else if d.zeroes < ZeroThreshold {
		d.zeroes++
	}

	return nil
}

// Bins implements processor.Output.
func (d *displayOutput) Bins(nchannels int) int {
	d.lock.Lock()
	defer d.lock.Unlock()

	return (*CairoDisplay)(d).bins(nchannels)
}

func (d *CairoDisplay) bins(nchannels int) int {
	return d.width / int(d.binWidth)
}

func (d *CairoDisplay) draw(area *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	wf := float64(width)
	hf := float64(height)

	if d.background.width != width || d.background.height != height {
		// Render the background onto the surface and use that as the source
		// surface for our context.
		d.background.surface = cr.Target().CreateSimilar(cairo.ContentColorAlpha, width, height)
		d.background.context = cairo.Create(d.background.surface)
		d.background.width = width
		d.background.height = height
	}

	// Clear the background surface.
	d.background.context.SetSourceRGBA(0, 0, 0, 0)
	d.background.context.SetOperator(cairo.OperatorSource)
	d.background.context.Paint()

	// Draw the CSS background. We use the .catnip-background to get the
	// CSS-drawn background, but we don't want to keep it around, so we
	// remove the class after we're done.
	styles := area.StyleContext()
	styles.Save()
	defer styles.Restore()

	styles.AddClass("catnip-background")
	gtk.RenderBackground(styles, d.background.context, 0, 0, wf, hf)

	cr.SetAntialias(cairo.AntialiasFast)
	cr.SetLineWidth(d.barWidth)
	cr.SetLineCap(d.lineCap)
	cr.SetSourceSurface(d.background.surface, 0, 0)

	d.lock.Lock()
	defer d.lock.Unlock()

	d.width = width
	d.height = height

	switch d.drawStyle {
	case DrawBottomBars:
		d.drawBottomBars(cr, wf, hf)
	case DrawLines:
		d.drawLines(cr, wf, hf)
	}
}

func (d *CairoDisplay) drawBottomBars(cr *cairo.Context, wf, hf float64) {
	bins := d.binsBuffer

	delta := 1
	scale := hf / d.scale
	nbars := d.bins(d.nchannels)

	// Round up the width so we don't draw a partial bar.
	xColMax := math.Round(wf/d.binWidth) * d.binWidth

	xBin := 0
	xCol := (d.binWidth)/2 + (wf-xColMax)/2

	for _, chBins := range bins {
		for xBin < nbars && xBin >= 0 && xCol < xColMax {
			stop := calculateBar(chBins[xBin]*scale, hf)
			d.drawBar(cr, xCol, hf, stop)

			xCol += d.binWidth
			xBin += delta
		}

		delta = -delta
		xBin += delta // ensure xBin is not out of bounds first.
	}
}

func (d *CairoDisplay) drawBar(cr *cairo.Context, xCol, to, from float64) {
	cr.MoveTo(xCol, from)
	cr.LineTo(xCol, to)
	cr.Stroke()
}

func (d *CairoDisplay) drawLines(cr *cairo.Context, wf, hf float64) {
	bins := d.binsBuffer
	scale := hf / d.scale
	nbars := d.bins(d.nchannels)

	// Flip this to iterate backwards and draw the other channel.
	delta := +1

	x := 0.0
	// Recalculate the bin width to be equally distributed throughout the width
	// without any gaps on either end. Ignore the last bar (-1-1) because it
	// peaks up for some reason.
	barCount := math.Min(
		math.Round(wf/d.binWidth),
		float64((nbars-2)*d.nchannels),
	)
	binWidth := wf / barCount

	var bar int
	first := true

	for _, ch := range bins {
		// If we're iterating backwards, then check the lower bound, or
		// if we're iterating forwards, then check the upper bound.
		// Ignore the last bar for the same reason above.
		for bar >= 0 && bar < nbars-1 {
			y := calculateBar(ch[bar]*scale, hf)
			if first {
				// First.
				cr.MoveTo(x, y)
				first = false
			} else if next := bar + delta; next >= 0 && next < len(ch) {
				// Average out the middle Y point with the next one for
				// smoothing.
				ynext := calculateBar(ch[next]*scale, hf)
				quadCurve(cr, x, y, x+(binWidth)/2, (y+ynext)/2)
			} else {
				// Ignore the last point's value and just use the ceiling.
				cr.LineTo(x, y)
			}

			x += binWidth
			bar += delta
		}

		delta = -delta
		bar += delta
	}

	// Commit the line.
	cr.Stroke()
}

// quadCurve draws a quadratic bezier curve into the given Cairo context.
func quadCurve(cr *cairo.Context, p1x, p1y, p2x, p2y float64) {
	p0x, p0y := cr.CurrentPoint()

	// https://stackoverflow.com/a/55034115
	cp1x := p0x + ((2.0 / 3.0) * (p1x - p0x))
	cp1y := p0y + ((2.0 / 3.0) * (p1y - p0y))

	cp2x := p2x + ((2.0 / 3.0) * (p1x - p2x))
	cp2y := p2y + ((2.0 / 3.0) * (p1y - p2y))

	cr.CurveTo(cp1x, cp1y, cp2x, cp2y, p2x, p2y)
}
