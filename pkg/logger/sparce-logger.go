package logger

type SparseLogger struct {
	*Logger
	target uint
}

func (l *Logger) Sparse(target uint) *SparseLogger {
	return &SparseLogger{
		Logger: l,
		target: target,
	}
}

func (sl *SparseLogger) Info(count uint, msg string, args ...any) {
	if count%sl.target == 0 || count == 1 {
		sl.Logger.Info(msg, args...)
	}
}

func (sl *SparseLogger) Error(count uint,msg string, args ...any) {
	if count%sl.target == 0 || count == 1 {
		sl.Logger.Error(msg, args...)
	}
}

func (sl *SparseLogger) Debug(count uint,msg string, args ...any) {
	if count%sl.target == 0 || count == 1 {
		sl.Logger.Debug(msg, args...)
	}
}
