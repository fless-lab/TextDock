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
	ID        string    `json:"id"`
	To        string    `json:"to"`
	From      string    `json:"from"`
	Body      string    `json:"body"`
	RunID     string    `json:"run_id"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Analysis  Analysis  `json:"analysis"`
}

type Analysis struct {
	Encoding   string `json:"encoding"`
	Units      int    `json:"units"`
	Segments   int    `json:"segments"`
	Characters int    `json:"characters"`
	OTP        string `json:"otp,omitempty"`
}

type Input struct {
	To    string `json:"to"`
	From  string `json:"from"`
	Body  string `json:"body"`
	RunID string `json:"run_id,omitempty"`
}

type Filter struct {
	Query string
	To    string
	RunID string
	Since time.Time
	Limit int
}

// Repository is the persistence seam. Hosted tenancy requires an explicit
// authorization scope before adding a shared PostgreSQL implementation.
type Repository interface {
	Save(context.Context, Message) error
	List(context.Context, Filter) ([]Message, error)
	Delete(context.Context, string) (bool, error)
	Close() error
}

var ErrInvalid = errors.New("invalid message")
var phone = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
var otp = regexp.MustCompile(`(?:^|[^[:alnum:]])([0-9]{4,8})(?:$|[^[:alnum:]])`)

func New(in Input, source string) (Message, error) {
	in.To = strings.TrimSpace(in.To)
	in.From = strings.TrimSpace(in.From)
	if !phone.MatchString(in.To) || in.From == "" || utf8.RuneCountInString(in.From) > 64 ||
		strings.TrimSpace(in.Body) == "" || utf8.RuneCountInString(in.Body) > 4096 ||
		len(in.RunID) > 128 || !utf8.ValidString(in.Body) {
		return Message{}, ErrInvalid
	}
	return Message{
		ID: "msg_" + rand.Text(), To: in.To, From: in.From, Body: in.Body,
		RunID: in.RunID, Source: source, Status: "captured",
		CreatedAt: time.Now().UTC(), Analysis: Analyze(in.Body),
	}, nil
}

// Analyze uses GSM 03.38 default/extension tables, or UTF-16 code units.
// Segmentation is an estimate: national language tables/provider rewriting
// are deliberately not assumed. OTP extraction is a convenience heuristic.
func Analyze(body string) Analysis {
	const basic = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"
	const extension = "\f^{}\\[~]|€"
	a := Analysis{Encoding: "GSM-7", Characters: utf8.RuneCountInString(body)}
	for _, r := range body {
		switch {
		case strings.ContainsRune(basic, r):
			a.Units++
		case strings.ContainsRune(extension, r):
			a.Units += 2
		default:
			a.Encoding = "UTF-16"
		}
	}
	single, multi := 160, 153
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
