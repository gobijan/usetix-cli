package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const taskJSON = `{"id":"TASK1","title":"Door briefing","status":"open","position":1,"lock_version":2,"description":"Meet at 18:00","description_html":"<p>Meet at 18:00</p>","attachments":[],"event":{"slug":"october-party","title":"October Party"},"assignee":{"id":42,"name":"Lisa"},"due_on":"2026-10-24","archived_at":null,"created_at":"2026-09-13T16:00:00Z","updated_at":"2026-09-13T16:10:00Z"}`
const taskEntryJSON = `{"id":"ENTRY1","kind":"comment","body":"Confirmed","body_html":"<p>Confirmed</p>","attachments":[{"signed_id":"SIGNED","attachable_sgid":"SGID","filename":"briefing.pdf","content_type":"application/pdf","byte_size":4096,"url":"/admin/task_uploads/SIGNED"}],"details":{},"author":"Lisa","created_at":"2026-09-13T16:15:00Z"}`

func TestTaskReadsAndOutputModes(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer task-test-token" {
			t.Error("missing authentication")
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admin/tasks.json":
			_, _ = io.WriteString(w, `{"tasks":[`+taskJSON+`]}`)
		case "/admin/tasks/TASK1.json":
			_, _ = io.WriteString(w, taskJSON)
		case "/admin/task_assignees.json":
			_, _ = io.WriteString(w, `{"people":[{"id":42,"name":"Lisa"}]}`)
		case "/admin/tasks/TASK1/comments.json":
			_, _ = io.WriteString(w, `{"entries":[`+taskEntryJSON+`]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_API_URL": server.URL, "USETIX_TOKEN": "task-test-token"}
	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"--json", "tasks", "list", "--event", "october-party", "--assignee", "me", "--archived", "--hide-done"}, `"tasks": [`},
		{[]string{"--count", "tasks", "list"}, "1\n"},
		{[]string{"--ids-only", "tasks", "list"}, "TASK1\n"},
		{[]string{"--styled", "tasks", "list"}, "Door briefing"},
		{[]string{"--json", "tasks", "show", "TASK1"}, `"lock_version": 2`},
		{[]string{"--styled", "tasks", "show", "TASK1"}, "Meet at 18:00"},
		{[]string{"--json", "tasks", "people", "--event", "october-party"}, `"people": [`},
		{[]string{"--ids-only", "tasks", "people"}, "42\n"},
		{[]string{"--json", "tasks", "comments", "TASK1"}, `"entries": [`},
		{[]string{"--styled", "tasks", "comments", "TASK1"}, "Confirmed"},
		{[]string{"--styled", "tasks", "comments", "TASK1"}, "File: briefing.pdf (4096 bytes)\n  /admin/task_uploads/SIGNED"},
	} {
		stdout, stderr, code := runCLI(t, check.args, "", env, nil)
		if code != 0 || stderr != "" || !strings.Contains(stdout, check.want) {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", check.args, code, stdout, stderr)
		}
		if len(check.args) > 8 && query != "archived=true&assignee_id=me&event_slug=october-party&hide_done=true" {
			t.Fatalf("filters = %s", query)
		}
	}
}

func TestTaskMutationsSendOnlyExplicitFields(t *testing.T) {
	var method, path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		body = nil
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case path == "/admin/tasks/TASK1.json" && method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(path, "/comments.json"):
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, taskEntryJSON)
		case path == "/admin/tasks.json":
			w.Header().Set("Location", "/admin/tasks/TASK1.json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, taskJSON)
		default:
			_, _ = io.WriteString(w, taskJSON)
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_API_URL": server.URL, "USETIX_TOKEN": "task-test-token"}
	for _, check := range []struct {
		args                  []string
		method, path, payload string
	}{
		{[]string{"create", "--title", "Door briefing"}, "POST", "/admin/tasks.json", `{"task":{"title":"Door briefing"}}`},
		{[]string{"update", "TASK1", "--event=", "--assignee=", "--due=", "--description=", "--lock-version", "0"}, "PATCH", "/admin/tasks/TASK1.json", `{"task":{"assignee_id":"","description":"","due_on":"","event_slug":"","lock_version":0}}`},
		{[]string{"move", "TASK1", "--status", "done", "--before", "TASK2", "--lock-version", "2"}, "PATCH", "/admin/tasks/TASK1/position.json", `{"position":{"before_id":"TASK2","lock_version":2,"status":"done"}}`},
		{[]string{"comment", "TASK1", "--body", "Confirmed"}, "POST", "/admin/tasks/TASK1/comments.json", `{"comment":{"body":"Confirmed"}}`},
		{[]string{"archive", "TASK1", "--lock-version", "2"}, "POST", "/admin/tasks/TASK1/archive.json", `{"lock_version":2}`},
		{[]string{"restore", "TASK1", "--lock-version", "3"}, "DELETE", "/admin/tasks/TASK1/archive.json", `{"lock_version":3}`},
		{[]string{"delete", "TASK1", "--yes"}, "DELETE", "/admin/tasks/TASK1.json", `null`},
	} {
		args := append([]string{"--json", "tasks"}, check.args...)
		stdout, stderr, code := runCLI(t, args, "", env, nil)
		encoded, _ := json.Marshal(body)
		if code != 0 || method != check.method || path != check.path || string(encoded) != check.payload {
			t.Fatalf("%v: code=%d %s %s body=%s stdout=%q stderr=%q", args, code, method, path, encoded, stdout, stderr)
		}
	}
}

func TestTaskValidationAndConflictsDoNotRetryWrites(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"errors":{"base":["Someone changed this task."]}}`)
	}))
	defer server.Close()
	env := map[string]string{"USETIX_API_URL": server.URL, "USETIX_TOKEN": "task-test-token"}
	for _, args := range [][]string{
		{"delete", "TASK1"}, {"create"}, {"update", "TASK1"}, {"move", "TASK1", "--status", "later"}, {"comment", "TASK1"}, {"archive", "TASK1", "--lock-version", "-1"},
	} {
		stdout, _, code := runCLI(t, append([]string{"--json", "tasks"}, args...), "", env, nil)
		if code != 1 || calls != 0 {
			t.Fatalf("%v: code=%d calls=%d stdout=%q", args, code, calls, stdout)
		}
	}
	stdout, _, code := runCLI(t, []string{"--json", "tasks", "update", "TASK1", "--title", "New title", "--lock-version", "1"}, "", env, nil)
	if code == 0 || calls != 1 || !strings.Contains(stdout, "Someone changed this task") {
		t.Fatalf("conflict: code=%d calls=%d stdout=%q", code, calls, stdout)
	}
}
