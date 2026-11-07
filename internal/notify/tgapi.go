package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// tgClient is the small part of the Telegram Bot API Troopa uses.
type tgClient struct {
	token string
	http  *http.Client
}

func newTG(token string) *tgClient {
	return &tgClient{token: token, http: &http.Client{Timeout: 70 * time.Second}}
}

type tgResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
}

type tgUpdate struct {
	UpdateID      int64       `json:"update_id"`
	Message       *tgMessage  `json:"message"`
	CallbackQuery *tgCallback `json:"callback_query"`
}

type tgMessage struct {
	MessageID int64      `json:"message_id"`
	Chat      tgChat     `json:"chat"`
	Text      string     `json:"text"`
	ReplyTo   *tgMessage `json:"reply_to_message"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgCallback struct {
	ID      string     `json:"id"`
	Data    string     `json:"data"`
	Message *tgMessage `json:"message"`
}

type tgButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

type tgKeyboard struct {
	Rows [][]tgButton `json:"inline_keyboard"`
}

func (c *tgClient) hide(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(strings.ReplaceAll(err.Error(), c.token, "<token>"))
}

func (c *tgClient) call(ctx context.Context, method string, form url.Values) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TelegramAPI+"/bot"+c.token+"/"+method, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, c.hide(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req)
}

func (c *tgClient) do(req *http.Request) (json.RawMessage, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.hide(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var r tgResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("telegram answered %d", resp.StatusCode)
	}
	if !r.OK {
		return nil, fmt.Errorf("telegram: %d %s", r.ErrorCode, r.Description)
	}
	return r.Result, nil
}

// replyKeyboard is the persistent keyboard under the text box.
type replyKeyboard struct {
	Rows       [][]replyButton `json:"keyboard"`
	Resize     bool            `json:"resize_keyboard"`
	Persistent bool            `json:"is_persistent"`
}

type replyButton struct {
	Text string `json:"text"`
}

// forceReply makes Telegram open a reply to the message it comes with.
var forceReply = map[string]any{"force_reply": true, "input_field_placeholder": "…"}

func markupJSON(markup any) string {
	if markup == nil {
		return ""
	}
	raw, err := json.Marshal(markup)
	if err != nil || string(raw) == "null" {
		return ""
	}
	return string(raw)
}

// send posts an HTML message; when the HTML is rejected it retries as plain
// text. markup is an inline keyboard, a reply keyboard, forceReply or nil.
// It returns the message id.
func (c *tgClient) send(ctx context.Context, chat, text string, markup any, replyTo int64) (int64, error) {
	form := url.Values{"chat_id": {chat}, "text": {text}, "parse_mode": {"HTML"}, "disable_web_page_preview": {"true"}}
	if m := markupJSON(markup); m != "" {
		form.Set("reply_markup", m)
	}
	if replyTo != 0 {
		form.Set("reply_to_message_id", strconv.FormatInt(replyTo, 10))
	}
	res, err := c.call(ctx, "sendMessage", form)
	if err != nil && strings.Contains(err.Error(), "parse") {
		form.Del("parse_mode")
		form.Set("text", stripTags(text))
		res, err = c.call(ctx, "sendMessage", form)
	}
	if err != nil {
		return 0, err
	}
	var m tgMessage
	_ = json.Unmarshal(res, &m)
	return m.MessageID, nil
}

// edit replaces the text and inline keyboard of a message the bot sent, so
// menus change in place instead of piling up. "not modified" is not an error.
func (c *tgClient) edit(ctx context.Context, chat string, msgID int64, text string, kb *tgKeyboard) error {
	form := url.Values{
		"chat_id": {chat}, "message_id": {strconv.FormatInt(msgID, 10)}, "text": {text},
		"parse_mode": {"HTML"}, "disable_web_page_preview": {"true"},
	}
	if kb == nil {
		kb = &tgKeyboard{Rows: [][]tgButton{}}
	}
	form.Set("reply_markup", markupJSON(kb))
	_, err := c.call(ctx, "editMessageText", form)
	if err != nil && strings.Contains(err.Error(), "parse") {
		form.Del("parse_mode")
		form.Set("text", stripTags(text))
		_, err = c.call(ctx, "editMessageText", form)
	}
	if err != nil && strings.Contains(err.Error(), "not modified") {
		return nil
	}
	return err
}

type tgCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// setCommands fills the menu button next to the text box.
func (c *tgClient) setCommands(ctx context.Context, cmds []tgCommand) error {
	raw, _ := json.Marshal(cmds)
	_, err := c.call(ctx, "setMyCommands", url.Values{"commands": {string(raw)}})
	return err
}

func (c *tgClient) answer(ctx context.Context, callbackID, text string) {
	_, _ = c.call(ctx, "answerCallbackQuery", url.Values{"callback_query_id": {callbackID}, "text": {short(text, 190)}})
}

// clearButtons removes the keyboard of a message once it was used.
func (c *tgClient) clearButtons(ctx context.Context, chat string, msgID int64) {
	_, _ = c.call(ctx, "editMessageReplyMarkup", url.Values{
		"chat_id": {chat}, "message_id": {strconv.FormatInt(msgID, 10)}, "reply_markup": {`{"inline_keyboard":[]}`},
	})
}

func (c *tgClient) updates(ctx context.Context, offset int64, timeout int) ([]tgUpdate, error) {
	res, err := c.call(ctx, "getUpdates", url.Values{
		"offset": {strconv.FormatInt(offset, 10)}, "timeout": {strconv.Itoa(timeout)},
		"allowed_updates": {`["message","callback_query"]`},
	})
	if err != nil {
		return nil, err
	}
	var ups []tgUpdate
	if err := json.Unmarshal(res, &ups); err != nil {
		return nil, err
	}
	return ups, nil
}

// document sends a file (the full diff) with a caption.
func (c *tgClient) document(ctx context.Context, chat, name string, data []byte, caption string) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", chat)
	_ = w.WriteField("caption", short(caption, 1000))
	part, err := w.CreateFormFile("document", name)
	if err != nil {
		return err
	}
	_, _ = part.Write(data)
	if err := w.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TelegramAPI+"/bot"+c.token+"/sendDocument", &buf)
	if err != nil {
		return c.hide(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, err = c.do(req)
	return err
}
