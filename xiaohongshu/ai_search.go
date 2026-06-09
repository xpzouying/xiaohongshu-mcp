package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/sirupsen/logrus"
)

const aiChatURL = "https://www.xiaohongshu.com/ai_chat"

type AISearchOptions struct {
	IncludeSources bool
	SourceLimit    int
	Timeout        time.Duration
}

type AISearchResult struct {
	Prompt         string          `json:"prompt"`
	ConversationID string          `json:"conversationId,omitempty"`
	MessageID      string          `json:"messageId,omitempty"`
	UUID           string          `json:"uuid,omitempty"`
	Answer         string          `json:"answer"`
	QuerySource    any             `json:"querySource,omitempty"`
	ThinkProgress  any             `json:"thinkProgress,omitempty"`
	FragmentCount  int             `json:"fragmentCount"`
	Sources        *AISourceResult `json:"sources,omitempty"`
}

type AISourceResult struct {
	OK              bool           `json:"ok"`
	Reason          string         `json:"reason,omitempty"`
	ProgressEntries int            `json:"progressEntries,omitempty"`
	Notes           []AISourceNote `json:"notes"`
}

type AISourceNote struct {
	Index      int    `json:"idx"`
	NoteID     string `json:"noteId,omitempty"`
	Title      string `json:"title,omitempty"`
	URL        string `json:"url,omitempty"`
	Cover      string `json:"cover,omitempty"`
	Author     string `json:"author,omitempty"`
	Time       string `json:"time,omitempty"`
	LikedCount string `json:"likedCount,omitempty"`
	Text       string `json:"text,omitempty"`
}

type aiAnswerState struct {
	Found          bool   `json:"found"`
	Finished       bool   `json:"finished"`
	Text           string `json:"text"`
	ConversationID string `json:"conversationId"`
	MessageID      string `json:"messageId"`
	UUID           string `json:"uuid"`
	QuerySource    any    `json:"querySource"`
	ThinkProgress  any    `json:"thinkProgress"`
	FragmentCount  int    `json:"fragmentCount"`
	AIMessageCount int    `json:"aiMessageCount"`
}

type AISearchAction struct {
	page *rod.Page
}

func NewAISearchAction(page *rod.Page) *AISearchAction {
	return &AISearchAction{page: page}
}

func (a *AISearchAction) Chat(ctx context.Context, prompt string, opts AISearchOptions) (*AISearchResult, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	if opts.Timeout <= 0 {
		opts.Timeout = 90 * time.Second
	}
	if opts.SourceLimit <= 0 {
		opts.SourceLimit = 30
	}

	page := a.page.Context(ctx).Timeout(opts.Timeout + 30*time.Second)
	logrus.Infof("打开小红书 AI Chat: %s", aiChatURL)
	page.MustNavigate(aiChatURL)
	page.MustWaitDOMStable()

	if err := waitForAIChatReady(page, opts.Timeout); err != nil {
		return nil, err
	}

	beforeCount := countAIMessages(page)
	if err := sendAIChatPrompt(page, prompt, opts.Timeout); err != nil {
		return nil, err
	}

	answerState, err := waitForAIAnswer(page, beforeCount, opts.Timeout)
	if err != nil {
		return nil, err
	}

	result := &AISearchResult{
		Prompt:         prompt,
		ConversationID: answerState.ConversationID,
		MessageID:      answerState.MessageID,
		UUID:           answerState.UUID,
		Answer:         strings.TrimSpace(answerState.Text),
		QuerySource:    answerState.QuerySource,
		ThinkProgress:  answerState.ThinkProgress,
		FragmentCount:  answerState.FragmentCount,
	}

	if opts.IncludeSources {
		sources, err := getAISources(page, opts.SourceLimit, opts.Timeout)
		if err != nil {
			result.Sources = &AISourceResult{
				OK:     false,
				Reason: err.Error(),
				Notes:  []AISourceNote{},
			}
		} else {
			result.Sources = sources
		}
	}

	return result, nil
}

