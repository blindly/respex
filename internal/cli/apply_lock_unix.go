//go:build !windows

package cli

import (
	"errors"
	"os"
	"syscall"
)

type applyLock struct {
	file *os.File
}

func tryApplyLock(path string) (*applyLock, bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &applyLock{file: f}, true, nil
}

func (l *applyLock) Close() error {
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	if closeErr := l.file.Close(); err == nil {
		err = closeErr
	}
	return err
}
