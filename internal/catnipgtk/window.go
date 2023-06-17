package catnipgtk

import (
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Window is the main catnip visualizer window.
type Window struct {
	*adw.ApplicationWindow
}

// NewWindow creates a new catnip visualizer window.
func NewWindow[AdwWindow *adw.Window | *adw.ApplicationWindow](window AdwWindow, display Display) *Window {
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

	w := gtk.Widgetter(window).(interface {
		gtk.Widgetter
		AddCSSClass(string)
		SetTitle(string)
		SetDefaultSize(int, int)
		SetContent(gtk.Widgetter)
	})
	w.AddCSSClass("catnip-window")
	w.SetTitle("Catnip")
	w.SetDefaultSize(600, 350)
	w.SetContent(woverlay)

	return &Window{w}
}
