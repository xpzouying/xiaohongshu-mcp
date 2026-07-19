//go:build unix

package main

import (
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReadSecretFileRejectsFIFOWithoutBlocking(t *testing.T) {
	path := t.TempDir() + "/secret-fifo"
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readSecretFile(path)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO secret must fail closed")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("FIFO secret open blocked")
	}
}
