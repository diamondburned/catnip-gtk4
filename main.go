package main

import (
	"context"
	"log"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"libdb.so/catnip-gtk4/internal/catnipctl"
	"libdb.so/catnip-gtk4/internal/catnipgtk"
	"libdb.so/catnip-gtk4/internal/catnipgtk/preferences"

	_ "github.com/noriah/catnip/input/all"
)

var _ = cssutil.WriteCSS(`
	.catnip-window {
		background: @theme_bg_color;
	}
`)

func main() {
	// Register for libadwaita.
	app.Hook(func(app *app.Application) { app.ConnectActivate(adw.Init) })

	a := app.New(context.Background(), "so.libdb.catnip-gtk4", "catnip-gtk4")
	a.ConnectActivate(func() { activate(a.Context()) })
	a.RunMain()
}

func activate(ctx context.Context) {
	config, err := catnipgtk.RestoreConfig()
	if err != nil {
		log.Println("cannot restore config:", err)
		log.Println("using default config")
		config = catnipgtk.DefaultConfig()
	}

	display := catnipgtk.NewCairoDisplay(config.SampleRate, config.SampleSize)
	instance := catnipctl.NewInstance(ctx, config, display)

	a := app.FromContext(ctx)
	a.ConnectShutdown(func() { instance.Finalize() })

	prefs := preferences.NewPreferences(instance)

	w := catnipgtk.NewWindow(adw.NewApplicationWindow(a.Application), display)
	gtkutil.BindPopoverMenuAtMouse(w, gtk.PosBottom, [][2]string{
		{"Preferences", "win.prefs"},
		{"About", "win.about"},
		{"Logs", "win.logs"},
		{"Quit", "win.quit"},
	})
	gtkutil.BindActionMap(w, map[string]func(){
		"win.prefs": func() { prefs.Show() },
		"win.logs":  func() { logui.ShowDefaultViewer(ctx) },
		"win.about": func() {}, // TODO
		"win.quit":  func() { a.Quit() },
	})

	w.Window().Show()
	instance.Start()
}
