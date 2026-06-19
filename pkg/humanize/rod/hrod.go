// Package hrod provides a thin humanized wrapper around go-rod.
//
// It keeps the go-rod API familiar while transparently adding human-like
// mouse movements, scrolls, and keystrokes to the most common automation
// methods (Click, Input, MustClick, MustInput, etc.).
//
// The wrapper uses explicit composition instead of embedding *rod.Page and
// *rod.Element, so child lookup methods can keep the natural Element/MustElement
// names and always return humanized *hrod.Element values.
//
// For methods that are not explicitly wrapped, access the underlying rod value
// via the exported Rod field: page.Rod.XXX() or el.Rod.XXX().
package hrod

import (
	"context"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/humanize"
)

// Browser wraps a *headless_browser.Browser and humanizes the pages it creates.
type Browser struct {
	hb  *headless_browser.Browser
	cfg humanize.Config
}

// NewBrowser wraps an existing *headless_browser.Browser.
func NewBrowser(hb *headless_browser.Browser, cfg humanize.Config) *Browser {
	return &Browser{hb: hb, cfg: cfg}
}

// Rod returns the underlying *headless_browser.Browser.
func (b *Browser) Rod() *headless_browser.Browser {
	return b.hb
}

// Close closes the underlying browser.
func (b *Browser) Close() error {
	b.hb.Close()
	return nil
}

// MustClose is the humanized version of MustClose.
func (b *Browser) MustClose() *Browser {
	b.hb.Close()
	return b
}

// NewPage creates a humanized page.
func (b *Browser) NewPage() *Page {
	return b.wrapPage(b.hb.NewPage())
}

// MustPage opens a humanized page.
func (b *Browser) MustPage(url string) *Page {
	p := b.hb.NewPage()
	if url != "" {
		p.MustNavigate(url)
	}
	return b.wrapPage(p)
}

func (b *Browser) wrapPage(p *rod.Page) *Page {
	if p == nil {
		return nil
	}
	page := &Page{
		Rod:      p,
		Mouse:    p.Mouse,
		Keyboard: p.Keyboard,
		actor:    humanize.New(p, b.cfg),
		browser:  b,
		cfg:      b.cfg,
	}
	// Eagerly initialize the cursor position so the first interaction does not
	// start from rod's default (0,0), which is an obvious automation signature.
	_ = page.actor.Mouse.InitPosition()
	return page
}

// Page wraps a *rod.Page and adds humanized versions of common actions.
// Access the underlying *rod.Page through the exported Rod field when a method
// is not explicitly wrapped.
type Page struct {
	Rod      *rod.Page
	Mouse    *rod.Mouse
	Keyboard *rod.Keyboard
	actor    *humanize.Actor
	browser  *Browser
	cfg      humanize.Config
}

// Actor exposes the underlying humanize actor for advanced use.
func (p *Page) Actor() *humanize.Actor {
	return p.actor
}

// Browser returns the wrapping humanized browser.
func (p *Page) Browser() *Browser {
	return p.browser
}

func (p *Page) wrapPage(rp *rod.Page) *Page {
	if rp == nil {
		return nil
	}
	return &Page{
		Rod:      rp,
		Mouse:    rp.Mouse,
		Keyboard: rp.Keyboard,
		actor:    humanize.New(rp, p.cfg),
		browser:  p.browser,
		cfg:      p.cfg,
	}
}

// Close closes the page.
func (p *Page) Close() error {
	return p.Rod.Close()
}

// Context returns a humanized clone with the specified context.
func (p *Page) Context(ctx context.Context) *Page {
	return p.wrapPage(p.Rod.Context(ctx))
}

// Timeout returns a humanized clone with the specified timeout.
func (p *Page) Timeout(d time.Duration) *Page {
	return p.wrapPage(p.Rod.Timeout(d))
}

// CancelTimeout returns a humanized clone with the timeout cancelled.
func (p *Page) CancelTimeout() *Page {
	return p.wrapPage(p.Rod.CancelTimeout())
}

// Navigate navigates to the URL.
func (p *Page) Navigate(url string) error {
	return p.Rod.Navigate(url)
}

// MustNavigate is the humanized MustNavigate.
func (p *Page) MustNavigate(url string) *Page {
	p.Rod.MustNavigate(url)
	return p
}