func waitForAIChatReady(page *rod.Page, timeout time.Duration) error {
	err := waitForJSFlag(page, `() => {
		return document.querySelector("textarea.textarea, .ai-chat-input-box textarea, textarea") ? "1" : "";
	}`, timeout)
	if err != nil {
		return fmt.Errorf("could not find AI chat textarea: %w", err)
	}
	return nil
}

func sendAIChatPrompt(page *rod.Page, prompt string, timeout time.Duration) error {
	textarea, err := page.Element("textarea.textarea, .ai-chat-input-box textarea, textarea")
	if err != nil {
		return fmt.Errorf("could not find AI chat textarea: %w", err)
	}

	if err := textarea.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("could not focus AI chat textarea: %w", err)
	}
	if err := textarea.SelectAllText(); err != nil {
		return fmt.Errorf("could not select AI chat textarea text: %w", err)
	}
	if err := page.Keyboard.Press(input.Backspace); err != nil {
		return fmt.Errorf("could not clear AI chat textarea: %w", err)
	}
	if err := textarea.Input(prompt); err != nil {
		return fmt.Errorf("could not input AI chat prompt: %w", err)
	}
	time.Sleep(500 * time.Millisecond)

	err = waitForJSFlag(page, `() => {
		const button = document.querySelector(".submit-button-wrapper");
		return button && !String(button.className || "").includes("disabled") ? "1" : "";
	}`, timeout)
	if err != nil {
		return fmt.Errorf("AI chat submit button did not become enabled: %w", err)
	}

	button, err := page.Element(".submit-button-wrapper")
	if err != nil {
		return fmt.Errorf("could not find AI chat submit button: %w", err)
	}
	if err := button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("could not click AI chat submit button: %w", err)
	}

	return nil
}

func countAIMessages(page *rod.Page) int {
	raw := page.MustEval(aiAnswerStateJS, 0).String()
	var state aiAnswerState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return 0
	}
	return state.AIMessageCount
}

func waitForAIAnswer(page *rod.Page, beforeCount int, timeout time.Duration) (*aiAnswerState, error) {
	deadline := time.Now().Add(timeout)
	var lastRaw string

	for time.Now().Before(deadline) {
		lastRaw = page.MustEval(aiAnswerStateJS, beforeCount).String()
		var state aiAnswerState
		if err := json.Unmarshal([]byte(lastRaw), &state); err == nil {
			if state.Found && state.Finished && strings.TrimSpace(state.Text) != "" {
				return &state, nil
			}
		}
		time.Sleep(1 * time.Second)
	}

	if len(lastRaw) > 1000 {
		lastRaw = lastRaw[:1000]
	}
	return nil, fmt.Errorf("timed out waiting for AI answer; last state: %s", lastRaw)
}

func getAISources(page *rod.Page, limit int, timeout time.Duration) (*AISourceResult, error) {
	rawProgressCount := page.MustEval(`() => String(document.querySelectorAll(".progress-wrapper").length)`).String()
	if rawProgressCount == "" || rawProgressCount == "0" {
		return nil, fmt.Errorf("no rendered source/progress entry was found for this answer")
	}

	clicked := page.MustEval(`() => {
		const nodes = Array.from(document.querySelectorAll(".progress-wrapper"));
		const last = nodes[nodes.length - 1];
		if (!last) return "";
		last.scrollIntoView({ block: "center", inline: "center" });
		last.click();
		return "1";
	}`).String()
	if clicked != "1" {
		return nil, fmt.Errorf("could not click source/progress entry")
	}

	if err := waitForJSFlag(page, `() => {
		return document.querySelector("section.note-item") ? "1" : "";
	}`, timeout); err != nil {
		return nil, fmt.Errorf("source drawer did not show note cards: %w", err)
	}

	raw := page.MustEval(aiSourceNotesJS, limit).String()
	var notes []AISourceNote
	if err := json.Unmarshal([]byte(raw), &notes); err != nil {
		return nil, fmt.Errorf("failed to parse AI source notes: %w", err)
	}

	result := &AISourceResult{
		OK:              len(notes) > 0,
		ProgressEntries: atoiDefault(rawProgressCount, 0),
		Notes:           notes,
	}
	if len(notes) == 0 {
		result.Reason = "the source drawer opened, but no note cards were found"
	}
	return result, nil
}

