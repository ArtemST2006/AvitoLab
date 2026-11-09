// Package sl - костыль для красивого вывода ошибок в логгере
package sl

import "log/slog"

// Err создаёт и возвращает slog.Attr с ошибкой
func Err(err error) slog.Attr {
	return slog.Attr{
		Key:   "error",
		Value: slog.StringValue(err.Error()),
	}
}
