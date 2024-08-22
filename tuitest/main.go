package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func NewInstanceCreator(app *tview.Application) *InstanceCreator {
	ic := &InstanceCreator{
		app:   app,
		pages: tview.NewPages(),
		gpuOptions: []string{
			"H100", "A100", "L40S", "A10", "A10G", "L4", "T4",
		},
	}
	ic.gpuSelect = NewGPUSelector(app, ic.gpuOptions)
	return ic
}

type InstanceCreator struct {
	app             *tview.Application
	pages           *tview.Pages
	containerSelect *tview.Form
	gpuSelect       *GPUSelector
	instanceConfig  *tview.Form
	gpuOptions      []string
	selectedGPU     string
}

func easeInOutQuad(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	}
	return -1 + (4-2*t)*t
}

func (ic *InstanceCreator) Run() error {
	ic.setupContainerSelect()
	ic.setupGPUSelect()
	ic.setupInstanceConfig()

	ic.pages.AddPage("container", ic.containerSelect, true, true)
	ic.pages.AddPage("gpu", ic.gpuSelect, true, false)
	ic.pages.AddPage("config", ic.instanceConfig, true, false)

	// ic.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
	// 	// name, _ := ic.pages.GetFrontPage()
	// 	// if ic.pages.HasPage("gpu") && name == "gpu" {
	// 	// 	ic.gpuSelect.InputHandler()(event, func(p tview.Primitive) {
	// 	// 		ic.app.SetFocus(p)
	// 	// 	})
	// 	// 	return nil
	// 	// }
	// 	// return event
	// })

	ic.app.SetRoot(ic.pages, true).EnableMouse(true)
	return ic.app.Run()
}

func (ic *InstanceCreator) setupContainerSelect() {
	ic.containerSelect = tview.NewForm().
		AddButton("Container Mode", func() {
			ic.pages = ic.pages.SwitchToPage("gpu")
		}).
		AddButton("VM Mode", func() {
			// Implement VM mode logic here
		})
	ic.containerSelect.SetBorder(true).SetTitle("Select Container Mode")
}

func (ic *InstanceCreator) setupGPUSelect() {
	ic.gpuSelect = NewGPUSelector(ic.app, ic.gpuOptions)
	ic.gpuSelect.SetSelectedFunc(func(gpu string) {
		ic.selectedGPU = gpu
		ic.updateInstanceOptions()
		ic.pages.SwitchToPage("config")
	})
	ic.gpuSelect.StartAnimation()

	// Handle input directly without recursion
	ic.gpuSelect.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			if ic.gpuSelect.selected > 0 {
				ic.gpuSelect.selected--
			}
		case tcell.KeyRight:
			if ic.gpuSelect.selected < len(ic.gpuSelect.gpus)-1 {
				ic.gpuSelect.selected++
			}
		case tcell.KeyEnter:
			if ic.gpuSelect.selectedFunc != nil {
				ic.gpuSelect.selectedFunc(ic.gpuSelect.gpus[ic.gpuSelect.selected])
			}
		}
		ic.app.Draw()
		return nil
	})
}

func (ic *InstanceCreator) updateInstanceOptions() {
	ic.instanceConfig.Clear(true)
	switch ic.selectedGPU {
	case "A100":
		ic.instanceConfig.AddCheckbox("1x NVIDIA A100 40GB VRAM 200GB RAM x 30 CPUs", false, nil)
		ic.instanceConfig.AddCheckbox("1x NVIDIA A100 40GB VRAM 120GB RAM x 12 CPUs (PCIE)", false, nil)
	case "H100":
		ic.instanceConfig.AddCheckbox("1x NVIDIA H100 80GB VRAM 200GB RAM x 26 CPUs", false, nil)
		ic.instanceConfig.AddCheckbox("2x NVIDIA H100 80GB VRAM 360GB RAM x 60 CPUs (PCIE)", false, nil)
	}
	ic.instanceConfig.AddButton("Deploy", ic.deployInstance)
}

func (ic *InstanceCreator) setupInstanceConfig() {
	ic.instanceConfig = tview.NewForm()
	ic.instanceConfig.SetBorder(true).SetTitle("Configure Instance")
}

func (ic *InstanceCreator) deployInstance() {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Deploying instance with %s GPU...", ic.selectedGPU)).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ic.pages.SwitchToPage("container")
		})
	ic.pages.AddPage("deploy", modal, false, true)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	app := tview.NewApplication()
	ic := NewInstanceCreator(app)
	// Set up panic handler
	defer func() {
		if r := recover(); r != nil {
			app.Stop()
			fmt.Fprintf(os.Stderr, "Panic: %v\n", r)
			fmt.Fprintf(os.Stderr, "Stack trace:\n%s", debug.Stack())
			os.Exit(1)
		}
	}()
	// Set up signal handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigChan
		app.Stop()
		fmt.Fprintln(os.Stderr, "\nReceived shutdown signal. Exiting...")
		os.Exit(0)
	}()
	// Run the application
	return ic.Run()
}