func waitForJSFlag(page *rod.Page, js string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if page.MustEval(js).String() == "1" {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

func atoiDefault(value string, fallback int) int {
	var n int
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
		return fallback
	}
	return n
}

const aiAnswerStateJS = `(before) => {
	function clone(value) {
		try { return JSON.parse(JSON.stringify(value)); } catch { return null; }
	}
	function buildText(message) {
		if (!message) return "";
		if (message.text) return message.text;
		return (message.dataFragments || [])
			.filter((fragment) => fragment && fragment.fragmentType === 1)
			.sort((a, b) => (a.indexInConversation || 0) - (b.indexInConversation || 0))
			.map((fragment) => fragment.text || "")
			.join("");
	}
	function collectAIMessages(debug) {
		const messages = clone(debug.messages) || [];
		const rounds = clone(debug.rounds) || [];
		const candidates = [
			...messages.filter((message) => message && message.sender === "ai"),
			...rounds.map((round) => round && round.aiMessage).filter(Boolean)
		];
		const seen = new Set();
		const result = [];
		for (const message of candidates) {
			const key = String(message.msgId || message.id || message.uuid || buildText(message).slice(0, 80));
			if (seen.has(key)) continue;
			seen.add(key);
			result.push(message);
		}
		return result;
	}

	const debug = window.__XHS_AI_DEBUG__;
	if (!debug) return JSON.stringify({ found: false, aiMessageCount: 0 });

	const messages = collectAIMessages(debug);
	const beforeCount = Number(before || 0);
	const aiMessage = messages[beforeCount] || messages[messages.length - 1];
	if (!aiMessage) {
		return JSON.stringify({ found: false, aiMessageCount: messages.length });
	}

	const fragments = aiMessage.dataFragments || [];
	return JSON.stringify({
		found: true,
		finished: !!aiMessage.isFinished,
		text: buildText(aiMessage),
		conversationId: aiMessage.conversationId || "",
		messageId: String(aiMessage.msgId || "").replace(/^ai-/, ""),
		uuid: aiMessage.uuid || "",
		querySource: aiMessage.querySource || null,
		thinkProgress: aiMessage.thinkProgress || [],
		fragmentCount: fragments.length,
		aiMessageCount: messages.length
	});
}`

const aiSourceNotesJS = `(limit) => {
	function abs(href) {
		try { return href ? new URL(href, location.origin).href : ""; }
		catch { return href || ""; }
	}
	function noteIdFromUrl(url) {
		const match = String(url || "").match(/(?:explore|search_result|discovery\/item)\/([0-9a-fA-F]+)/);
		return match ? match[1] : "";
	}

	const notes = Array.from(document.querySelectorAll("section.note-item"))
		.slice(0, Number(limit || 30))
		.map((el, idx) => {
			const titleEl = el.querySelector("a.title");
			const coverEl = el.querySelector("a.cover");
			const authorEl = el.querySelector("a.author");
			const img = el.querySelector("img");
			const url = abs(
				(titleEl && titleEl.getAttribute("href")) ||
				(coverEl && coverEl.getAttribute("href")) ||
				""
			);
			const nameEl = el.querySelector(".name");
			const likedCountEl = el.querySelector(".like-wrapper .count, .like-wrapper");
			return {
				idx,
				noteId: noteIdFromUrl(url),
				title: (titleEl && titleEl.textContent.trim()) || "",
				url,
				cover: (img && (img.currentSrc || img.src)) || "",
				author: (nameEl && nameEl.textContent.trim()) ||
					(authorEl && authorEl.textContent.replace(/\n.*/s, "").trim()) ||
					"",
				time: (el.querySelector(".time") && el.querySelector(".time").textContent.trim()) || "",
				likedCount: (likedCountEl && likedCountEl.textContent.trim()) || "",
				text: el.innerText.trim()
			};
		})
		.filter((note) => note.title || note.url);

	return JSON.stringify(notes);
}`
