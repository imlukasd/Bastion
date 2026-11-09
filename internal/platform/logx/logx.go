package logx

import (
	"io"
	"log/slog"
)

// New returns a JSON slog logger on w at level; every record carries service (AD-21, E35).
func New(w io.Writer, level slog.Leveler, service string) *slog.Logger {
	_, _, _ = w, level, service
	return slog.New(slog.DiscardHandler) // P1-03 G2 stub
}
