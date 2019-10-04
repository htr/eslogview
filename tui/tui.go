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

type FetchDirection int

const (
	pageSize = 200

	FetchPrevious FetchDirection = 1
	FetchNext     FetchDirection = 2
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
	mainWindow, err := t.newMainWindow(queryString)
	if err != nil {
		ui.Close()
		log.Fatalln(err)
	}

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
		case "J", "<C-d>":
			currentWindow.ScrollAmount(currentWindow.containerWidget.Rectangle.Dy() / 2)
		case "K", "<C-u>":
			currentWindow.ScrollAmount(-currentWindow.containerWidget.Rectangle.Dy() / 2)
		case "<C-f>", "<PageDown>":
			currentWindow.ScrollAmount(currentWindow.containerWidget.Rectangle.Dy())
		case "<C-b>", "<PageUp>":
			currentWindow.ScrollAmount(-currentWindow.containerWidget.Rectangle.Dy())

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

func (t *TUI) newMainWindow(queryString string) (*logentriesWindow, error) {
	entries, err := t.esCtx.Search(queryString, "", "")
	if err != nil {
		return nil, fmt.Errorf("Unable to fetch log entries: %v.\n", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("Unable to find any results.")
	}

	win := &logentriesWindow{}
	win.logentries = entries
	win.createWidgets()

	return win, nil
}

func (t *TUI) newContextWindow(firstEntry eslogview.LogEntry) *logentriesWindow {
	entries, err := t.esCtx.LogEntryContext(firstEntry, pageSize)
	if err != nil {
		log.Fatalf("unable to fetch log entries: %v\n", err)
	}

	win := &logentriesWindow{}
	win.logentries = entries
	win.createWidgets()
	win.fetchMore = func(baseEntry eslogview.LogEntry, direction FetchDirection) []eslogview.LogEntry {
		fetchAmount := pageSize
		if direction == FetchPrevious {
			fetchAmount = -pageSize
		}

		// I probably shouldn't ignore this error
		entries, _ := t.esCtx.LogEntryContext(baseEntry, fetchAmount)
		if len(entries) == 0 {
			return entries
		}
		if direction == FetchPrevious {
			return entries[0 : len(entries)-1]
		} else {
			return entries[1:]
		}
	}

	return win
}

type logentriesWindow struct {
	containerWidget *ui.Grid
	listWidget      *widgets.List
	logentries      []eslogview.LogEntry
	fetchMore       func(eslogview.LogEntry, FetchDirection) []eslogview.LogEntry
}

func (evWin *logentriesWindow) selectedEntry() eslogview.LogEntry {
	return evWin.logentries[evWin.listWidget.SelectedRow]
}

func (evWin *logentriesWindow) ScrollAmount(amount int) {
	if evWin.fetchMore != nil &&
		(evWin.listWidget.SelectedRow+amount > (len(evWin.logentries)-1) ||
			evWin.listWidget.SelectedRow+amount < 0) {
		selectedRow := evWin.listWidget.SelectedRow
		if amount < 0 {
			moreEntries := evWin.fetchMore(evWin.logentries[0], FetchPrevious)
			evWin.logentries = append(moreEntries, evWin.logentries...)
			selectedRow = selectedRow + len(moreEntries)
		} else if amount > 0 {
			moreEntries := evWin.fetchMore(evWin.logentries[len(evWin.logentries)-1], FetchNext)
			evWin.logentries = append(evWin.logentries, moreEntries...)
			selectedRow = selectedRow - len(moreEntries)
		}

		// I should be able to just reset the list' Rows, but lets do it quick and dirty for now
		evWin.createWidgets()
		evWin.listWidget.SelectedRow = selectedRow
	}
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
