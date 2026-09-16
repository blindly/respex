//go:build windows

package cli

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type applyLock struct {
	file *os.File
}

func tryApplyLock(path string) (*applyLock, bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, err
	}
	var overlapped windows.Overlapped
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
	if err != nil {
		f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &applyLock{file: f}, true, nil
}

func (l *applyLock) Close() error {
	var overlapped windows.Overlapped
	err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &overlapped)
	if closeErr := l.file.Close(); err == nil {
		err = closeErr
	}
	return err
}