// Has checks if an element exists.
func (p *Page) Has(selector string) (bool, *Element, error) {
	found, el, err := p.Rod.Has(selector)
	if err != nil || !found {
		return found, nil, err
	}
	return true, p.wrapElement(el), nil
}

// Element finds an element and returns a humanized wrapper.
func (p *Page) Element(selector string) (*Element, error) {
	el, err := p.Rod.Element(selector)
	if err != nil {
		return nil, err
	}
	return p.wrapElement(el), nil
}

// MustElement finds an element and returns a humanized wrapper.
func (p *Page) MustElement(selector string) *Element {
	return p.wrapElement(p.Rod.MustElement(selector))
}

// ElementR finds an element by regex and returns a humanized wrapper.
func (p *Page) ElementR(selector, regex string) (*Element, error) {
	el, err := p.Rod.ElementR(selector, regex)
	if err != nil {
		return nil, err
	}
	return p.wrapElement(el), nil
}

// MustElementR finds an element by regex and returns a humanized wrapper.
func (p *Page) MustElementR(selector, regex string) *Element {
	return p.wrapElement(p.Rod.MustElementR(selector, regex))
}

// ElementX finds an element by XPath and returns a humanized wrapper.
func (p *Page) ElementX(xpath string) (*Element, error) {
	el, err := p.Rod.ElementX(xpath)
	if err != nil {
		return nil, err
	}
	return p.wrapElement(el), nil
}

// MustElementX finds an element by XPath and returns a humanized wrapper.
func (p *Page) MustElementX(xpath string) *Element {
	return p.wrapElement(p.Rod.MustElementX(xpath))
}

// Elements returns humanized elements.
func (p *Page) Elements(selector string) ([]*Element, error) {
	els, err := p.Rod.Elements(selector)
	if err != nil {
		return nil, err
	}
	result := make([]*Element, len(els))
	for i, el := range els {
		result[i] = p.wrapElement(el)
	}
	return result, nil
}

// MustElements returns humanized elements.
func (p *Page) MustElements(selector string) []*Element {
	els := p.Rod.MustElements(selector)
	result := make([]*Element, len(els))
	for i, el := range els {
		result[i] = p.wrapElement(el)
	}
	return result
}

// Eval evaluates JS on the page.
func (p *Page) Eval(js string, params ...interface{}) (*proto.RuntimeRemoteObject, error) {
	return p.Rod.Eval(js, params...)
}

// MustEval is the humanized MustEval.
func (p *Page) MustEval(js string, params ...interface{}) gson.JSON {
	return p.Rod.MustEval(js, params...)
}

// Wait waits until the JS expression returns true.
func (p *Page) Wait(opts *rod.EvalOptions) error {
	return p.Rod.Wait(opts)
}

// MustWait is the humanized MustWait.
func (p *Page) MustWait(js string, params ...interface{}) *Page {
	p.Rod.MustWait(js, params...)
	return p
}

// WaitLoad waits for the page load event.
func (p *Page) WaitLoad() error {
	return p.Rod.WaitLoad()
}

// MustWaitLoad is the humanized MustWaitLoad.
func (p *Page) MustWaitLoad() *Page {
	p.Rod.MustWaitLoad()
	return p
}

// WaitStable waits for the page to be stable.
func (p *Page) WaitStable(d time.Duration) error {
	return p.Rod.WaitStable(d)
}

// MustWaitStable is the humanized MustWaitStable.
func (p *Page) MustWaitStable() *Page {
	p.Rod.MustWaitStable()
	return p
}

// WaitDOMStable waits for the DOM to be stable.
func (p *Page) WaitDOMStable(d time.Duration, diff float64) error {
	return p.Rod.WaitDOMStable(d, diff)
}

// MustWaitDOMStable is the humanized MustWaitDOMStable.
func (p *Page) MustWaitDOMStable() *Page {
	p.Rod.MustWaitDOMStable()
	return p
}

// MustClick clicks the element matched by selector with human-like movement.
// It panics if the element cannot be found or clicked.
func (p *Page) MustClick(selector string) *Page {
	el, err := p.Element(selector)
	if err != nil {
		panic(err)
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		panic(err)
	}
	return p
}

// MustInput focuses the element and types text with human-like timing.
// It panics if the element cannot be found or the text cannot be entered.
func (p *Page) MustInput(selector string, text string) *Page {
	el, err := p.Element(selector)
	if err != nil {
		panic(err)
	}
	if err := el.Input(text); err != nil {
		panic(err)
	}
	return p
}

