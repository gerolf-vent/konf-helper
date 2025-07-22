package internal

import "go.uber.org/zap/zapcore"

type Notifier interface {
	zapcore.ObjectMarshaler
	Notify() bool
}
