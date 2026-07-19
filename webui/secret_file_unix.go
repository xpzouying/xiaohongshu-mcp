//go:build unix

package main

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func openSecretFileNoFollow(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New("could not create secret file handle")
	}
	return file, nil
}
