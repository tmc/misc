// Command wanix-macapp builds and runs native macOS app bundles for Wanix apps.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/tmc/apple/appkit"
	"github.com/tmc/apple/objc"
	"github.com/tmc/misc/wanix-macapp/internal/webkithost"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		ok, err := runBundledApp()
		if ok || err != nil {
			if err != nil {
				log.Fatal(err)
			}
			return
		}
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "host":
		err = runHost(os.Args[2:])
	case "run":
		err = runOpen(os.Args[2:])
	case "build":
		err = runBuild(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		log.Fatalf("wanix-macapp %s: %v", os.Args[1], err)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: wanix-macapp <command> [arguments]

Commands:
  host      run the WebKit host from a Wanix asset directory
  build     build a small .app bundle around this launcher
  run       open a built .app bundle

The host mounts a Plan 9-style macOS namespace at macos:
  macos/app/ctl
  macos/window/title
  macos/window/ctl
  macos/pasteboard/text
  macos/alert/clone
`)
}

func runHost(args []string) error {
	fs := flag.NewFlagSet("host", flag.ExitOnError)
	assetsDir := fs.String("assets-dir", "", "directory containing wanix.js and wanix.debug.wasm")
	rcWASM := fs.String("rc-wasm", "", "rc.wasm to run in the default terminal")
	url := fs.String("url", "", "Wanix page URL to load instead of -assets-dir")
	visible := fs.Bool("visible", true, "show the WebKit window")
	inspectable := fs.Bool("inspectable", true, "allow Safari Web Inspector")
	selfTest := fs.Bool("self-test", false, "run a fail-closed runtime check")
	readyTimeout := fs.Duration("ready-timeout", 15*time.Second, "maximum time to wait for the Wanix runtime during -self-test")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *assetsDir == "" && *url == "" {
		return fmt.Errorf("-assets-dir or -url is required")
	}
	if *assetsDir != "" && *rcWASM == "" {
		*rcWASM = defaultRCPath(*assetsDir)
	}
	if *assetsDir != "" && *rcWASM == "" {
		return fmt.Errorf("rc.wasm not found; run make -C <wanix>/rc build or pass -rc-wasm")
	}
	appkit.RunApp(func(app appkit.NSApplication, delegate appkit.NSApplicationDelegateObject) {
		host := webkithost.New(webkithost.Config{AssetsDir: *assetsDir, RCPath: *rcWASM, URL: *url, Visible: *visible, Inspectable: *inspectable, ReadyTimeout: *readyTimeout})
		app.SetMainMenu(buildMenuBar())
		if err := host.Load(); err != nil {
			log.Printf("load failed: %v", err)
			app.Terminate(nil)
			os.Exit(1)
		}
		app.Activate()
		if *selfTest {
			go func() {
				if err := host.SelfTest(context.Background()); err != nil {
					log.Printf("self-test failed: %v", err)
					app.Terminate(nil)
					os.Exit(1)
				}
				log.Printf("self-test passed")
				_ = host.Close(context.Background())
				app.Terminate(nil)
			}()
		}
	})
	return nil
}

func buildMenuBar() appkit.NSMenu {
	menuBar := appkit.NewNSMenu()

	appMenuItem := appkit.NewMenuItemWithTitleActionKeyEquivalent("", 0, "")
	appMenu := appkit.NewNSMenu()
	appMenu.AddItemWithTitleActionKeyEquivalent("About Wanix", objc.Sel("orderFrontStandardAboutPanel:"), "")
	appMenu.AddItem(appkit.GetNSMenuItemClass().SeparatorItem())
	appMenu.AddItemWithTitleActionKeyEquivalent("Quit Wanix", objc.Sel("terminate:"), "q")
	appMenuItem.SetSubmenu(appMenu)
	menuBar.AddItem(appMenuItem)

	editMenuItem := appkit.NewMenuItemWithTitleActionKeyEquivalent("Edit", 0, "")
	editMenu := appkit.NewNSMenu()
	editMenu.SetTitle("Edit")
	editMenu.AddItemWithTitleActionKeyEquivalent("Undo", objc.Sel("undo:"), "z")
	editMenu.AddItemWithTitleActionKeyEquivalent("Redo", objc.Sel("redo:"), "Z")
	editMenu.AddItem(appkit.GetNSMenuItemClass().SeparatorItem())
	editMenu.AddItemWithTitleActionKeyEquivalent("Cut", objc.Sel("cut:"), "x")
	editMenu.AddItemWithTitleActionKeyEquivalent("Copy", objc.Sel("copy:"), "c")
	editMenu.AddItemWithTitleActionKeyEquivalent("Paste", objc.Sel("paste:"), "v")
	editMenu.AddItemWithTitleActionKeyEquivalent("Select All", objc.Sel("selectAll:"), "a")
	editMenuItem.SetSubmenu(editMenu)
	menuBar.AddItem(editMenuItem)

	windowMenuItem := appkit.NewMenuItemWithTitleActionKeyEquivalent("Window", 0, "")
	windowMenu := appkit.NewNSMenu()
	windowMenu.SetTitle("Window")
	windowMenu.AddItemWithTitleActionKeyEquivalent("Close", objc.Sel("performClose:"), "w")
	windowMenu.AddItemWithTitleActionKeyEquivalent("Minimize", objc.Sel("performMiniaturize:"), "m")
	windowMenuItem.SetSubmenu(windowMenu)
	menuBar.AddItem(windowMenuItem)

	return menuBar
}

func runBundledApp() (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	contents := filepath.Dir(filepath.Dir(exe))
	resources := filepath.Join(contents, "Resources", "wanix")
	if _, err := os.Stat(filepath.Join(resources, "wanix.js")); err != nil {
		return false, nil
	}
	return true, runHost([]string{"-assets-dir", resources})
}

func runOpen(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: wanix-macapp run <app.app>")
	}
	return exec.Command("open", args[0]).Run()
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	name := fs.String("name", "WanixApp", "application name")
	out := fs.String("out", "", "output .app path")
	assetsDir := fs.String("assets-dir", "", "Wanix asset directory to copy into Contents/Resources/wanix")
	rcWASM := fs.String("rc-wasm", "", "rc.wasm to copy into Contents/Resources/wanix")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *assetsDir == "" {
		return fmt.Errorf("-assets-dir is required")
	}
	appPath := *out
	if appPath == "" {
		appPath = filepath.Join("dist", *name+".app")
	}
	appPath, err := filepath.Abs(appPath)
	if err != nil {
		return err
	}
	if *rcWASM == "" {
		*rcWASM = defaultRCPath(*assetsDir)
	}
	if *rcWASM == "" {
		return fmt.Errorf("rc.wasm not found; run make -C <wanix>/rc build or pass -rc-wasm")
	}
	return buildBundle(appPath, *name, *assetsDir, *rcWASM)
}

func defaultRCPath(assetsDir string) string {
	candidates := []string{
		filepath.Join(assetsDir, "rc.wasm"),
		filepath.Join(filepath.Dir(assetsDir), "rc", "rc.wasm"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
