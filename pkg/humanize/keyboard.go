package humanize

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Keyboard provides human-like keyboard input.
type Keyboard struct {
	page     *rod.Page
	cfg      Config
	mouse    *Mouse
	lastEl   *rod.Element
}

// NewKeyboard creates a new humanized keyboard wrapper.
func NewKeyboard(page *rod.Page, cfg Config, mouse *Mouse) *Keyboard {
	return &Keyboard{page: page, cfg: cfg, mouse: mouse}
}

// Type types text into el with realistic timing, occasional typos, and corrections.
// ASCII characters are typed key-by-key; CJK and other non-keyboard characters
// are inserted via simulated voice/IME composition events.
func (k *Keyboard) Type(el *rod.Element, text string) error {
	// Ensure the element is rendered before typing, so the cursor lands on a
	// visible input area even when the page is long.
	if err := el.ScrollIntoView(); err != nil {
		return err
	}

	// Move the cursor onto the element and click it, just like a human would
	// before typing. Skip the click if we just typed into the same element to
	// avoid repeated cursor jumps during continuous input (e.g. typing tags
	// char by char). This also keeps the mouse position continuous between
	// actions without querying DOM state that a page could detect.
	if k.mouse != nil && k.lastEl != el {
		if err := k.mouse.Click(el); err != nil {
			return err
		}
	}
	k.lastEl = el

	if err := el.Focus(); err != nil {
		return err
	}

	cfg := k.cfg.Keyboard
	if cfg.TypoChars == nil {
		cfg.TypoChars = []rune("qwertyuiopasdfghjklzxcvbnm1234567890")
	}
	if cfg.BurstLength <= 0 {
		cfg.BurstLength = 1
	}

	cpm := cfg.CPM * (1 + (rand.Float64()*2-1)*cfg.CPMVariance)
	msPerChar := 60000.0 / cpm
	// ASCII is typed roughly 2x faster; CJK voice/IME composition is slower.
	asciiMsPerChar := msPerChar / 2
	cjkMsPerChar := msPerChar * 3

	tokens := tokenizeText(text)
	typed := 0
	lastScrollCheck := 0

	for _, token := range tokens {
		if token.isASCII {
			for _, r := range token.text {
				// Occasional typo for ASCII keys.
				if rand.Float64() < cfg.TypoProbability {
					typo := randomTypo(r, cfg.TypoChars)
					if err := k.press(input.Key(typo)); err != nil {
						return err
					}
					time.Sleep(cfg.PauseAfterTypo + time.Duration(rand.Float64()*200)*time.Millisecond)
					if err := k.pressBackspace(); err != nil {
						return err
					}
					time.Sleep(randDuration(50*time.Millisecond, 150*time.Millisecond))
				}

				if err := k.press(input.Key(r)); err != nil {
					return err
				}
				typed++

				delay := time.Duration(asciiMsPerChar * (0.6 + rand.Float64()*0.8) * float64(time.Millisecond))
				if delay < 10*time.Millisecond {
					delay = 10 * time.Millisecond
				}
				time.Sleep(delay)

				if typed%cfg.BurstLength == 0 {
					time.Sleep(randDuration(cfg.BurstPause, cfg.BurstPause+80*time.Millisecond))
				}

				if typed-lastScrollCheck >= 30 {
					_ = k.scrollToCursor(el)
					lastScrollCheck = typed
				}
			}
		} else {
			// CJK / emoji / special chars: simulate voice/IME composition.
			segments := segmentCJK(token.text)
			for _, seg := range segments {
				if err := k.insertCompositionText(el, seg); err != nil {
					return err
				}
				segRunes := []rune(seg)
				// Pause between voice/IME chunks scales with segment length and
				// the slower CJK speed.
				pause := time.Duration(cjkMsPerChar * float64(len(segRunes)) * (0.8 + rand.Float64()*0.6))
				if pause < 150*time.Millisecond {
					pause = 150 * time.Millisecond
				}
				time.Sleep(pause)
				typed += len(segRunes)

				if typed-lastScrollCheck >= 30 {
					_ = k.scrollToCursor(el)
					lastScrollCheck = typed
				}
			}
		}
	}

	return nil
}

// Press presses a single key with human-like delay.
func (k *Keyboard) Press(key input.Key) error {
	if err := k.press(key); err != nil {
		return err
	}
	time.Sleep(randDuration(50*time.Millisecond, 150*time.Millisecond))
	return nil
}

func (k *Keyboard) press(key input.Key) error {
	return k.page.Keyboard.Press(key)
}

