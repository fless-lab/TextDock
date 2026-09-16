// Package audit defines a deliberately closed metadata vocabulary. Never add
// request bodies, URLs, credentials, phone numbers or arbitrary detail maps here.
package audit

import (
	"context"
	"errors"
	"slices"
	"time"
)

type Actor struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}
type actorKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}
func ContextActor(ctx context.Context) Actor {
	if actor, ok := ctx.Value(actorKey{}).(Actor); ok {
		return actor
	}
	return Actor{Kind: "system"}
}

var Actions = []string{"api_key.created", "api_key.revoked", "export.prepared", "inbox.created", "login.failed", "membership.assigned", "membership.removed", "message.created", "message.deleted", "message.updated", "messages.pruned", "messages.purged", "project.created", "scenario.deleted", "scenario.saved", "session.created", "session.revoked", "user.created", "user.disabled", "user.enabled", "user.password_changed"}

type Event struct {
	ID           int64     `json:"id"`
	At           time.Time `json:"at"`
	Actor        Actor     `json:"actor"`
	Action       string    `json:"action"`
	Outcome      string    `json:"outcome"`
	ProjectID    string    `json:"project_id,omitempty"`
	InboxID      string    `json:"inbox_id,omitempty"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id,omitempty"`
	Count        int64     `json:"count"`
	RoleBefore   string    `json:"role_before,omitempty"`
	RoleAfter    string    `json:"role_after,omitempty"`
	Permissions  []string  `json:"permissions,omitempty"`
}

func (e Event) Validate() error {
	if !slices.Contains(Actions, e.Action) || !slices.Contains([]string{"committed", "denied", "prepared"}, e.Outcome) || !slices.Contains([]string{"operator", "local_operator", "user", "api_key", "system", "anonymous"}, e.Actor.Kind) || !slices.Contains([]string{"project", "inbox", "message", "user", "session", "api_key", "membership", "login", "export", "scenario"}, e.ResourceType) {
		return errors.New("invalid audit metadata")
	}
	for _, id := range []string{e.Actor.ID, e.ProjectID, e.InboxID, e.ResourceID} {
		if len(id) > 128 {
			return errors.New("audit identifier too long")
		}
	}
	for _, role := range []string{e.RoleBefore, e.RoleAfter} {
		if !slices.Contains([]string{"", "viewer", "member", "admin"}, role) {
			return errors.New("invalid audit role")
		}
	}
	if e.Count < 0 || len(e.Permissions) > 3 {
		return errors.New("invalid audit count or permissions")
	}
	for _, p := range e.Permissions {
		if !slices.Contains([]string{"messages:read", "messages:write", "messages:delete"}, p) {
			return errors.New("invalid audit permission")
		}
	}
	return nil
}

type Filter struct {
	ProjectID, InboxID, Action, Outcome, ActorID string
	Before                                       int64
	Since                                        time.Time
	Limit                                        int
}
type Repository interface {
	AppendAudit(context.Context, Event) error
	ListAudit(context.Context, Filter) ([]Event, error)
}
