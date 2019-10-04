package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/htr/eslogview"
	"github.com/htr/eslogview/elasticsearch"
	"github.com/htr/eslogview/tui"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	app            = kingpin.New("eslogview", "command line frontend to logs stored in elasticsearch")
	verboseLogging = app.Flag("verbose", "Increase logging verbosity").Short('v').Bool()
	confFile       = app.Flag("config", "configuration file path").Default(os.Getenv("HOME") + "/.eslogview.yaml").Short('c').File()

	search            = app.Command("search", "Search for occurrences of a given query string")
	searchQueryString = search.Arg("query-string", "query string/elasticsearch simple query").Required().String()
	searchBefore      = search.Flag("before", "timestamp upper bound").String()
	searchAfter       = search.Flag("after", "timestamp lower bound").String()

	showContext           = app.Command("show-context", "Show context of a given log entry")
	showContextAfter      = showContext.Flag("after-context", "Number of loglines to print after the selected logline").Short('A').Default("200").Int()
	showContextBefore     = showContext.Flag("before-context", "Number of loglines to print before the selected logline").Short('B').Default("200").Int()
	showContextLogEntryID = showContext.Arg("id", "logentry id").Required().String()

	tuiCmd         = app.Command("tui", "Experimental text user interface")
	tuiQueryString = tuiCmd.Arg("query-string", "query string/elasticsearch simple query").Required().String()
)

func main() {
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	config := eslogview.MustLoadConfig(*confFile)
	//fmt.Printf("%+v\n", config)

	switch cmd {
	case search.FullCommand():
		esCtx, err := elasticsearch.NewContext(config)
		panicIf(err)
		logEntries, err := esCtx.Search(*searchQueryString, *searchBefore, *searchAfter)
		panicIf(err)

		w := &tabwriter.Writer{}
		w.Init(os.Stdout, 0, 8, 0, ' ', 0)
		for _, logEntry := range logEntries {
			args := []interface{}{logEntry.ID, "\t", logEntry.Timestamp.Format(time.RFC3339Nano), "\t"}
			for _, contextField := range config.ContextFields {
				args = append(args, logEntry.Context[contextField].(string))
				args = append(args, "\t")
			}
			args = append(args, logEntry.Message)

			fmt.Fprintln(w, args...)
		}
		w.Flush()

	case showContext.FullCommand():
		esCtx, err := elasticsearch.NewContext(config)
		panicIf(err)
		ev, err := esCtx.LogEntryByID(*showContextLogEntryID)

		logLinesBefore, err := esCtx.LogEntryContext(ev, -*showContextBefore)
		panicIf(err)
		logLinesAfter, err := esCtx.LogEntryContext(ev, *showContextAfter)
		panicIf(err)

		allLogLines := append(logLinesBefore, logLinesAfter[1:]...)
		for _, logLine := range allLogLines {
			fmt.Printf("%-30s %s\n", logLine.Timestamp.Format(time.RFC3339Nano), logLine.Message)
		}

	case tuiCmd.FullCommand():
		esCtx, err := elasticsearch.NewContext(config)
		panicIf(err)

		t := tui.New(esCtx)

		t.Run(*tuiQueryString)
	}
}

func panicIf(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
