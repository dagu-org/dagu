package tag

import "golang.org/x/exp/slog"

func Error(err error) slog.Attr {
	if err != nil {
		return slog.String("error", err.Error())
	}
	return slog.String("error", "")
}
