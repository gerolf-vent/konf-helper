package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ProcessNotifyer struct {
	processName string
	signal      syscall.Signal

	logger *zap.Logger
}

func NewProcessNotifyer(processName string, signal syscall.Signal) *ProcessNotifyer {
	return &ProcessNotifyer{
		processName: processName,
		signal:      signal,

		logger: zap.L().With(
			zap.String("process-name", processName),
		),
	}
}

func (pn *ProcessNotifyer) ProcessName() string {
	return pn.processName
}

func (pn *ProcessNotifyer) Signal() syscall.Signal {
	return pn.signal
}

func (pn *ProcessNotifyer) Notify() bool {
	pid, err := pn.findPID()
	if err != nil {
		pn.logger.Error("Failed to find process PID", zap.Error(err))
		return false
	}
	pn.logger.Debug("Successfully found process", zap.Int("pid", pid))

	process, err := os.FindProcess(pid)
	if err != nil {
		pn.logger.Error("Failed to find process by PID", zap.Int("pid", pid), zap.Error(err))
		return false
	}

	if err := process.Signal(pn.signal); err != nil {
		pn.logger.Error("Failed to send signal to process", zap.String("signal", pn.signal.String()), zap.Int("pid", pid), zap.Error(err))
		return false
	}
	pn.logger.Info("Successfully notified process", zap.String("signal", pn.signal.String()), zap.Int("pid", pid))
	return true
}

func (pn *ProcessNotifyer) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("process-name", pn.processName)
	enc.AddString("signal", pn.signal.String())
	return nil
}

func (pn *ProcessNotifyer) findPID() (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if pid, err := strconv.ParseUint(entry.Name(), 10, 32); err == nil {
			commPath := filepath.Join("/proc", entry.Name(), "comm")
			if commData, err := os.ReadFile(commPath); err == nil {
				if strings.TrimSpace(string(commData)) == pn.processName {
					return int(pid), nil
				}
			}
		}
	}

	return 0, fmt.Errorf("process %q not found", pn.processName)
}
