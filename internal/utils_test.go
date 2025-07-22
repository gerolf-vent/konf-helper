package internal

import (
	"time"

	"go.uber.org/zap/zapcore"
)

// MockObjectEncoder is a helper for testing zapcore.ObjectMarshaler implementations
type MockObjectEncoder struct {
	Fields map[string]interface{}
}

func NewMockObjectEncoder() *MockObjectEncoder {
	return &MockObjectEncoder{
		Fields: make(map[string]interface{}),
	}
}

func (m *MockObjectEncoder) AddArray(string, zapcore.ArrayMarshaler) error   { return nil }
func (m *MockObjectEncoder) AddObject(string, zapcore.ObjectMarshaler) error { return nil }
func (m *MockObjectEncoder) AddBinary(string, []byte)                        {}
func (m *MockObjectEncoder) AddByteString(string, []byte)                    {}
func (m *MockObjectEncoder) AddBool(key string, value bool)                  { m.Fields[key] = value }
func (m *MockObjectEncoder) AddComplex128(string, complex128)                {}
func (m *MockObjectEncoder) AddComplex64(string, complex64)                  {}
func (m *MockObjectEncoder) AddDuration(string, time.Duration)               {}
func (m *MockObjectEncoder) AddFloat64(string, float64)                      {}
func (m *MockObjectEncoder) AddFloat32(string, float32)                      {}
func (m *MockObjectEncoder) AddInt(string, int)                              {}
func (m *MockObjectEncoder) AddInt64(string, int64)                          {}
func (m *MockObjectEncoder) AddInt32(string, int32)                          {}
func (m *MockObjectEncoder) AddInt16(string, int16)                          {}
func (m *MockObjectEncoder) AddInt8(string, int8)                            {}
func (m *MockObjectEncoder) AddString(key string, value string)              { m.Fields[key] = value }
func (m *MockObjectEncoder) AddTime(string, time.Time)                       {}
func (m *MockObjectEncoder) AddUint(string, uint)                            {}
func (m *MockObjectEncoder) AddUint64(string, uint64)                        {}
func (m *MockObjectEncoder) AddUint32(key string, value uint32)              { m.Fields[key] = value }
func (m *MockObjectEncoder) AddUint16(string, uint16)                        {}
func (m *MockObjectEncoder) AddUint8(string, uint8)                          {}
func (m *MockObjectEncoder) AddUintptr(string, uintptr)                      {}
func (m *MockObjectEncoder) AddReflected(string, interface{}) error          { return nil }
func (m *MockObjectEncoder) OpenNamespace(string)                            {}
