package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStartupConfigResolvesAuthToken(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		envToken  string
		wantToken string
	}{
		{name: "environment fallback", envToken: "environment-token", wantToken: "environment-token"},
		{name: "explicit flag overrides environment", args: []string{"-token=flag-token"}, envToken: "environment-token", wantToken: "flag-token"},
		{name: "explicit empty flag disables environment token", args: []string{"-token="}, envToken: "environment-token", wantToken: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagSet := flag.NewFlagSet("test", flag.ContinueOnError)

			config, err := parseStartupConfig(flagSet, tt.args, func(string) string {
				return tt.envToken
			})

			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, config.token)
		})
	}
}

func TestParseStartupConfigDoesNotExposeEnvironmentToken(t *testing.T) {
	const sentinel = "review-sentinel-token"
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{name: "help", args: []string{"-h"}, wantErr: flag.ErrHelp},
		{name: "parse error", args: []string{"-not-a-real-flag"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
			flagSet.SetOutput(&output)
			getenvCalled := false

			_, err := parseStartupConfig(flagSet, tt.args, func(string) string {
				getenvCalled = true
				return sentinel
			})

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.Error(t, err)
			}
			assert.False(t, getenvCalled)
			assert.False(t, strings.Contains(output.String(), sentinel), output.String())
		})
	}
}
