package eslogview

import (
	"time"
)

type LogEntry struct {
	ID        string
	Context   map[string]interface{}
	Message   string
	Timestamp time.Time
}

// required to use sort.*
type LogEntries []LogEntry

func (s LogEntries) Len() int {
	return len(s)
}
func (s LogEntries) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s LogEntries) Less(i, j int) bool {
	return s[i].Timestamp.Before(s[j].Timestamp)
}