// MustScroll scrolls vertically by the given amount in a human-like way.
// It panics if the scroll cannot be performed.
func (p *Page) MustScroll(vertical float64) *Page {
	if err := p.actor.Mouse.Scroll(0, vertical); err != nil {
		panic(err)
	}
	return p
}

func (p *Page) wrapElement(el *rod.Element) *Element {
	if el == nil {
		return nil
	}
	return NewElement(el, p.actor)
}

// NewElement creates a humanized element from a raw *rod.Element.
func NewElement(el *rod.Element, actor *humanize.Actor) *Element {
	if el == nil {
		return nil
	}
	return &Element{Rod: el, actor: actor}
}

// Element wraps a *rod.Element and adds humanized Click/Input methods.
// Access the underlying *rod.Element through the exported Rod field when a method
// is not explicitly wrapped.
type Element struct {
	Rod   *rod.Element
	actor *humanize.Actor
}

// Actor exposes the underlying humanize actor for advanced use.
func (el *Element) Actor() *humanize.Actor {
	return el.actor
}

// Page returns the wrapping humanized page.
func (el *Element) Page() *Page {
	rp := el.Rod.Page()
	return &Page{
		Rod:      rp,
		Mouse:    rp.Mouse,
		Keyboard: rp.Keyboard,
		actor:    el.actor,
		browser:  nil,
		cfg:      el.actor.Config(),
	}
}

// Click performs a human-like click.
func (el *Element) Click(button proto.InputMouseButton, clickCount int) error {
	return el.actor.Mouse.Click(el.Rod)
}

// ClickNoScroll performs a human-like click without scrolling the element into
// view first. Useful for sticky/fixed elements that are already visible.
func (el *Element) ClickNoScroll() error {
	return el.actor.Mouse.ClickNoScroll(el.Rod)
}

// MustClick is the humanized MustClick.
// It panics if the element cannot be clicked.
func (el *Element) MustClick() *Element {
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		panic(err)
	}
	return el
}

// Input performs human-like typing.
func (el *Element) Input(text string) error {
	return el.actor.Keyboard.Type(el.Rod, text)
}

// MustInput is the humanized MustInput.
// It panics if the text cannot be entered.
func (el *Element) MustInput(text string) *Element {
	if err := el.Input(text); err != nil {
		panic(err)
	}
	return el
}

// Hover moves the cursor over the element in a human-like way.
func (el *Element) Hover() error {
	return el.actor.Mouse.Hover(el.Rod)
}

// MustHover is the humanized MustHover.
// It panics if the cursor cannot be moved over the element.
func (el *Element) MustHover() *Element {
	if err := el.Hover(); err != nil {
		panic(err)
	}
	return el
}

// ScrollIntoView scrolls the element into view in a human-like way by
// dispatching wheel events until the element is centered in the viewport.
func (el *Element) ScrollIntoView() error {
	return el.actor.Mouse.ScrollIntoView(el.Rod)
}

// MustScrollIntoView is the humanized MustScrollIntoView.
// It panics if the element cannot be scrolled into view.
func (el *Element) MustScrollIntoView() *Element {
	if err := el.ScrollIntoView(); err != nil {
		panic(err)
	}
	return el
}

// Element finds a child element and returns a humanized wrapper.
func (el *Element) Element(selector string) (*Element, error) {
	child, err := el.Rod.Element(selector)
	if err != nil {
		return nil, err
	}
	return NewElement(child, el.actor), nil
}

// MustElement finds a child element and returns a humanized wrapper.
func (el *Element) MustElement(selector string) *Element {
	return NewElement(el.Rod.MustElement(selector), el.actor)
}

// ElementR finds a child element by regex and returns a humanized wrapper.
func (el *Element) ElementR(selector, regex string) (*Element, error) {
	child, err := el.Rod.ElementR(selector, regex)
	if err != nil {
		return nil, err
	}
	return NewElement(child, el.actor), nil
}

// MustElementR finds a child element by regex and returns a humanized wrapper.
func (el *Element) MustElementR(selector, regex string) *Element {
	return NewElement(el.Rod.MustElementR(selector, regex), el.actor)
}

// ElementX finds a child element by XPath and returns a humanized wrapper.
func (el *Element) ElementX(xpath string) (*Element, error) {
	child, err := el.Rod.ElementX(xpath)
	if err != nil {
		return nil, err
	}
	return NewElement(child, el.actor), nil
}