// pressBackspace sends a Backspace key via CDP directly.
func (k *Keyboard) pressBackspace() error {
	return proto.InputDispatchKeyEvent{
		Type:                  proto.InputDispatchKeyEventTypeKeyDown,
		Key:                   "Backspace",
		Code:                  "Backspace",
		WindowsVirtualKeyCode: 8,
	}.Call(k.page)
}

// scrollToCursor scrolls the page so the text cursor remains visible while
// typing long content. It is best-effort and ignores errors to avoid breaking
// the typing flow.
func (k *Keyboard) scrollToCursor(el *rod.Element) error {
	obj, err := el.Eval(`() => {
		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0) return null;
		const range = sel.getRangeAt(0);
		const rect = range.getBoundingClientRect();
		if (rect.width === 0 && rect.height === 0) return null;
		return {
			cursorTop: rect.top + window.scrollY,
			cursorBottom: rect.bottom + window.scrollY,
			cursorLeft: rect.left + window.scrollX,
			cursorRight: rect.right + window.scrollX,
		};
	}`)
	if err != nil {
		return err
	}
	if obj == nil {
		return nil
	}
	val, err := k.page.ObjectToJSON(obj)
	if err != nil {
		return err
	}

	cursorTop := val.Get("cursorTop").Num()
	cursorBottom := val.Get("cursorBottom").Num()

	vp, err := k.viewport()
	if err != nil {
		return err
	}

	const margin = 100
	var deltaY float64
	if cursorBottom > vp.scrollY+vp.height-margin {
		deltaY = cursorBottom - (vp.scrollY + vp.height) + margin + 50
	} else if cursorTop < vp.scrollY+margin {
		deltaY = cursorTop - vp.scrollY - margin - 50
	}

	if deltaY != 0 {
		return k.page.Mouse.Scroll(0, deltaY, 1)
	}
	return nil
}

func (k *Keyboard) viewport() (struct {
	scrollX, scrollY float64
	width, height    float64
}, error) {
	var vp struct {
		scrollX, scrollY float64
		width, height    float64
	}
	obj, err := k.page.Eval(`() => ({
		scrollX: window.scrollX,
		scrollY: window.scrollY,
		innerWidth: window.innerWidth,
		innerHeight: window.innerHeight,
	})`)
	if err != nil {
		return vp, err
	}
	res, err := k.page.ObjectToJSON(obj)
	if err != nil {
		return vp, err
	}
	vp.scrollX = res.Get("scrollX").Num()
	vp.scrollY = res.Get("scrollY").Num()
	vp.width = res.Get("innerWidth").Num()
	vp.height = res.Get("innerHeight").Num()
	return vp, nil
}

// insertText inserts text directly via CDP Input.insertText.
func (k *Keyboard) insertText(text string) error {
	return proto.InputInsertText{Text: text}.Call(k.page)
}

// insertCompositionText simulates voice/IME input by dispatching composition
// and input events, then inserting text into either an input-like element or
// a contenteditable element.
func (k *Keyboard) insertCompositionText(el *rod.Element, text string) error {
	_, err := el.Eval(`(text) => {
		const el = this;
		el.focus();

		const isInputLike = el.tagName === 'INPUT' || el.tagName === 'TEXTAREA';

		// Start composition.
		el.dispatchEvent(new CompositionEvent('compositionstart', {
			bubbles: true,
			cancelable: true,
			data: ''
		}));
		el.dispatchEvent(new CompositionEvent('compositionupdate', {
			bubbles: true,
			cancelable: true,
			data: text
		}));
		el.dispatchEvent(new InputEvent('beforeinput', {
			bubbles: true,
			cancelable: true,
			inputType: 'insertCompositionText',
			data: text
		}));

		if (isInputLike) {
			const start = el.selectionStart == null ? el.value.length : el.selectionStart;
			const end = el.selectionEnd == null ? el.value.length : el.selectionEnd;
			el.value = el.value.slice(0, start) + text + el.value.slice(end);
			el.selectionStart = el.selectionEnd = start + text.length;
		} else {
			// contenteditable: use execCommand to insert at cursor.
			// If it fails (no selection or command unsupported), fall back to appending.
			const ok = document.execCommand('insertText', false, text);
			if (!ok) {
				el.innerText = (el.innerText || '') + text;
			}
		}

		el.dispatchEvent(new InputEvent('input', {
			bubbles: true,
			cancelable: true,
			inputType: 'insertCompositionText',
			data: text
		}));
		el.dispatchEvent(new CompositionEvent('compositionend', {
			bubbles: true,
			cancelable: true,
			data: text
		}));
	}`, text)
	return err
}

