package tui

import (
	"fmt"
	"log"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/htr/eslogview"
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
	mainWindow := t.newMainWindow(queryString)

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
				// currentWindow = t.newContextWindow(currentWindow.selectedEntry())
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

func (t *TUI) newMainWindow(queryString string) *logentriesWindow {
	entries, err := t.esCtx.Search(queryString)
	if err != nil {
		log.Fatalf("unable to fetch log entries: %v\n", err)
	}

	win := &logentriesWindow{}
	win.logentries = entries
	win.createWidgets()

	return win
}

type logentriesWindow struct {
	containerWidget *ui.Grid
	listWidget      *widgets.List
	logentries      []eslogview.LogEntry
}

func (evWin *logentriesWindow) selectedEntry() eslogview.LogEntry {
	return evWin.logentries[evWin.listWidget.SelectedRow]
}

func (evWin *logentriesWindow) ScrollAmount(amount int) {
	evWin.listWidget.ScrollAmount(amount)
}

func (evWin *logentriesWindow) createWidgets() {
	list := widgets.NewList()
	list.TextStyle = ui.NewStyle(ui.ColorClear, ui.ColorClear)
	list.Border = false
	list.SelectedRowStyle = ui.NewStyle(ui.ColorClear, ui.ColorClear, ui.ModifierReverse)

	rows := []string{}
	for _, ev := range evWin.logentries {
		fmt.Println(ev.Timestamp.Format(time.RFC3339), ev.Message)
		rows = append(rows, fmt.Sprintf("%s  %s", ev.Timestamp.Format(time.RFC3339), ev.Message))
	}

	list.Rows = rows

	grid := ui.NewGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	grid.SetRect(0, 0, termWidth, termHeight)
	grid.Set(ui.NewRow(1.0, list))

	evWin.listWidget = list
	evWin.containerWidget = grid
}