// MustElementX finds a child element by XPath and returns a humanized wrapper.
func (el *Element) MustElementX(xpath string) *Element {
	return NewElement(el.Rod.MustElementX(xpath), el.actor)
}

// Elements returns humanized child elements.
func (el *Element) Elements(selector string) ([]*Element, error) {
	children, err := el.Rod.Elements(selector)
	if err != nil {
		return nil, err
	}
	result := make([]*Element, len(children))
	for i, child := range children {
		result[i] = NewElement(child, el.actor)
	}
	return result, nil
}

// MustElements returns humanized child elements.
func (el *Element) MustElements(selector string) []*Element {
	children := el.Rod.MustElements(selector)
	result := make([]*Element, len(children))
	for i, child := range children {
		result[i] = NewElement(child, el.actor)
	}
	return result
}

// Parent returns the humanized parent element.
func (el *Element) Parent() (*Element, error) {
	p, err := el.Rod.Parent()
	if err != nil {
		return nil, err
	}
	return NewElement(p, el.actor), nil
}

// Next returns the humanized next sibling element.
func (el *Element) Next() (*Element, error) {
	next, err := el.Rod.Next()
	if err != nil {
		return nil, err
	}
	return NewElement(next, el.actor), nil
}

// Previous returns the humanized previous sibling element.
func (el *Element) Previous() (*Element, error) {
	prev, err := el.Rod.Previous()
	if err != nil {
		return nil, err
	}
	return NewElement(prev, el.actor), nil
}

// Attribute returns the value of an attribute.
func (el *Element) Attribute(name string) (*string, error) {
	return el.Rod.Attribute(name)
}

// MustAttribute is the humanized MustAttribute.
func (el *Element) MustAttribute(name string) *string {
	return el.Rod.MustAttribute(name)
}

// Text returns the element text.
func (el *Element) Text() (string, error) {
	return el.Rod.Text()
}

// MustText is the humanized MustText.
func (el *Element) MustText() string {
	return el.Rod.MustText()
}

// Visible returns whether the element is visible.
func (el *Element) Visible() (bool, error) {
	return el.Rod.Visible()
}

// MustVisible is the humanized MustVisible.
func (el *Element) MustVisible() bool {
	return el.Rod.MustVisible()
}

// WaitVisible waits for the element to become visible.
func (el *Element) WaitVisible() error {
	return el.Rod.WaitVisible()
}

// MustWaitVisible is the humanized MustWaitVisible.
func (el *Element) MustWaitVisible() *Element {
	el.Rod.MustWaitVisible()
	return el
}

// Eval evaluates JS on the element.
func (el *Element) Eval(js string, params ...interface{}) (*proto.RuntimeRemoteObject, error) {
	return el.Rod.Eval(js, params...)
}

// MustEval is the humanized MustEval.
func (el *Element) MustEval(js string, params ...interface{}) gson.JSON {
	return el.Rod.MustEval(js, params...)
}

// Shape returns the element shape.
func (el *Element) Shape() (*proto.DOMGetContentQuadsResult, error) {
	return el.Rod.Shape()
}

// KeyActions returns key actions for the element.
func (el *Element) KeyActions() (*rod.KeyActions, error) {
	return el.Rod.KeyActions()
}

// SelectAllText selects all text in the element.
func (el *Element) SelectAllText() error {
	return el.Rod.SelectAllText()
}

// SetFiles sets files for a file input element.
func (el *Element) SetFiles(paths []string) error {
	return el.Rod.SetFiles(paths)
}

// MustSetFiles is the humanized MustSetFiles.
func (el *Element) MustSetFiles(paths ...string) *Element {
	el.Rod.MustSetFiles(paths...)
	return el
}

// Remove removes the element from the DOM.
func (el *Element) Remove() error {
	return el.Rod.Remove()
}

// MustRemove is the humanized MustRemove.
func (el *Element) MustRemove() *Element {
	el.Rod.MustRemove()
	return el
}

// Timeout returns a humanized clone with the specified timeout.
func (el *Element) Timeout(d time.Duration) *Element {
	return NewElement(el.Rod.Timeout(d), el.actor)
}

// Context returns a humanized clone with the specified context.
func (el *Element) Context(ctx context.Context) *Element {
	return NewElement(el.Rod.Context(ctx), el.actor)
}
