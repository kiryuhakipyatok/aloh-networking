package logger

type SparseLogger struct {
	*Logger
	count  uint
	target uint
}

func (l *Logger) Sparse(count, target uint) *SparseLogger {
	return &SparseLogger{
		Logger: l,
		count:  count,
		target: target,
	}
}

func (sl *SparseLogger) Info(msg string, args ...any) {
	if sl.count%sl.target == 0 || sl.count == 1 {
		sl.Logger.Info(msg, args...)
	}
}

func (sl *SparseLogger) Error(msg string, args ...any) {
	if sl.count%sl.target == 0 || sl.count == 1 {
		sl.Logger.Error(msg, args...)
	}
}

func (sl *SparseLogger) Debug(msg string, args ...any) {
	if sl.count%sl.target == 0 || sl.count == 1 {
		sl.Logger.Debug(msg, args...)
	}
}
