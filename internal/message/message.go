// Package message owns the provider-independent capture model.
package message

import (
	"context"
	"crypto/rand"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

type Message struct {
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Mode           string    `json:"mode"`
	Direction      string    `json:"direction"`
	ID             string    `json:"id"`
	Inbox          string    `json:"inbox"`
	Favorite       bool      `json:"favorite"`
	Tags           []string  `json:"tags"`
	To             string    `json:"to"`
	From           string    `json:"from"`
	Body           string    `json:"body"`
	RunID          string    `json:"run_id"`
	Source         string    `json:"source"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	Analysis       Analysis  `json:"analysis"`
}

type Analysis struct {
	NonGSM     []string `json:"non_gsm,omitempty"`
	Encoding   string   `json:"encoding"`
	Units      int      `json:"units"`
	Segments   int      `json:"segments"`
	Characters int      `json:"characters"`
	OTP        string   `json:"otp,omitempty"`
}

type Input struct {
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	ForceUnicode   bool   `json:"-"`
	Mode           string `json:"mode,omitempty"`
	Direction      string `json:"direction,omitempty"`
	ScenarioID     string `json:"scenario_id,omitempty"`
	CallbackURL    string `json:"callback_url,omitempty"`
	Inbox          string `json:"inbox,omitempty"`
	To             string `json:"to"`
	From           string `json:"from"`
	Body           string `json:"body"`
	RunID          string `json:"run_id,omitempty"`
}

type Filter struct {
	Status   string
	Inbox    string
	Before   time.Time
	BeforeID string
	Favorite bool
	OTPOnly  bool
	Tag      string
	Query    string
	To       string
	RunID    string
	Since    time.Time
	Limit    int
}

// Repository is the persistence seam. Hosted tenancy requires an explicit
// authorization scope before adding a shared PostgreSQL implementation.
type Repository interface {
	Save(context.Context, Message) error
	Get(context.Context, string) (Message, error)
	List(context.Context, Filter) ([]Message, error)
	Delete(context.Context, string) (bool, error)
	Update(context.Context, string, Update) (Message, error)
	Purge(context.Context, string) (int64, error)
	Close() error
}

var ErrInvalid = errors.New("invalid message")

type Update struct {
	Favorite *bool     `json:"favorite,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
}

var phone = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

func ValidRecipient(to string) bool { return phone.MatchString(to) }

var otp = regexp.MustCompile(`(?:^|[^[:alnum:]])([0-9]{4,8})(?:$|[^[:alnum:]])`)

func New(in Input, source string) (Message, error) {
	if len(in.IdempotencyKey) > 128 {
		return Message{}, ErrInvalid
	}
	if in.Mode == "" {
		in.Mode = "capture"
	}
	if in.Mode != "capture" && in.Mode != "simulate" && in.Mode != "relay" {
		return Message{}, ErrInvalid
	}
	if in.Direction == "" {
		in.Direction = "outbound"
	}
	if in.Direction != "outbound" && in.Direction != "inbound" {
		return Message{}, ErrInvalid
	}
	if in.Inbox == "" {
		in.Inbox = "local"
	}
	in.To = strings.TrimSpace(in.To)
	in.From = strings.TrimSpace(in.From)
	if !phone.MatchString(in.To) || in.From == "" || utf8.RuneCountInString(in.From) > 64 ||
		strings.TrimSpace(in.Body) == "" || utf8.RuneCountInString(in.Body) > 4096 ||
		len(in.RunID) > 128 || !utf8.ValidString(in.Body) {
		return Message{}, ErrInvalid
	}
	analysis := Analyze(in.Body)
	if in.ForceUnicode {
		analysis = AnalyzeUnicode(in.Body)
	}
	return Message{
		IdempotencyKey: in.IdempotencyKey,
		Mode:           in.Mode, Direction: in.Direction,
		Inbox: in.Inbox, Tags: []string{},
		ID: "msg_" + rand.Text(), To: in.To, From: in.From, Body: in.Body,
		RunID: in.RunID, Source: source, Status: "captured",
		CreatedAt: time.Now().UTC(), Analysis: analysis,
	}, nil
}

// Analyze uses GSM 03.38 default/extension tables, or UTF-16 code units.
// Segmentation is an estimate: national language tables/provider rewriting
// are deliberately not assumed. OTP extraction is a convenience heuristic.
func Analyze(body string) Analysis {
	return analyze(body, false)
}
func AnalyzeUnicode(body string) Analysis { return analyze(body, true) }
func analyze(body string, forceUnicode bool) Analysis {
	const basic = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"
	const extension = "\f^{}\\[~]|€"
	a := Analysis{Encoding: "GSM-7", Characters: utf8.RuneCountInString(body)}
	seen := make(map[rune]bool)
	for _, r := range body {
		switch {
		case strings.ContainsRune(basic, r):
			a.Units++
		case strings.ContainsRune(extension, r):
			a.Units += 2
		default:
			a.Encoding = "UTF-16"
			if !seen[r] {
				a.NonGSM = append(a.NonGSM, string(r))
				seen[r] = true
			}
		}
	}
	single, multi := 160, 153
	if forceUnicode {
		a.Encoding = "UTF-16"
	}
	if a.Encoding == "UTF-16" {
		a.Units = len(utf16.Encode([]rune(body)))
		single, multi = 70, 67
	}
	if a.Units > 0 {
		a.Segments = 1
	}
	if a.Units > single {
		// An extension escape or UTF-16 surrogate pair cannot be split across
		// segments. Pack complete characters, rather than just dividing units.
		a.Segments = 1
		used := 0
		for _, r := range body {
			width := 1
			if (a.Encoding == "GSM-7" && strings.ContainsRune(extension, r)) || (a.Encoding == "UTF-16" && r > 0xffff) {
				width = 2
			}
			if used+width > multi {
				a.Segments++
				used = 0
			}
			used += width
		}
	}
	if match := otp.FindStringSubmatch(body); len(match) > 1 {
		a.OTP = match[1]
	}
	return a
}
