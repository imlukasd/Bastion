package logx

import (
	"io"
	"log/slog"
)

// New returns a slog logger that writes one JSON object per record to w,
// drops records below level, and adds service to every record (AD-21, E35).
// Callers never log secrets (E36, TD §32).
func New(w io.Writer, level slog.Leveler, service string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(h).With(slog.String("service", service))
}
