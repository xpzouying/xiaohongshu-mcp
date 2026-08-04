package browser

import (
	"testing"

	"github.com/go-rod/rod/lib/launcher/flags"
)

func TestNewBrowserLauncherKeepsSandboxEnabled(t *testing.T) {
	l := newBrowserLauncher(browserEngineConfig{headless: true})
	if l.Has(flags.NoSandbox) {
		t.Fatal("browser launcher must not enable --no-sandbox by default")
	}
	if l.Has(flags.Flag("disable-site-isolation-trials")) {
		t.Fatal("browser launcher must keep site isolation enabled")
	}
	if l.Has(flags.Flag("disable-ipc-flooding-protection")) {
		t.Fatal("browser launcher must keep IPC flood protection enabled")
	}
	if got := l.Get(flags.Flag("disable-features")); got != "TranslateUI" {
		t.Fatalf("browser launcher disabled unexpected features: %q", got)
	}
}
