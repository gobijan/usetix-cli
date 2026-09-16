package api

import (
	"context"
	"net/url"
)

type TaskPerson struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type TaskEvent struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type TaskAttachment struct {
	SignedID       string `json:"signed_id"`
	AttachableSGID string `json:"attachable_sgid"`
	Filename       string `json:"filename"`
	ContentType    string `json:"content_type"`
	ByteSize       int64  `json:"byte_size"`
	URL            string `json:"url"`
}

type Task struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	DescriptionHTML string           `json:"description_html"`
	Attachments     []TaskAttachment `json:"attachments"`
	ArchivedAt      *string          `json:"archived_at"`
	Status          string           `json:"status"`
	Position        int              `json:"position"`
	LockVersion     int              `json:"lock_version"`
	DueOn           *string          `json:"due_on"`
	Event           *TaskEvent       `json:"event"`
	Assignee        *TaskPerson      `json:"assignee"`
	CreatedAt       string           `json:"created_at"`
	UpdatedAt       string           `json:"updated_at"`
}

type TasksResponse struct {
	Tasks []Task `json:"tasks"`
}

type TaskPeopleResponse struct {
	People []TaskPerson `json:"people"`
}

type TaskEntry struct {
	ID          string           `json:"id"`
	Kind        string           `json:"kind"`
	Body        *string          `json:"body"`
	BodyHTML    *string          `json:"body_html"`
	Attachments []TaskAttachment `json:"attachments"`
	Details     map[string]any   `json:"details"`
	Author      *string          `json:"author"`
	CreatedAt   string           `json:"created_at"`
}

type TaskEntriesResponse struct {
	Entries []TaskEntry `json:"entries"`
}

func (client *Client) ListTasks(ctx context.Context, filters url.Values) (TasksResponse, error) {
	var response TasksResponse
	err := client.get(ctx, "/admin/tasks.json?"+filters.Encode(), &response)
	return response, err
}

func (client *Client) GetTask(ctx context.Context, id string) (Task, error) {
	var task Task
	err := client.get(ctx, taskPath(id)+".json", &task)
	return task, err
}

func (client *Client) TaskPeople(ctx context.Context, event string) (TaskPeopleResponse, error) {
	var response TaskPeopleResponse
	err := client.get(ctx, "/admin/task_assignees.json?"+url.Values{"event_slug": {event}}.Encode(), &response)
	return response, err
}

func (client *Client) TaskEntries(ctx context.Context, id string) (TaskEntriesResponse, error) {
	var response TaskEntriesResponse
	err := client.get(ctx, taskPath(id)+"/comments.json", &response)
	return response, err
}

func (client *Client) CreateTask(ctx context.Context, attributes map[string]any) (Task, string, error) {
	var task Task
	response, err := client.post(ctx, "/admin/tasks.json", map[string]any{"task": attributes}, &task)
	return task, response.Location, err
}

func (client *Client) UpdateTask(ctx context.Context, id string, attributes map[string]any) (Task, error) {
	var task Task
	err := client.patch(ctx, taskPath(id)+".json", map[string]any{"task": attributes}, &task)
	return task, err
}

func (client *Client) MoveTask(ctx context.Context, id string, attributes map[string]any) (Task, error) {
	var task Task
	err := client.patch(ctx, taskPath(id)+"/position.json", map[string]any{"position": attributes}, &task)
	return task, err
}

func (client *Client) CommentOnTask(ctx context.Context, id, body string) (TaskEntry, error) {
	var entry TaskEntry
	_, err := client.post(ctx, taskPath(id)+"/comments.json", map[string]any{"comment": map[string]string{"body": body}}, &entry)
	return entry, err
}

func (client *Client) ArchiveTask(ctx context.Context, id string, attributes map[string]any) (Task, error) {
	var task Task
	_, err := client.post(ctx, taskPath(id)+"/archive.json", attributes, &task)
	return task, err
}

func (client *Client) RestoreTask(ctx context.Context, id string, attributes map[string]any) (Task, error) {
	var task Task
	_, err := client.do(ctx, "DELETE", taskPath(id)+"/archive.json", attributes, &task)
	return task, err
}

func (client *Client) DeleteTask(ctx context.Context, id string) error {
	return client.delete(ctx, taskPath(id)+".json", nil)
}

func taskPath(id string) string {
	return "/admin/tasks/" + url.PathEscape(id)
}
