package cli

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const liveDesignVersionJSON = `{
  "id":1,"name":"Live","live":true,"shop_theme":null,"shop_theme_renewed":false,"published_at":"2026-11-01T09:00:00Z",
  "look":{"theme":"light","accent_color":"#111111","shop_heading_font":"geist","shop_body_font":"geist","shop_palette":null,"shop_style":{}},
  "free_pages":[
    {"kind":"home","state":"live","url":"https://app.usetix.io/admin/website/free_pages/home.json"},
    {"kind":"events","state":"none","url":"https://app.usetix.io/admin/website/free_pages/events.json"}],
  "url":"https://app.usetix.io/admin/website/versions/1.json",
  "created_at":"2026-11-01T09:00:00Z","updated_at":"2026-11-01T09:00:00Z"
}`

const designVersionJSON = `{
  "id":4,"name":"Sommer 2027","live":false,"shop_theme":null,"shop_theme_renewed":false,"published_at":null,
  "look":{"theme":"custom","accent_color":null,"shop_heading_font":"syne","shop_body_font":"geist",
    "shop_palette":{"page":"#101014","card":"#1a1a22","text":"#f5f5f5"},"shop_style":{"corners":"sharp"}},
  "free_pages":[
    {"kind":"home","state":"ready","url":"https://app.usetix.io/admin/website/versions/4/free_pages/home.json"},
    {"kind":"events","state":"none","url":"https://app.usetix.io/admin/website/versions/4/free_pages/events.json"}],
  "url":"https://app.usetix.io/admin/website/versions/4.json",
  "created_at":"2026-11-18T09:00:00Z","updated_at":"2026-11-19T09:00:00Z"
}`

func TestDesignVersionsListShowsTheLiveOneFirstWithLookAndDesigns(t *testing.T) {
	payload := `{"versions":[` + liveDesignVersionJSON + `,` + designVersionJSON + `]}`
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method != http.MethodGet || r.URL.Path != "/admin/website/versions.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, payload)
	})

	stdout, stderr, code := runCLI(t, []string{"design-versions", "list", "--json"}, "", env, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("json: %d %s %s", code, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, payload)

	for _, check := range []struct{ flag, want string }{{"--count", "2\n"}, {"--ids-only", "1\n4\n"}} {
		stdout, _, code = runCLI(t, []string{"design-versions", "list", check.flag}, "", env, nil)
		if code != 0 || stdout != check.want {
			t.Fatalf("%s: %d %q", check.flag, code, stdout)
		}
	}

	stdout, _, code = runCLI(t, []string{"--styled", "design-versions", "list"}, "", env, nil)
	for _, want := range []string{
		"1  Live  live since 2026-11-01T09:00:00Z\n",
		"   Look     light, geist / geist, accent #111111\n",
		"   Designs  home: live, events: none\n",
		"4  Sommer 2027  not live\n",
		"   Look     custom, syne / geist\n",
		"   Designs  home: ready, goes live with its version, events: none\n",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("styled output %q does not contain %q", stdout, want)
		}
	}
}

