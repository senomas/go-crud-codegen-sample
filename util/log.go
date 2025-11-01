package util

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
)

var (
	LOG_FILE      = os.Getenv("LOG_FILE")
	CONSOLE_ERROR = os.Getenv("CONSOLE_ERROR")
)

func init() {
	var lh slog.Handler
	var level slog.Level
	var w io.Writer = os.Stdout
	logFileName := LOG_FILE
	if logFileName != "" {
		flog, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			panic(err)
		}
		w = flog
	}
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	switch os.Getenv("LOG_FORMAT") {
	case "pretty":
		lh = NewPrettyHandler(w, &slog.HandlerOptions{
			Level: level,
		})
	case "json":
		lh = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: level,
		})
	default:
		lh = slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: level,
		})
	}

	if logFileName != "" && strings.ToLower(os.Getenv("CONSOLE_ERROR")) != "false" {
		lhm := NewPrettyHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelError,
		})
		lh = NewTeeHandler(lh, lhm)
	}
	slog.SetDefault(slog.New(lh))
}

type StackTraceHandler struct {
	slog.Handler
}

func (h *StackTraceHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract error field if any
	r.Attrs(func(a slog.Attr) bool {
		if _, ok := a.Value.Any().(error); ok {
			stack := string(debug.Stack())
			r.Add("stack", slog.StringValue(stack))
		}
		return true
	})
	return h.Handler.Handle(ctx, r)
}

// PrettyHandler is a custom slog.Handler that outputs colorful, human-readable logs
type PrettyHandler struct {
	opts   slog.HandlerOptions
	output io.Writer
}

// NewPrettyHandler returns a console-friendly slog handler
func NewPrettyHandler(w io.Writer, opts *slog.HandlerOptions) *PrettyHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &PrettyHandler{output: w, opts: *opts}
}

func (h *PrettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	min := slog.LevelInfo
	if h.opts.Level != nil {
		min = h.opts.Level.Level()
	}
	return level >= min
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	var b strings.Builder

	// Timestamp
	b.WriteString(r.Time.Format("15:04:05"))
	b.WriteString(" ")

	// Level color
	levelColor := map[slog.Level]string{
		slog.LevelDebug: "\033[36mDEBUG\033[0m",
		slog.LevelInfo:  "\033[32mINFO\033[0m",
		slog.LevelWarn:  "\033[33mWARN\033[0m",
		slog.LevelError: "\033[31mERROR\033[0m",
	}
	var levelStr string
	if "LOG_FILE" != "" && CONSOLE_ERROR != "true" {
		levelStr = r.Level.String()
	} else {
		levelStr = levelColor[r.Level]
		if levelStr == "" {
			levelStr = r.Level.String()
		}
	}
	b.WriteString(levelStr)
	b.WriteString(" ")

	// Message
	b.WriteString(r.Message)

	// Attributes
	r.Attrs(func(a slog.Attr) bool {
		val := a.Value.String()

		// If the value is an error, add stack trace
		if err, ok := a.Value.Any().(error); ok {
			stack := strings.TrimSpace(string(debug.Stack()))
			val = fmt.Sprintf("%v\n%s", err, indent(stack, "    "))
		}

		b.WriteString(fmt.Sprintf("\n  • %s: %s", a.Key, val))
		return true
	})

	if h.output == nil {
		fmt.Fprintln(os.Stdout, b.String())
	} else {
		fmt.Fprintln(h.output, b.String())
	}
	return nil
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *PrettyHandler) WithGroup(name string) slog.Handler       { return h }

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
