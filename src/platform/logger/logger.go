// Package logger menyiapkan slog logger JSON untuk seluruh aplikasi.
package logger

import (
	"io"
	"log/slog"
)

// New membuat logger JSON dengan level tertentu dan menjadikannya default slog.
func New(w io.Writer, level slog.Level, env string) *slog.Logger {
	l := slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})).
		With(slog.String("service", "lovoria"), slog.String("env", env))
	slog.SetDefault(l)
	return l
}
