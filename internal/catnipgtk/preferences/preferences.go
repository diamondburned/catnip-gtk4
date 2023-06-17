package preferences

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"path/filepath"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/noriah/catnip/input"
	"libdb.so/catnip-gtk4/internal/catnipctl"
	"libdb.so/catnip-gtk4/internal/catnipgtk"
)

//go:embed preferences.blueprint.ui
var preferencesBlueprintUI string

const blueprintIDRegex = `[^\s:]+ +(\S+?) +{`

// Preferences is a wrapper around the preferences window.
type Preferences struct {
	*adw.PreferencesWindow
	built struct {
		Preference         *adw.PreferencesWindow `name:"preference"`
		Backend            *adw.ComboRow          `name:"backend"`
		Device             *adw.ComboRow          `name:"device"`
		Monaural           *gtk.Switch            `name:"monaural"`
		SamplingGroup      *adw.PreferencesGroup  `name:"samplingGroup"`
		SampleRate         *gtk.SpinButton        `name:"sampleRate"`
		SampleSize         *gtk.SpinButton        `name:"sampleSize"`
		WindowFunc         *adw.ComboRow          `name:"windowFunc"`
		SmoothFactor       *gtk.SpinButton        `name:"smoothFactor"`
		DrawStyle          *adw.ComboRow          `name:"drawStyle"`
		LineCap            *adw.ComboRow          `name:"lineCap"`
		LineWidth          *gtk.SpinButton        `name:"lineWidth"`
		GapWidth           *gtk.SpinButton        `name:"gapWidth"`
		OpenCustomCSS      *gtk.Button            `name:"openCustomCSS"`
		ShowWindowControls *gtk.Switch            `name:"showWindowControls"`
	}
	controlling *catnipctl.Instance
	ctx         context.Context
}

// NewPreferences creates a new preferences window.
func NewPreferences(controlling *catnipctl.Instance) *Preferences {
	p := &Preferences{
		controlling: controlling,
		ctx:         controlling.Context(),
	}

	builder := gtk.NewBuilderFromString(preferencesBlueprintUI, -1)
	gtkutil.MustUnmarshalBuilder(&p.built, builder)

	p.PreferencesWindow = p.built.Preference

	p.built.Backend.SetModel(gtk.NewStringList(input.GetAllBackendNames()))
	p.built.WindowFunc.SetModel(windowFuncsModel)
	p.built.DrawStyle.SetModel(drawStylesModel)
	p.built.LineCap.SetModel(lineCapsModel)

	var deviceNames []string
	var deviceNamesModel *gtk.StringList

	p.built.Backend.NotifyProperty("selected", func() {
		defer p.save(p.controlling.Config())

		resume := p.controlling.PauseUpdates()
		defer resume()

		backend := input.Backends[p.built.Backend.Selected()]
		var device string
		p.update(func(config *catnipgtk.Config) {
			config.Backend = backend.Name
			device = config.Device
		})

		devices, err := backend.Devices()
		if err != nil {
			log.Println("Failed to get devices:", err)
			return
		}

		deviceNames = append(
			[]string{"(default)"},
			mapSlice(devices, input.Device.String)...,
		)
		deviceNamesModel = gtk.NewStringList(deviceNames)
		p.built.Device.SetModel(deviceNamesModel)

		// Try to restore the previous device when switching backends,
		// defaulting to the first device if not found. The emitted signal will
		// update the config.
		log.Println("Restoring device:", device)
		p.built.Device.SetSelected(uint(findOr(deviceNames, device, 0)))
	})

	p.built.Device.NotifyProperty("selected", func() {
		if deviceNamesModel == nil {
			log.Println("unexpected nil devicesModel")
			return
		}

		device := deviceNamesModel.String(p.built.Device.Selected())
		if device == "(default)" {
			device = ""
		}

		p.update(func(config *catnipgtk.Config) {
			config.Device = device
		})
	})

	p.built.Monaural.NotifyProperty("active", func() {
		var ch int
		if p.built.Monaural.Active() {
			ch = 1
		} else {
			ch = 2
		}

		p.update(func(config *catnipgtk.Config) {
			config.ChannelCount = ch
		})
	})

	p.built.SampleRate.ConnectValueChanged(func() {
		p.update(func(config *catnipgtk.Config) {
			config.SampleRate = p.built.SampleRate.Value()
		})
	})

	p.built.SampleSize.ConnectValueChanged(func() {
		p.update(func(config *catnipgtk.Config) {
			config.SampleSize = int(p.built.SampleSize.Value())
		})
	})

	// TODO: figure out how to add custom parameters
	p.built.WindowFunc.NotifyProperty("selected", func() {
		ix := p.built.WindowFunc.Selected()
		if int(ix) >= len(windowFuncs) {
			log.Println("Invalid window function index:", ix)
			return
		}

		p.update(func(config *catnipgtk.Config) {
			config.WindowFunc = windowFuncs[ix]
		})
	})

	p.built.SmoothFactor.ConnectValueChanged(func() {
		p.update(func(config *catnipgtk.Config) {
			config.SmoothingFactor = p.built.SmoothFactor.Value()
		})
	})

	p.built.DrawStyle.NotifyProperty("selected", func() {
		p.update(func(config *catnipgtk.Config) {
			config.DrawStyle = drawStyles[p.built.DrawStyle.Selected()]
		})
	})

	p.built.LineCap.NotifyProperty("selected", func() {
		p.update(func(config *catnipgtk.Config) {
			config.LineCap = lineCaps[p.built.LineCap.Selected()]
		})
	})

	p.built.LineWidth.ConnectValueChanged(func() {
		p.update(func(config *catnipgtk.Config) {
			config.LineWidth = p.built.LineWidth.Value()
		})
	})

	p.built.GapWidth.ConnectValueChanged(func() {
		p.update(func(config *catnipgtk.Config) {
			config.GapWidth = p.built.GapWidth.Value()
		})
	})

	p.built.OpenCustomCSS.ConnectClicked(func() {
		app.OpenURI(p.ctx, "file://"+filepath.ToSlash(catnipgtk.ConfigDir)+"/user.css")
	})

	p.built.ShowWindowControls.NotifyProperty("active", func() {
		p.update(func(config *catnipgtk.Config) {
			config.WindowControls = p.built.ShowWindowControls.Active()
		})
	})

	currentConfig := controlling.Config()

	resume := controlling.PauseUpdates()
	defer resume()

	defer func() { log.Println(controlling.Config()) }()

	p.built.Backend.SetSelected(uint(findOr(input.GetAllBackendNames(), currentConfig.Backend, 0)))
	p.built.Monaural.SetActive(currentConfig.ChannelCount == 1)
	p.built.SampleRate.SetValue(currentConfig.SampleRate)
	p.built.SampleSize.SetValue(float64(currentConfig.SampleSize))
	p.built.WindowFunc.SetSelected(uint(findOr(windowFuncs, currentConfig.WindowFunc, 0)))
	p.built.SmoothFactor.SetValue(currentConfig.SmoothingFactor)
	p.built.DrawStyle.SetSelected(uint(findOr(drawStyles, currentConfig.DrawStyle, 0)))
	p.built.LineCap.SetSelected(uint(findOr(lineCaps, currentConfig.LineCap, 0)))
	p.built.LineWidth.SetValue(currentConfig.LineWidth)
	p.built.GapWidth.SetValue(currentConfig.GapWidth)
	p.built.ShowWindowControls.SetActive(currentConfig.WindowControls)

	return p
}

