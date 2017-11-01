package tui

import (
	"fmt"
	"log"
	"time"

	"github.com/htr/eslogview"
	"github.com/htr/eslogview/elasticsearch"
	"github.com/jroimartin/gocui"
)

func New(esCtx *elasticsearch.Context) *TUI {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		panic(err)
	}

	return &TUI{
		logEntries:        []eslogview.LogEntry{},
		logEntriesContext: []eslogview.LogEntry{},
		esCtx:             esCtx,
		g:                 g,
	}
}

func (t *TUI) Run(queryString string) error {
	t.SearchString = queryString

	defer t.g.Close()

	t.g.Cursor = true

	t.g.SetManagerFunc(t.layout)

	if err := t.keybindings(t.g); err != nil {
		log.Panicln(err)
	}

	t.g.Execute(func(*gocui.Gui) error {
		return t.showLogEntries(t.g, nil)
	})

	if err := t.g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
	return nil
}

type TUI struct {
	logEntries        []eslogview.LogEntry
	logEntriesContext []eslogview.LogEntry
	SearchString      string

	esCtx *elasticsearch.Context

	g *gocui.Gui
}

const cursorStep = 10

func (t *TUI) redrawCtx(g *gocui.Gui) error {
	if v, err := g.View("log_entry_context"); err == nil {
		v.Clear()
		for _, ev := range t.logEntriesContext {
			fmt.Fprintln(v, ev.Timestamp.Format(time.RFC3339), ev.Message)
		}
	}
	return nil
}

func (t *TUI) requestMoreContext(g *gocui.Gui, line int) {
	if v, err := g.View("log_entry_context"); err == nil {
		_, oy := v.Origin()
		newY := oy
		if len(t.logEntriesContext) == 0 {
			return
		}

		if line < 0 {
			evs, err := t.esCtx.LogEntryContext(t.logEntriesContext[0], -500)
			if err != nil {
				return
			}
			newY += len(evs) - cursorStep
			t.logEntriesContext = append(evs, t.logEntriesContext[1:]...)
		} else {
			evs, err := t.esCtx.LogEntryContext(t.logEntriesContext[len(t.logEntriesContext)-1], 500)
			if err != nil {
				return
			}
			t.logEntriesContext = append(t.logEntriesContext, evs[1:]...)
		}
		t.redrawCtx(g)
		v.SetOrigin(0, newY)
	}
}

func (t *TUI) mkMoveCursorFn(dy int, containerLen func() int, needMoreContentCallback func(g *gocui.Gui, line int)) func(g *gocui.Gui, v *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		if v != nil {
			cx, cy := v.Cursor()
			ox, oy := v.Origin()
			currentLine := cy + oy
			// fmt.Fprintln(v, currentLine, currentLine+dy)
			if containerLen != nil {
				if currentLine+dy >= containerLen() {
					if needMoreContentCallback != nil {
						// fmt.Fprintln(v, "getting more lines")
						needMoreContentCallback(g, currentLine+dy)
					}
					return nil
				}
			}
			if currentLine+dy < 0 && needMoreContentCallback != nil {
				// fmt.Fprintln(v, "getting more lines")
				needMoreContentCallback(g, currentLine+dy)
				return nil
			}
			if err := v.SetCursor(cx, cy+dy); err != nil && oy+dy >= 0 {
				if err := v.SetOrigin(ox, oy+dy); err != nil {
					return err
				}
			}
		}
		return nil

	}
}

func (t *TUI) getLogEntryContext(g *gocui.Gui, v *gocui.View) error {
	_, cy := v.Cursor()
	if cy < len(t.logEntries) {
		if v, err := g.View("log_entry_context"); err == nil {
			v.Clear()

			evs, err := t.esCtx.LogEntryContext(t.logEntries[cy], 500)
			if err != nil {
				return err
			}

			t.logEntriesContext = evs

			t.redrawCtx(g)
			v.SetCursor(0, 0)
			v.SetOrigin(0, 0)
		}
		return t.closeLogEntries(g, v)
	}
	return nil
}

func (t *TUI) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (t *TUI) logEntriesCtxLen() int {
	return len(t.logEntriesContext)
}

func (t *TUI) logEntriesLen() int {
	return len(t.logEntries)
}

func (t *TUI) keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, t.quit); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entry_context", 'q', gocui.ModNone, t.showLogEntries); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entry_context", 'j', gocui.ModNone, t.mkMoveCursorFn(10, t.logEntriesCtxLen, t.requestMoreContext)); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entry_context", 'k', gocui.ModNone, t.mkMoveCursorFn(-10, t.logEntriesCtxLen, t.requestMoreContext)); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entries", 'j', gocui.ModNone, t.mkMoveCursorFn(1, t.logEntriesLen, nil)); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entries", 'k', gocui.ModNone, t.mkMoveCursorFn(-1, t.logEntriesLen, nil)); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entries", 'J', gocui.ModNone, t.mkMoveCursorFn(20, t.logEntriesLen, nil)); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entries", 'K', gocui.ModNone, t.mkMoveCursorFn(-20, t.logEntriesLen, nil)); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entries", 'q', gocui.ModNone, t.quit); err != nil {
		return err
	}

	if err := g.SetKeybinding("log_entries", gocui.KeyEnter, gocui.ModNone, t.getLogEntryContext); err != nil {
		return err
	}

	return nil
}

func (t *TUI) closeLogEntries(g *gocui.Gui, v *gocui.View) error {
	if err := g.DeleteView("log_entries"); err != nil {
		return err
	}
	if _, err := g.SetCurrentView("log_entry_context"); err != nil {
		return err
	}
	return nil
}

func (t *TUI) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("log_entry_context", -1, -1, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		fmt.Fprintln(v, "press /")
		v.Editable = false
		v.Wrap = true
		if _, err := g.SetCurrentView("log_entry_context"); err != nil {
			return err
		}

	}

	return nil
}

func (t *TUI) showLogEntries(g *gocui.Gui, v *gocui.View) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("log_entries", -1, -1, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		v.Autoscroll = false

		evs, err := t.esCtx.Search(t.SearchString)
		if err != nil {
			return err
		}
		t.logEntries = evs

		for _, ev := range evs {
			fmt.Fprintln(v, ev.Timestamp.Format(time.RFC3339), ev.Message)
		}
		fmt.Fprintln(v, " == end of results == ")
		v.SetCursor(0, 0)

		if _, err := g.SetCurrentView("log_entries"); err != nil {
			return err
		}
	}

	return nil
}
