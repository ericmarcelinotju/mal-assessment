package logger

import (
	"context"
	"log/slog"
	"os"
)

type Environment string

const (
	ENV_LOCAL Environment = "local"
	ENV_PROD  Environment = "production"
)

type ContextHandler struct {
	handler slog.Handler
}

func NewContextHandler(env Environment) *ContextHandler {
	var base slog.Handler
	if env == ENV_LOCAL {
		base = slog.NewTextHandler(os.Stdout, nil)
	} else {
		base = slog.NewJSONHandler(os.Stdout, nil)
	}
	return &ContextHandler{handler: base}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	return h.handler.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{handler: h.handler.WithGroup(name)}
}
