//go:build !unix

package main

import (
	"errors"
	"os"
)

func openSecretFileNoFollow(string) (*os.File, error) {
	return nil, errors.New("secure secret files are unsupported on this platform")
}