func (p *Preferences) updateSamplingGroup(config *catnipgtk.Config) {
	fₛ := float64(config.SampleRate) / float64(config.SampleSize)
	p.built.SamplingGroup.SetDescription(fmt.Sprintf(
		"fₛ ≈ %.1f samples/s, latency ≈ %.1fms", fₛ, 1000/fₛ,
	))
}

func (p *Preferences) save(cfg *catnipgtk.Config) {
	p.controlling.Config().SaveAsync(func(err error) {
		if err != nil {
			log.Println("failed to save preferences:", err)
			p.PreferencesWindow.AddToast(newErrorToast())
		}
	})
}

func (p *Preferences) update(f func(cfg *catnipgtk.Config)) {
	var cfg *catnipgtk.Config
	p.controlling.Update(func(c *catnipgtk.Config) {
		f(c)
		cfg = c
	})

	p.updateSamplingGroup(cfg)

	if !p.controlling.UpdateIsPaused() {
		p.save(cfg)
	}
}

func setComboBoxText(combo *gtk.ComboBoxText, values []string) {
	combo.RemoveAll()
	for _, value := range values {
		combo.Append(value, value)
	}
}

func mapSlice[From, To any](slice []From, f func(From) To) []To {
	result := make([]To, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

func findOr[T comparable](slice []T, value T, defaultIx int) int {
	for i, v := range slice {
		if v == value {
			return i
		}
	}
	return defaultIx
}

var windowFuncs = []catnipgtk.WindowFunc{
	catnipgtk.WindowRectangle,
	catnipgtk.WindowLanczos,
	catnipgtk.WindowHamming,
	catnipgtk.WindowHann,
	catnipgtk.WindowBartlett,
	catnipgtk.WindowBlackman,
}

var windowFuncsModel = gtk.NewStringList([]string{
	"Rectangle",
	"Lanczos",
	"Hamming",
	"Hann",
	"Bartlett",
	"Blackman",
})

var lineCaps = []cairo.LineCap{
	cairo.LineCapButt,
	cairo.LineCapRound,
	cairo.LineCapSquare,
}

var lineCapsModel = gtk.NewStringList([]string{
	"Butt",
	"Round",
	"Square",
})

var drawStyles = []catnipgtk.DrawStyle{
	catnipgtk.DrawBottomBars,
	catnipgtk.DrawLines,
}

var drawStylesModel = gtk.NewStringList([]string{
	"Bottom Bars",
	"Lines",
})

func newErrorToast() *adw.Toast {
	toast := adw.NewToast("Error saving preferences")
	toast.SetTimeout(0)
	toast.SetActionName("win.logs")
	toast.SetButtonLabel("View logs")
	toast.SetPriority(adw.ToastPriorityHigh)
	return toast
}
