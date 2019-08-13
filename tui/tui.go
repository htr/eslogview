package tui

import (
	"log"

	ui "github.com/gizak/termui/v3"
	"github.com/htr/eslogview/elasticsearch"
)

type TUI struct {
	esCtx *elasticsearch.Context
}

func New(esCtx *elasticsearch.Context) *TUI {
	return &TUI{
		esCtx: esCtx,
	}
}

func (t *TUI) Run(queryString string) error {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	// create mainWindow
	var mainWindow = nil

	currentWindow := mainWindow

	// eventLoop

	ui.Render(currentWindow.containerWidget)

	uiEvents := ui.PollEvents()
	for {
		e := <-uiEvents
		switch e.ID {
		case "q":
			if currentWindow == mainWindow {
				return nil
			} else {
				currentWindow = mainWindow
			}
		case "<Enter>":
			if currentWindow == mainWindow {
				currentWindow = t.newContextWindow(currentWindow.selectedEntry())
			}
		case "j", "<Down>":
			currentWindow.ScrollAmount(1)
		case "k", "<Up>":
			currentWindow.ScrollAmount(-1)
		case "<Resize>":
			payload := e.Payload.(ui.Resize)
			if currentWindow != mainWindow {
				mainWindow.containerWidget.SetRect(0, 0, payload.Width, payload.Height)
			}
			currentWindow.containerWidget.SetRect(0, 0, payload.Width, payload.Height)
			ui.Clear()
			ui.Render(currentWindow.containerWidget)
		}
		ui.Render(currentWindow.containerWidget)
	}

	return nil
}

type eventsWindow struct {
}