func TestDesignVersionsShow(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, designVersionJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"design-versions", "show", "4", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "GET /admin/website/versions/4.json" {
		t.Fatalf("show: %d %v %s %s", code, requests, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, designVersionJSON)

	requests = nil
	stdout, _, code = runCLI(t, []string{"design-versions", "show", "four", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "design version ID must be a positive integer") {
		t.Fatalf("invalid ID: %d %v %s", code, requests, stdout)
	}
}

func TestDesignVersionsCreateCopiesTheLiveVersionUnlessAnotherIsNamed(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		w.Header().Set("Location", "https://app.usetix.io/admin/website/versions/4.json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, designVersionJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"design-versions", "create", "--name", "Sommer 2027", "--from", "3", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "POST /admin/website/versions.json" {
		t.Fatalf("create: %d %v %s %s", code, requests, stdout, stderr)
	}
	if !reflect.DeepEqual(body, map[string]any{"shop_version": map[string]any{"name": "Sommer 2027", "source_id": float64(3)}}) {
		t.Fatalf("body: %#v", body)
	}
	assertGuestListJSON(t, stdout, designVersionJSON)
	if !strings.Contains(stdout, `"location": "https://app.usetix.io/admin/website/versions/4.json"`) {
		t.Fatalf("create output: %s", stdout)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "design-versions", "create"}, "", env, nil)
	if code != 0 || !reflect.DeepEqual(body, map[string]any{"shop_version": map[string]any{}}) ||
		!strings.Contains(stdout, "Created the design version") || !strings.Contains(stdout, "4  Sommer 2027  not live") {
		t.Fatalf("create without flags: %d %#v %q", code, body, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"design-versions", "create", "--from", "0", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "--from must be a positive design version ID") {
		t.Fatalf("invalid --from: %d %v %s", code, requests, stdout)
	}
}

func TestDesignVersionsCreateReportsValidationErrors(t *testing.T) {
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"errors":{"name":["is too long (maximum is 60 characters)"]}}`)
	})
	stdout, _, code := runCLI(t, []string{"design-versions", "create", "--name", strings.Repeat("x", 61), "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "name: is too long") {
		t.Fatalf("validation: %d %s", code, stdout)
	}
}

func TestDesignVersionsRename(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		_, _ = io.WriteString(w, strings.Replace(designVersionJSON, `"name":"Sommer 2027"`, `"name":"Sommer"`, 1))
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "design-versions", "rename", "4", "Sommer"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "PATCH /admin/website/versions/4.json" ||
		!reflect.DeepEqual(body, map[string]any{"shop_version": map[string]any{"name": "Sommer"}}) {
		t.Fatalf("rename: %d %v %#v %s %s", code, requests, body, stdout, stderr)
	}
	if !strings.Contains(stdout, "Renamed the design version") || !strings.Contains(stdout, "4  Sommer  not live") {
		t.Fatalf("rename output: %q", stdout)
	}

	for _, args := range [][]string{{"4"}, {"4", " "}, {"four", "Sommer"}} {
		requests = nil
		_, _, code = runCLI(t, append([]string{"design-versions", "rename", "--json"}, args...), "", env, nil)
		if code != 1 || len(requests) != 0 {
			t.Fatalf("%v: %d %v", args, code, requests)
		}
	}
}

func TestDesignVersionsDeleteRequiresConfirmationAndNeverTheLiveOne(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/admin/website/versions/1.json" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"errors":{"base":["The live version cannot be deleted. Take another one live first."]}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	stdout, _, code := runCLI(t, []string{"design-versions", "delete", "4", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "requires explicit confirmation") || !strings.Contains(stdout, "Re-run with --yes") {
		t.Fatalf("unconfirmed delete: %d %v %s", code, requests, stdout)
	}

	stdout, _, code = runCLI(t, []string{"design-versions", "delete", "4", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/website/versions/4.json" {
		t.Fatalf("delete: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, `{"id":4,"deleted":true}`)

	stdout, _, code = runCLI(t, []string{"design-versions", "delete", "1", "--yes", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "The live version cannot be deleted.") {
		t.Fatalf("live delete: %d %s", code, stdout)
	}
}

func TestDesignVersionsPublishShowsWhatGoesLiveBeforePosting(t *testing.T) {
	published := strings.Replace(designVersionJSON, `"live":false,"shop_theme":null,"shop_theme_renewed":false,"published_at":null`,
		`"live":true,"shop_theme":null,"shop_theme_renewed":false,"published_at":"2026-11-20T08:00:00Z"`, 1)
	published = strings.Replace(published, `"state":"ready"`, `"state":"live"`, 1)
	current := designVersionJSON
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPost {
			w.Header().Set("Location", "https://app.usetix.io/admin/website/versions/4.json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, published)
			return
		}
		_, _ = io.WriteString(w, current)
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "design-versions", "publish", "4"}, "", env, nil)
	notice := `"Sommer 2027" takes the live version's place at once: the shop wears its look (custom, syne / geist) and shows its own designs (home: ready, goes live with its version, events: none). The version live now is kept beside it.`
	if code != 1 || len(requests) != 1 || requests[0] != "GET /admin/website/versions/4.json" || !strings.Contains(stdout+stderr, notice) || !strings.Contains(stdout+stderr, "--yes") {
		t.Fatalf("preview: %d %v %q %q", code, requests, stdout, stderr)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"design-versions", "publish", "4", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/website/versions/4/publication.json" {
		t.Fatalf("publish: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, published)

	stdout, _, code = runCLI(t, []string{"--styled", "design-versions", "publish", "4", "--yes"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Took the design version live") || !strings.Contains(stdout, "4  Sommer 2027  live since 2026-11-20T08:00:00Z") {
		t.Fatalf("styled publish: %d %q", code, stdout)
	}

	current = published
	requests = nil
	stdout, _, code = runCLI(t, []string{"--styled", "design-versions", "publish", "4"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "GET /admin/website/versions/4.json" || !strings.Contains(stdout, "already live") {
		t.Fatalf("already live: %d %v %q", code, requests, stdout)
	}
}

func TestDesignVersionsPublishReportsWhyTheServerTurnsItDown(t *testing.T) {
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"errors":{"base":["Theme is not included in the list"]}}`)
	})
	stdout, _, code := runCLI(t, []string{"design-versions", "publish", "4", "--yes", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "Theme is not included in the list") {
		t.Fatalf("validation: %d %s", code, stdout)
	}
}
