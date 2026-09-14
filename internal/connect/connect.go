// Package connect defines scoped phone credentials independently of HTTP.
package connect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

var ErrInvalid = errors.New("pairing code or device session is invalid or expired")

type Scope struct {
	Inbox string `json:"inbox"`
	To    string `json:"to"`
	RunID string `json:"run_id"`
}

type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Scope     Scope     `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

type Repository interface {
	CreatePair(context.Context, string, Scope, time.Time) error
	ClaimPair(context.Context, string, string, Device) (Device, error)
	DeviceByHash(context.Context, string) (Device, error)
	ListDevices(context.Context) ([]Device, error)
	RevokeDevice(context.Context, string) (bool, error)
}

func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
