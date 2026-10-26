package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Level is a log verbosity. Each level includes the ones before it.
type Level int

const (
	LevelNone Level = iota
	LevelErrors
	LevelWarnings
	LevelInfo
	LevelTrace
)

var levelNames = map[string]Level{
	"none":     LevelNone,
	"errors":   LevelErrors,
	"warnings": LevelWarnings,
	"info":     LevelInfo,
	"trace":    LevelTrace,
}

// ParseLevel parses a level name as used in configurations.
func ParseLevel(s string) (Level, error) {
	if l, ok := levelNames[s]; ok {
		return l, nil
	}
	return 0, fmt.Errorf("unknown log level %q (none, errors, warnings, info, trace)", s)
}

// Event is one log entry. Line and Column locate it in the input; zero
// values mean "not tied to a line / column".
type Event struct {
	Line    int
	Column  string
	Message string
}

// Logger writes events as text lines or JSON lines.
type Logger struct {
	w     io.Writer
	level Level
	json  bool
	now   func() time.Time
}

// NewLogger returns a logger writing to w. format is "text" or "json". now
// stamps each line; nil writes no timestamps (used by tests).
func NewLogger(w io.Writer, level Level, format string, now func() time.Time) *Logger {
	return &Logger{w: w, level: level, json: format == "json", now: now}
}

// Discard is a logger that writes nothing.
var Discard = NewLogger(io.Discard, LevelNone, "text", nil)

// Enabled reports whether events at level are written. Check it before
// building expensive messages.
func (l *Logger) Enabled(level Level) bool { return level <= l.level }

func (l *Logger) Error(e Event) { l.log(LevelErrors, "ERROR", e) }
func (l *Logger) Warn(e Event)  { l.log(LevelWarnings, "WARN", e) }
func (l *Logger) Info(e Event)  { l.log(LevelInfo, "INFO", e) }
func (l *Logger) Trace(e Event) { l.log(LevelTrace, "TRACE", e) }

func (l *Logger) log(level Level, name string, e Event) {
	if !l.Enabled(level) {
		return
	}
	if l.json {
		entry := struct {
			Time    string `json:"time,omitempty"`
			Level   string `json:"level"`
			Line    int    `json:"line,omitempty"`
			Column  string `json:"column,omitempty"`
			Message string `json:"message"`
		}{Level: strings.ToLower(name), Line: e.Line, Column: e.Column, Message: e.Message}
		if l.now != nil {
			entry.Time = l.now().UTC().Format(time.RFC3339)
		}
		b, _ := json.Marshal(entry)
		l.w.Write(append(b, '\n'))
		return
	}

	var b strings.Builder
	if l.now != nil {
		b.WriteString(l.now().UTC().Format(time.RFC3339))
		b.WriteByte(' ')
	}
	fmt.Fprintf(&b, "%-5s ", name)
	switch {
	case e.Line > 0 && e.Column != "":
		fmt.Fprintf(&b, "line %d, column %s: ", e.Line, e.Column)
	case e.Line > 0:
		fmt.Fprintf(&b, "line %d: ", e.Line)
	case e.Column != "":
		fmt.Fprintf(&b, "column %s: ", e.Column)
	}
	// Continuation lines are indented under the message.
	b.WriteString(strings.ReplaceAll(e.Message, "\n", "\n      "))
	b.WriteByte('\n')
	io.WriteString(l.w, b.String())
}

// Print writes free text (the summary) regardless of level.
func (l *Logger) Print(text string) {
	if l.json {
		return // JSON consumers read the report file instead
	}
	io.WriteString(l.w, text)
}
