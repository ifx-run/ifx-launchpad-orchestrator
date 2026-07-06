package logx

import (
	"fmt"
	"log"
	"time"
)

var debugEnabled bool

// Init configures global logging behavior from server config.
func Init(debug bool) {
	debugEnabled = debug
}

func Info(component, msg string, kv ...any) {
	log.Printf(formatLine(component, msg, kv...))
}

func Warn(component, msg string, kv ...any) {
	log.Printf("WARN "+formatLine(component, msg, kv...))
}

func Error(component, msg string, kv ...any) {
	log.Printf("ERROR "+formatLine(component, msg, kv...))
}

func Debug(component, msg string, kv ...any) {
	if !debugEnabled {
		return
	}
	log.Printf("DEBUG "+formatLine(component, msg, kv...))
}

// Since returns elapsed milliseconds since start.
func Since(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func formatLine(component, msg string, kv ...any) string {
	line := "[" + component + "] " + msg
	for i := 0; i+1 < len(kv); i += 2 {
		line += " " + stringify(kv[i]) + "=" + stringify(kv[i+1])
	}
	return line
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		if x == nil {
			return "<nil>"
		}
		return x.Error()
	default:
		return fmt.Sprintf("%v", v)
	}
}
