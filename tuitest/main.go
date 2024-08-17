package main

import (
	"fmt"
	"math"
	"os"
	"time"

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

func NewGPUSelector(app *tview.Application, gpus []string) *GPUSelector {
	return &GPUSelector{
		Box:        tview.NewBox(),
		app:        app,
		gpus:       gpus,
		lastUpdate: time.Now(),
	}
}

type GPUSelector struct {
	*tview.Box
	app            *tview.Application
	gpus           []string
	selected       int
	selectedFunc   func(string)
	animationPos   float64
	lastUpdate     time.Time
	pulseAnimation float64
}

func (g *GPUSelector) Draw(screen tcell.Screen) {
	g.Box.DrawForSubclass(screen, g)
	x, y, width, _ := g.GetInnerRect()

	squareSize := 9
	spacing := 3
	totalWidth := len(g.gpus)*(squareSize+spacing) - spacing

	startX := x + (width-totalWidth)/2

	for i, gpu := range g.gpus {
		sqX := startX + i*(squareSize+spacing)
		borderColor := tcell.ColorWhite

		if i == g.selected {
			borderColor = tcell.ColorGreen
			drawAnimatedSquare(screen, sqX, y, squareSize, gpu, borderColor, g.animationPos, g.pulseAnimation)
		} else {
			drawSquare(screen, sqX, y, squareSize, gpu, borderColor)
		}
	}

	// Update animation position using a smoother easing function
	g.animationPos = easeInOutQuad(g.animationPos)
	// g.pulseAnimation = math.Sin(g.animationPos * math.Pi * 2)
}

func easeInOutQuad(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	}
	return -1 + (4-2*t)*t
}

func drawAnimatedSquare(screen tcell.Screen, x, y, size int, label string, color tcell.Color, progress, pulse float64) {
	drawSquare(screen, x, y, size, label, color)

	animationLength := size*4 - 4
	pos := int(progress * float64(animationLength))

	glowColor := tcell.NewRGBColor(255, 255, 0) // Yellow glow

	for i := 0; i < animationLength; i++ {
		intensity := 1.0 - math.Abs(float64(i-pos)/float64(animationLength))
		intensity = math.Max(intensity, 0)
		intensity *= 0.5 + 0.5*pulse // Add pulsing effect

		r, g, b := glowColor.RGB()
		currentColor := tcell.NewRGBColor(
			int32(float64(r)*intensity),
			int32(float64(g)*intensity),
			int32(float64(b)*intensity),
		)

		if i < size-1 {
			screen.SetContent(x+i, y, tview.BoxDrawingsLightHorizontal, nil, tcell.StyleDefault.Foreground(currentColor))
		} else if i < size*2-2 {
			screen.SetContent(x+size-1, y+i-(size-1), tview.BoxDrawingsLightVertical, nil, tcell.StyleDefault.Foreground(currentColor))
		} else if i < size*3-3 {
			screen.SetContent(x+size-1-(i-(size*2-2)), y+size-1, tview.BoxDrawingsLightHorizontal, nil, tcell.StyleDefault.Foreground(currentColor))
		} else {
			screen.SetContent(x, y+size-1-(i-(size*3-3)), tview.BoxDrawingsLightVertical, nil, tcell.StyleDefault.Foreground(currentColor))
		}
	}
}

func (g *GPUSelector) SetSelectedFunc(f func(string)) {
	g.selectedFunc = f
}

func (g *GPUSelector) StartAnimation() {
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				g.app.QueueUpdateDraw(func() {
					g.animationPos += 0.02
					if g.animationPos >= 1 {
						g.animationPos = 0
					}
				})
			}
		}
	}()
}

func drawSquare(screen tcell.Screen, x, y, size int, label string, color tcell.Color) {
	for i := 0; i < size; i++ {
		screen.SetContent(x+i, y, tview.BoxDrawingsLightHorizontal, nil, tcell.StyleDefault.Foreground(color))
		screen.SetContent(x+i, y+size-1, tview.BoxDrawingsLightHorizontal, nil, tcell.StyleDefault.Foreground(color))
		screen.SetContent(x, y+i, tview.BoxDrawingsLightVertical, nil, tcell.StyleDefault.Foreground(color))
		screen.SetContent(x+size-1, y+i, tview.BoxDrawingsLightVertical, nil, tcell.StyleDefault.Foreground(color))
	}
	screen.SetContent(x, y, tview.BoxDrawingsLightDownAndRight, nil, tcell.StyleDefault.Foreground(color))
	screen.SetContent(x+size-1, y, tview.BoxDrawingsLightDownAndLeft, nil, tcell.StyleDefault.Foreground(color))
	screen.SetContent(x, y+size-1, tview.BoxDrawingsLightUpAndRight, nil, tcell.StyleDefault.Foreground(color))
	screen.SetContent(x+size-1, y+size-1, tview.BoxDrawingsLightUpAndLeft, nil, tcell.StyleDefault.Foreground(color))

	labelX := x + (size-len(label))/2
	labelY := y + size/2
	tview.Print(screen, label, labelX, labelY, len(label), tview.AlignCenter, color)
}

func (ic *InstanceCreator) Run() error {
	ic.setupContainerSelect()
	ic.setupGPUSelect()
	ic.setupInstanceConfig()

	ic.pages.AddPage("container", ic.containerSelect, true, true)
	ic.pages.AddPage("gpu", ic.gpuSelect, true, false)
	ic.pages.AddPage("config", ic.instanceConfig, true, false)

	ic.pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ic.pages.HasPage("gpu") {
			ic.gpuSelect.InputHandler()(event, func(p tview.Primitive) {
				ic.app.SetFocus(p)
			})
		}
		return event
	})

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
	ic.gpuSelect.SetBorder(true).SetTitle("Select GPU")
	ic.gpuSelect.SetSelectedFunc(func(gpu string) {
		ic.selectedGPU = gpu
		ic.updateInstanceOptions()
		ic.pages.SwitchToPage("config")
	})
	ic.gpuSelect.StartAnimation()
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
	app := tview.NewApplication()
	ic := NewInstanceCreator(app)
	if err := ic.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
