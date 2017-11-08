package elasticsearch

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/htr/eslogview"
	elastic "gopkg.in/olivere/elastic.v3"
)

const logEntriesReqSize = 300
const logEntriesCtxReqSize = 500

type Context struct {
	client *elastic.Client
	config eslogview.Config
}

func NewContext(config eslogview.Config) (*Context, error) {
	esclient, err := elastic.NewSimpleClient(elastic.SetURL("http://kibana.internal.digitalocean.com:9200"))
	if err != nil {
		return nil, err
	}
	return &Context{client: esclient, config: config}, nil
}

func (esCtx *Context) Search(searchString string) ([]eslogview.LogEntry, error) {
	logEntries := []eslogview.LogEntry{}

	query := elastic.NewBoolQuery().Must(elastic.NewQueryStringQuery(searchString).AnalyzeWildcard(true))

	searchResult, err := esCtx.client.Search().
		Index(esCtx.config.Index).
		Query(query).
		From(0).Size(logEntriesReqSize).
		SortWithInfo(elastic.SortInfo{Field: "@timestamp", Ascending: false}).
		Do()

	if err != nil {
		return logEntries, nil
	}

	return esCtx.logEntriesFromResult(searchResult), nil
}

func (esCtx *Context) LogEntryByID(ID string) (eslogview.LogEntry, error) {

	searchResult, err := esCtx.client.Search().
		Index(esCtx.config.Index).
		Query(elastic.NewBoolQuery().Must(elastic.NewTermQuery("_id", ID))).
		Do()

	if err != nil {
		return eslogview.LogEntry{}, err
	}

	evs := esCtx.logEntriesFromResult(searchResult)
	if len(evs) < 1 || err != nil {
		return eslogview.LogEntry{}, fmt.Errorf("unable to find document with id '%s'", ID)
	}

	return esCtx.logEntriesFromResult(searchResult)[0], nil
}

// this lookback defines whether we want to load context where timestamp < ev.timestamp
func (esCtx *Context) LogEntryContext(ev eslogview.LogEntry, timelineLength int) ([]eslogview.LogEntry, error) {
	//by using ev's program and host instead of esCtx's, it is able to retrieve context from different ogs/ports

	queries := []elastic.Query{}
	for _, ctxVar := range esCtx.config.ContextFields {
		queries = append(queries, elastic.NewTermQuery(ctxVar+".raw", ev.Context[ctxVar]))
	}

	query := elastic.NewBoolQuery().Must(queries...)

	sortInfo := elastic.SortInfo{Field: esCtx.config.TimestampField}

	size := 0

	if timelineLength > 0 { // ascending order, starting from ev's ts
		sortInfo.Ascending = true
		query = query.Must(elastic.NewRangeQuery(esCtx.config.TimestampField).From(ev.Timestamp).To("now"))
		size = timelineLength
	} else {
		sortInfo.Ascending = false
		query = query.Must(elastic.NewRangeQuery(esCtx.config.TimestampField).To(ev.Timestamp))
		size = -timelineLength
	}

	searchResult, err := esCtx.client.Search().
		Index(esCtx.config.Index).
		Query(query).
		Size(size).
		SortWithInfo(sortInfo).
		Do()

	if err != nil {
		return []eslogview.LogEntry{}, err
	}

	return esCtx.logEntriesFromResult(searchResult), nil
}

func (esCtx *Context) logEntriesFromResult(searchResult *elastic.SearchResult) []eslogview.LogEntry {
	logEntries := eslogview.LogEntries{}
	re := regexp.MustCompile(esCtx.config.MessageCleanupRegex)

	for _, hit := range searchResult.Hits.Hits {
		source := map[string]interface{}{}
		json.Unmarshal(*hit.Source, &source)
		ev := eslogview.LogEntry{}
		ev.Message = re.ReplaceAllLiteralString(strings.TrimSpace(source[esCtx.config.MessageField].(string)), "")

		switch v := source[esCtx.config.TimestampField].(type) {
		case string:
			ev.Timestamp.UnmarshalText([]byte(v))
		default:
			panic(fmt.Sprintf("don't know how to handle timestamps: %+v ", v))
		}

		ev.ID = hit.Id

		ev.Context = map[string]interface{}{}
		for _, ctxField := range esCtx.config.ContextFields {
			ev.Context[ctxField] = source[ctxField]
		}

		if esCtx.config.IgnoreBlankLogLines && len(strings.TrimSpace(ev.Message)) == 0 {
			continue
		}

		logEntries = append(logEntries, ev)
	}

	sort.Sort(logEntries)
	return logEntries
}
