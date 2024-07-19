package logger

type (
	Logger interface {
		Debug(msg string, tags ...any)
		Info(msg string, tags ...any)
		Warn(msg string, tags ...any)
		Error(msg string, tags ...any)
	}
)
