package workspace

import "context"

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Inbox struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
}
type Repository interface {
	ListProjects(context.Context) ([]Project, error)
	ListInboxes(context.Context) ([]Inbox, error)
	CreateProject(context.Context, Project, Inbox) error
	CreateInbox(context.Context, Inbox) error
	InboxExists(context.Context, string) (bool, error)
}
