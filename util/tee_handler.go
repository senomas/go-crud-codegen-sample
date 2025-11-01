package util

import (
	"context"
	"log/slog"
)

type TeeHandler struct{ hs []slog.Handler }

func NewTeeHandler(hs ...slog.Handler) *TeeHandler { return &TeeHandler{hs: hs} }

func (t *TeeHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	for _, h := range t.hs {
		if h.Enabled(ctx, lvl) {
			return true
		}
	}
	return false
}

func (t *TeeHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range t.hs {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (t *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(t.hs))
	for i, h := range t.hs {
		out[i] = h.WithAttrs(attrs)
	}
	return &TeeHandler{hs: out}
}

func (t *TeeHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(t.hs))
	for i, h := range t.hs {
		out[i] = h.WithGroup(name)
	}
	return &TeeHandler{hs: out}
}