// isRodKeySupported reports whether r can be sent via rod.Keyboard.Press.
// Rod only supports a subset of printable ASCII keys; CJK characters panic.
func isRodKeySupported(r rune) bool {
	return r >= 32 && r <= 126
}

// textToken represents a contiguous ASCII or non-ASCII run of text.
type textToken struct {
	text   string
	isASCII bool
}

// tokenizeText splits text into alternating ASCII and non-ASCII tokens.
func tokenizeText(text string) []textToken {
	var tokens []textToken
	var current []rune
	var currentASCII bool
	var hasCurrent bool

	for _, r := range text {
		ascii := isRodKeySupported(r)
		if !hasCurrent {
			current = append(current, r)
			currentASCII = ascii
			hasCurrent = true
			continue
		}
		if ascii == currentASCII {
			current = append(current, r)
		} else {
			tokens = append(tokens, textToken{text: string(current), isASCII: currentASCII})
			current = []rune{r}
			currentASCII = ascii
		}
	}
	if hasCurrent {
		tokens = append(tokens, textToken{text: string(current), isASCII: currentASCII})
	}
	return tokens
}

// segmentCJK splits a CJK string into small segments that mimic voice/IME
// recognition chunks. It prefers splitting at punctuation and keeps segments
// within 2-6 characters.
func segmentCJK(text string) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	// Sentence-ending or phrase-breaking punctuation.
	breakers := map[rune]bool{
		'。': true, '，': true, '、': true, '；': true, '：': true,
		'？': true, '！': true, '…': true, '“': true, '”': true,
		'（': true, '）': true, '【': true, '】': true, '《': true, '》': true,
		'.': true, ',': true, '!': true, '?': true, ';': true, ':': true,
	}

	var segments []string
	var start int
	minSeg := 2
	maxSeg := 6

	for i := 0; i < len(runes); i++ {
		length := i - start + 1
		isBreaker := breakers[runes[i]]

		// Break at punctuation or when segment reaches preferred size.
		if isBreaker || length >= maxSeg {
			if length >= minSeg || isBreaker {
				segments = append(segments, string(runes[start:i+1]))
				start = i + 1
				continue
			}
		}

		// If we're approaching max size and next char isn't a breaker, cut here.
		if length >= maxSeg-1 && i+1 < len(runes) && !breakers[runes[i+1]] {
			segments = append(segments, string(runes[start:i+1]))
			start = i + 1
		}
	}

	if start < len(runes) {
		segments = append(segments, string(runes[start:]))
	}

	return segments
}

// randomTypo returns a character near the intended one or a random typo char.
func randomTypo(intended rune, pool []rune) rune {
	// Try to pick a visually/adjacently similar key from QWERTY rows.
	neighbors := map[rune][]rune{
		'q': {'w', 'a', 's'},
		'w': {'q', 'e', 's'},
		'e': {'w', 'r', 'd'},
		'r': {'e', 't', 'f'},
		't': {'r', 'y', 'g'},
		'y': {'t', 'u', 'h'},
		'u': {'y', 'i', 'j'},
		'i': {'u', 'o', 'k'},
		'o': {'i', 'p', 'l'},
		'p': {'o', 'l'},
		'a': {'q', 'w', 's'},
		's': {'a', 'w', 'd'},
		'd': {'s', 'e', 'f'},
		'f': {'d', 'r', 'g'},
		'g': {'f', 't', 'h'},
		'h': {'g', 'y', 'j'},
		'j': {'h', 'u', 'k'},
		'k': {'j', 'i', 'l'},
		'l': {'k', 'o', 'p'},
		'z': {'a', 's', 'x'},
		'x': {'z', 's', 'd'},
		'c': {'x', 'd', 'f'},
		'v': {'c', 'f', 'g'},
		'b': {'v', 'g', 'h'},
		'n': {'b', 'h', 'j'},
		'm': {'n', 'j', 'k'},
	}

	lower := intended
	if intended >= 'A' && intended <= 'Z' {
		lower = intended - 'A' + 'a'
	}

	if nbs, ok := neighbors[lower]; ok && len(nbs) > 0 && rand.Float64() < 0.7 {
		return nbs[rand.Intn(len(nbs))]
	}

	return pool[rand.Intn(len(pool))]
}

// randomUniform returns a random float in [min, max].
func randomUniform(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

// randomNormal returns a normally distributed value with mean and stddev.
func randomNormal(mean, stddev float64) float64 {
	return mean + stddev*rand.NormFloat64()
}

// clamp ensures value is within [min, max].
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// unused helpers kept for future use.
var _ = randomUniform
var _ = randomNormal
var _ = clamp
var _ = math.Pi
