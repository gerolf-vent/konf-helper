package internal

import "go.uber.org/zap/zapcore"

type Notifyer interface {
	zapcore.ObjectMarshaler
	Notify() bool
}
