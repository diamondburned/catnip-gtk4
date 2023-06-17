package catnipgtk

import (
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Window is the main catnip visualizer window.
type Window struct {
	AdwWindow
}

// AdwWindow is the interface for adwaita's ApplicationWindow.
type AdwWindow interface {
	gtk.Widgetter
	AddCSSClass(string)
	SetTitle(string)
	SetDefaultSize(int, int)
	SetContent(gtk.Widgetter)
}

// NewWindow creates a new catnip visualizer window.
func NewWindow(window AdwWindow, display Display) *Window {
	wndh := gtk.NewWindowHandle()
	wndh.SetChild(display)

	wlcontrols := gtk.NewWindowControls(gtk.PackEnd)
	wlcontrols.SetVAlign(gtk.AlignStart)
	wlcontrols.SetHAlign(gtk.AlignEnd)

	wrcontrols := gtk.NewWindowControls(gtk.PackStart)
	wrcontrols.SetVAlign(gtk.AlignStart)
	wrcontrols.SetHAlign(gtk.AlignStart)

	woverlay := gtk.NewOverlay()
	woverlay.AddOverlay(wlcontrols)
	woverlay.AddOverlay(wrcontrols)
	woverlay.SetChild(wndh)

	window.AddCSSClass("catnip-window")
	window.SetTitle("Catnip")
	window.SetDefaultSize(600, 350)
	window.SetContent(woverlay)

	return &Window{window}
}

// Window returns the underlying gtk.Window.
func (w *Window) Window() *gtk.Window {
	switch window := w.AdwWindow.(type) {
	case *adw.Window:
		return &window.Window
	case *adw.ApplicationWindow:
		return &window.Window
	default:
		panic("unknown window type")
	}
}
