// Package humanize provides human-like interaction primitives for go-rod.
//
// It is designed to make browser automation harder to reverse-engineer by:
//   - using non-deterministic mouse paths (not a single Bezier curve)
//   - injecting random scroll events during mouse movement
//   - simulating realistic typing with occasional typos and corrections
//   - varying timing based on human speed profiles
package humanize

import "time"

// SpeedProfile selects a base operating speed for a human session.
type SpeedProfile int

const (
	SpeedSlow SpeedProfile = iota
	SpeedNormal
	SpeedFast
)

// Config controls how "human" actions look.
type Config struct {
	// Profile determines the base speed of all actions.
	Profile SpeedProfile

	// Mouse controls mouse movement behavior.
	Mouse MouseConfig

	// Keyboard controls typing behavior.
	Keyboard KeyboardConfig
}

// MouseConfig controls mouse movement realism.
type MouseConfig struct {
	// MinSteps/MaxSteps define the number of intermediate points for a move.
	MinSteps, MaxSteps int

	// MoveSpeedPxPerSec is the average cursor speed in pixels per second.
	MoveSpeedPxPerSec float64

	// SpeedVariance adds randomness to move speed (0.2 = ±20%).
	SpeedVariance float64

	// PauseProbability is the chance to pause mid-movement.
	PauseProbability float64

	// PauseMin/PauseMax control the duration of mid-movement pauses.
	PauseMin, PauseMax time.Duration

	// JitterRadius is the max pixel distance of small hand tremors.
	JitterRadius float64

	// JitterProbability is the chance to add a jitter at each step.
	JitterProbability float64

	// ScrollDuringMoveProbability is the chance to inject a scroll event
	// during a longer mouse movement.
	ScrollDuringMoveProbability float64

	// ScrollMin/ScrollMax control injected scroll amounts.
	ScrollMin, ScrollMax float64

	// OvershootRatio controls how much the cursor may overshoot the target
	// before correcting (0.1 = up to 10% past target).
	OvershootRatio float64
}

// KeyboardConfig controls typing realism.
type KeyboardConfig struct {
	// CPM is the average characters-per-minute typing speed.
	CPM float64

	// CPMVariance adds randomness to typing speed.
	CPMVariance float64

	// TypoProbability is the chance per character to hit a wrong key.
	TypoProbability float64

	// TypoChars are the characters that may be used as typos.
	// Defaults to nearby keys if empty.
	TypoChars []rune

	// PauseAfterTypo is the delay before recognizing and correcting the typo.
	PauseAfterTypo time.Duration

	// BurstLength is the average number of chars typed before a short pause.
	BurstLength int

	// BurstPause is the short pause after a burst.
	BurstPause time.Duration
}

// DefaultConfig returns a balanced human profile.
func DefaultConfig() Config {
	return Config{
		Profile: SpeedNormal,
		Mouse: MouseConfig{
			MinSteps:                    40,
			MaxSteps:                    100,
			MoveSpeedPxPerSec:           3600,
			SpeedVariance:               0.35,
			PauseProbability:            0.08,
			PauseMin:                    80 * time.Millisecond,
			PauseMax:                    350 * time.Millisecond,
			JitterRadius:                1.5,
			JitterProbability:           0.15,
			ScrollDuringMoveProbability: 0.12,
			ScrollMin:                   40,
			ScrollMax:                   250,
			OvershootRatio:              0.08,
		},
		Keyboard: KeyboardConfig{
			CPM:             240,
			CPMVariance:     0.25,
			TypoProbability: 0.03,
			TypoChars:       []rune("qwertyuiopasdfghjklzxcvbnm1234567890"),
			PauseAfterTypo:  250 * time.Millisecond,
			BurstLength:     6,
			BurstPause:      120 * time.Millisecond,
		},
	}
}

// SlowConfig returns a cautious, slower human profile.
func SlowConfig() Config {
	cfg := DefaultConfig()
	cfg.Profile = SpeedSlow
	cfg.Mouse.MoveSpeedPxPerSec = 220
	cfg.Mouse.PauseProbability = 0.12
	cfg.Keyboard.CPM = 160
	cfg.Keyboard.TypoProbability = 0.02
	return cfg
}

// FastConfig returns a quick but still human profile.
func FastConfig() Config {
	cfg := DefaultConfig()
	cfg.Profile = SpeedFast
	cfg.Mouse.MoveSpeedPxPerSec = 520
	cfg.Mouse.PauseProbability = 0.05
	cfg.Keyboard.CPM = 360
	cfg.Keyboard.TypoProbability = 0.04
	return cfg
}
