// Package humanize provides human-like browser interactions for go-rod.
//
// Unlike a single smooth Bezier curve, this package:
//   - splits movements into multiple segments with different curve families
//   - adds jitter, pauses, overshoots, and random scroll events
//   - types with variable speed, bursts, and occasional typos + corrections
//   - exposes slow/normal/fast speed profiles based on real human timing
package humanize

import (
	"github.com/go-rod/rod"
)

// Actor groups humanized mouse and keyboard actions.
type Actor struct {
	Mouse    *Mouse
	Keyboard *Keyboard
	cfg      Config
}

// New creates a humanized actor for the given page.
func New(page *rod.Page, cfg Config) *Actor {
	mouse := NewMouse(page, cfg)
	return &Actor{
		Mouse:    mouse,
		Keyboard: NewKeyboard(page, cfg, mouse),
		cfg:      cfg,
	}
}

// Config returns the actor's configuration.
func (a *Actor) Config() Config {
	return a.cfg
}
