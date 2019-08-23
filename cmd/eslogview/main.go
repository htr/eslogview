package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/htr/eslogview"
	"github.com/htr/eslogview/elasticsearch"
	"github.com/htr/eslogview/tui"
	homedir "github.com/mitchellh/go-homedir"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	app            = kingpin.New("eslogview", "command line frontend to logs stored in elasticsearch")
	verboseLogging = app.Flag("verbose", "Increase logging verbosity").Short('v').Bool()
	confFile       = app.Flag("config", "configuration file path").Default(os.Getenv("HOME") + "/.eslogview.yaml").Short('c').File()

	search            = app.Command("search", "Search for occurrences of a given query string")
	searchQueryString = search.Arg("query-string", "query string/elasticsearch simple query").Required().String()

	showContext           = app.Command("show-context", "Show context of a given log entry")
	showContextAfter      = showContext.Flag("after-context", "Number of loglines to print after the selected logline").Short('A').Default("200").Int()
	showContextBefore     = showContext.Flag("before-context", "Number of loglines to print before the selected logline").Short('B').Default("200").Int()
	showContextLogEntryID = showContext.Arg("id", "logentry id").Required().String()

	tuiCmd         = app.Command("tui", "Experimental text user interface")
	tuiQueryString = tuiCmd.Arg("query-string", "query string/elasticsearch simple query").Required().String()
)

type CsvIndex struct {
	cleanupFunc func(string) string
	index       map[string]string
}

func newCsvIndex(path string) (*CsvIndex, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	idx := &CsvIndex{
		cleanupFunc: func(s string) string {
			return s
		},
		index: map[string]string{},
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		splitted := strings.SplitN(line, ",", 2)
		idx.index[splitted[1]] = splitted[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return idx, nil
}

func (idx *CsvIndex) lookup(s string) string {
	if idx == nil {
		return ""
	}

	if val, ok := idx.index[(idx.cleanupFunc)(s)]; ok {
		return val
	}
	return ""
}

func main() {
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	config := eslogview.MustLoadConfig(*confFile)

	var friendlyNamesIdx *CsvIndex

	if config.FriendlyNames.Csv != "" {
		if path, err := homedir.Expand(config.FriendlyNames.Csv); err == nil {
			friendlyNamesIdx, _ = newCsvIndex(path)
			if friendlyNamesIdx != nil && config.FriendlyNames.ContextCleanupRegex != "" {
				re := regexp.MustCompile(config.FriendlyNames.ContextCleanupRegex)
				friendlyNamesIdx.cleanupFunc = func(i string) string {
					return re.ReplaceAllLiteralString(strings.TrimSpace(i), "")
				}
			}
		} else {
			log.Panicln(err)
		}
	}

	switch cmd {
	case search.FullCommand():
		esCtx, err := elasticsearch.NewContext(config)
		panicIf(err)
		logEntries, err := esCtx.Search(*searchQueryString)
		panicIf(err)

		w := &tabwriter.Writer{}
		w.Init(os.Stdout, 0, 8, 0, ' ', 0)
		for _, logEntry := range logEntries {
			args := []interface{}{logEntry.ID, "\t", logEntry.Timestamp.Format(time.RFC3339Nano), "\t"}
			ctxFields := []string{}
			for _, contextField := range config.ContextFields {
				ctxFields = append(ctxFields, logEntry.Context[contextField].(string))
			}
			friendlyName := friendlyNamesIdx.lookup(strings.Join(ctxFields, ":"))
			if friendlyName != "" {
				args = append(args, friendlyName, " \t")
			} else {
				args = append(args, strings.Join(ctxFields, " ")+"\t")
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
