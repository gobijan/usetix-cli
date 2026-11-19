package cli

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const freePagePartJSON = `{
  "key":"3f9a1c0b7d2e","name":"Hero","heading":"Summer Night","anchor":null,"hidden":false,
  "texts":[{"key":"a81c4e02f6b9","kind":"heading","original":"Summer Night","text":"Summer Night"}],
  "media":[{"kind":"picture","file":"poster.jpg","still":null}],
  "items":[],
  "url":"https://app.usetix.io/admin/website/free_pages/home/parts/3f9a1c0b7d2e.json"
}`

const freePageSlidesPartJSON = `{
  "key":"77b0c2d4e6f8","name":"Videos","heading":null,"anchor":"feel","hidden":true,
  "texts":[{"key":"c0ffee112233","kind":"heading","original":"Friday","text":"Freitag"}],
  "media":[{"kind":"video","file":"clip.mp4","still":"poster.jpg"}],
  "items":[{"number":1,"name":"Slide 1","texts":["c0ffee112233"],"files":["clip.mp4"]}],
  "url":"https://app.usetix.io/admin/website/free_pages/home/parts/77b0c2d4e6f8.json"
}`

const freePageJSON = `{
  "kind":"home","title":"Usetix Club","state":"live","shows":"free_page","held":false,
  "template":"<main data-part=\"Hero\"><h1>{{ shop.name }}</h1></main>","stylesheet":"h1 { color: var(--shop-text); }",
  "follows_look":true,
  "files":[{"name":"poster.jpg","content_type":"image/jpeg","byte_size":184220}],
  "parts":[` + freePagePartJSON + `,` + freePageSlidesPartJSON + `],
  "check":{"passed":true,"checked_at":"2026-11-17T12:00:00Z","problems":[],"violations":[],"unjudged":[],"render_error":null,
    "pictures":{"desktop":"https://app.usetix.io/admin/website/free_pages/home/pictures/desktop","phone":null,"narrow":null}},
  "design_fee":{"amount":"3.00","currency":"EUR"},
  "design":null,
  "url":"https://app.usetix.io/admin/website/free_pages/home.json","updated_at":"2026-11-17T12:00:04Z"
}`

const noFreePageJSON = `{
  "kind":"event","title":"Summer Night","state":"none","shows":"shared_design","held":false,
  "template":null,"stylesheet":null,"follows_look":false,"files":[],"parts":[],"check":null,
  "design_fee":{"amount":"0.00","currency":"EUR"},"design":null,
  "url":"https://app.usetix.io/admin/events/summer-night/page/free_page.json","updated_at":null
}`

const failedFreePageJSON = `{
  "kind":"events","title":"All events","state":"failed","shows":"usual_page","held":false,
  "template":"<h1>{{ event.title }}</h1>","stylesheet":"","follows_look":false,"files":[],"parts":[],
  "check":{"passed":false,"checked_at":"2026-11-17T12:00:00Z",
    "problems":["phone: color-contrast (serious): Text must stand out — .title (2.1:1, needs 4.5:1)"],
    "violations":[{"rule":"color-contrast","impact":"serious","help":"Text must stand out","size":"phone","elements":[".title (2.1:1, needs 4.5:1)"]}],
    "unjudged":["desktop: .date"],"render_error":null,"pictures":{"desktop":null,"phone":null,"narrow":null}},
  "design_fee":{"amount":"3.00","currency":"EUR"},
  "design":{"state":"failed","error":"The page did not pass its check","requested_at":"2026-11-19T10:12:00Z","fee":{"amount":"3.00","currency":"EUR"}},
  "url":"https://app.usetix.io/admin/website/free_pages/events.json","updated_at":"2026-11-17T12:00:04Z"
}`

func designingFreePageJSON(fee string) string {
	return strings.Replace(strings.Replace(freePageJSON, `"state":"live","shows":"free_page"`, `"state":"none","shows":"usual_page"`, 1),
		`"design":null`, `"design":{"state":"pending","error":null,"requested_at":"2026-11-19T10:12:00Z","fee":{"amount":"`+fee+`","currency":"EUR"}}`, 1)
}

func TestFreePageShowNamesThePageTheSameWayAtEveryAddress(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, freePageJSON)
	})

	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"home"}, "GET /admin/website/free_pages/home.json"},
		{[]string{"events"}, "GET /admin/website/free_pages/events.json"},
		{[]string{"about-us"}, "GET /admin/website/free_pages/about-us.json"},
		{[]string{"--event", "summer-night"}, "GET /admin/events/summer-night/page/free_page.json"},
		{[]string{"home", "--version", "4"}, "GET /admin/website/versions/4/free_pages/home.json"},
		{[]string{"events", "--version", "4"}, "GET /admin/website/versions/4/free_pages/events.json"},
	} {
		requests = nil
		stdout, stderr, code := runCLI(t, append([]string{"free-pages", "show", "--json"}, check.args...), "", env, nil)
		if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != check.want {
			t.Fatalf("%v: %d %v %s %s", check.args, code, requests, stdout, stderr)
		}
		assertGuestListJSON(t, stdout, freePageJSON)
	}

	stdout, stderr, code := runCLI(t, []string{"--styled", "free-pages", "show", "home"}, "", env, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("styled: %d %s %s", code, stdout, stderr)
	}
	for _, want := range []string{
		"Usetix Club (home page)",
		"State     live",
		"Visitors  this free page",
		"Look      follows the shop's look",
		"Files     poster.jpg",
		"Blocks    2 blocks",
		"Check     passed 2026-11-17T12:00:00Z",
		"AI price  next design 3.00 EUR plus VAT from Usetix Credit, taken once the page passed its check\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("styled output %q does not contain %q", stdout, want)
		}
	}
	if strings.Contains(stdout, "AI  ") || strings.Contains(stdout, "publish") {
		t.Fatalf("styled output names a design or a waiting version: %q", stdout)
	}
}

func TestFreePageShowSaysStateCheckAndDesignInWords(t *testing.T) {
	payload := noFreePageJSON
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		_, _ = io.WriteString(w, payload)
	})

	stdout, _, code := runCLI(t, []string{"--styled", "free-pages", "show", "--event", "summer-night"}, "", env, nil)
	for _, want := range []string{
		"Summer Night (event page)", "State     no free page", "Visitors  the design all events wear",
		"AI price  next design free: your shop's first page\n",
		"usetix free-pages set --event summer-night --template FILE --stylesheet FILE",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("none: %d %q does not contain %q", code, stdout, want)
		}
	}
	stdout, _, _ = runCLI(t, []string{"free-pages", "show", "--event", "summer-night", "--json"}, "", env, nil)
	assertGuestListJSON(t, stdout, noFreePageJSON)

	payload = failedFreePageJSON
	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "show", "events"}, "", env, nil)
	for _, want := range []string{
		"All events (all event pages)", "State     failed its check", "Visitors  the usual page", "Look      keeps its own colors",
		"Check     failed 2026-11-17T12:00:00Z · 1 text without a contrast verdict",
		"    - phone: color-contrast (serious): Text must stand out — .title (2.1:1, needs 4.5:1)",
		"AI price  next design 3.00 EUR plus VAT from Usetix Credit, taken once the page passed its check\n",
		"AI        failed (The page did not pass its check) · asked 2026-11-19T10:12:00Z · fee 3.00 EUR plus VAT from Usetix Credit once the page passed its check",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("failed: %d %q does not contain %q", code, stdout, want)
		}
	}
	stdout, _, _ = runCLI(t, []string{"free-pages", "show", "events", "--json"}, "", env, nil)
	assertGuestListJSON(t, stdout, failedFreePageJSON)

	for _, check := range []struct{ state, shows, wantState, wantShows string }{
		{"checking", "earlier_version", "State     being checked", "Visitors  the version that was live before"},
		{"held", "usual_page", "State     passed, waiting to be taken live", "Take it live with: usetix free-pages publish home"},
		{"ready", "with_version", "State     ready, goes live with its version", "Visitors  nothing of it until its version is live"},
	} {
		payload = strings.Replace(freePageJSON, `"state":"live","shows":"free_page"`, `"state":"`+check.state+`","shows":"`+check.shows+`"`, 1)
		stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "show", "home"}, "", env, nil)
		if code != 0 || !strings.Contains(stdout, check.wantState+"\n") || !strings.Contains(stdout, check.wantShows+"\n") {
			t.Fatalf("%s: %d %q", check.state, code, stdout)
		}
	}
}

func TestFreePageCommandsRefuseAPageNamedTwiceOrNotAtAll(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, freePageJSON)
	})

	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"free-pages", "show"}, "pass --event SLUG for an event's page"},
		{[]string{"free-pages", "show", "home", "--event", "summer-night"}, "name a page or pass --event, not both"},
		{[]string{"free-pages", "show", "home", "events"}, "accepts 1 arg(s), received 2"},
		{[]string{"free-pages", "show", ""}, "the page can't be blank"},
		{[]string{"free-pages", "show", "--event", "summer-night", "--version", "4"}, "--version takes home or events, not --event"},
		{[]string{"free-pages", "show", "about-us", "--version", "4"}, "a design version has its own design for home and events only"},
		{[]string{"free-pages", "show", "home", "--version", "0"}, "--version must be a positive design version ID"},
		{[]string{"free-pages", "blocks", "hide", "3f9a1c0b7d2e"}, "accepts 2 arg(s), received 1"},
		{[]string{"free-pages", "blocks", "hide", "home", "3f9a1c0b7d2e", "--event", "summer-night"}, "name a page or pass --event, not both"},
		{[]string{"free-pages", "publish", "home", "--version", "4"}, "unknown flag: --version"},
		{[]string{"free-pages", "discard", "home", "--version", "4", "--yes"}, "unknown flag: --version"},
		{[]string{"free-pages", "design", "home", "--version", "4", "--prompt", "Dark", "--yes"}, "unknown flag: --version"},
	} {
		stdout, _, code := runCLI(t, append(check.args, "--json"), "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, `"code": "usage"`) || !strings.Contains(stdout, check.want) {
			t.Fatalf("%v: %d %v %s", check.args, code, requests, stdout)
		}
	}
}

func TestFreePageSetSendsTheFilesAndOnlyWhatWasPassed(t *testing.T) {
	directory := t.TempDir()
	write := func(name string, contents []byte) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	template := write("home.liquid", []byte("<main data-part=\"Hero\"><h1>{{ shop.name }}</h1></main>\n"))
	stylesheet := write("home.css", []byte("h1 { color: var(--shop-text); }\n"))
	empty := write("empty.css", nil)
	blank := write("blank.liquid", []byte(" \n"))
	binary := write("poster.jpg", []byte{0xff, 0xd8, 0xff, 0xe0})

	var requests []string
	var body map[string]any
	checking := strings.Replace(freePageJSON, `"state":"live","shows":"free_page","held":false`, `"state":"checking","shows":"usual_page","held":true`, 1)
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		_, _ = io.WriteString(w, checking)
	})

	stdout, stderr, code := runCLI(t, []string{"free-pages", "set", "home", "--template", template, "--stylesheet", stylesheet, "--hold", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "PATCH /admin/website/free_pages/home.json" {
		t.Fatalf("set: %d %v %s %s", code, requests, stdout, stderr)
	}
	want := map[string]any{"free_page": map[string]any{
		"template":   "<main data-part=\"Hero\"><h1>{{ shop.name }}</h1></main>\n",
		"stylesheet": "h1 { color: var(--shop-text); }\n",
		"hold":       true,
	}}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body: %#v", body)
	}
	assertGuestListJSON(t, stdout, checking)

	for _, check := range []struct {
		args        []string
		wantRequest string
		wantBody    map[string]any
	}{
		{[]string{"--event", "summer-night", "--template", template}, "PATCH /admin/events/summer-night/page/free_page.json",
			map[string]any{"template": "<main data-part=\"Hero\"><h1>{{ shop.name }}</h1></main>\n"}},
		{[]string{"events", "--stylesheet", empty}, "PATCH /admin/website/free_pages/events.json", map[string]any{"stylesheet": ""}},
		{[]string{"home", "--hold=false"}, "PATCH /admin/website/free_pages/home.json", map[string]any{"hold": false}},
		{[]string{"home", "--version", "4", "--stylesheet", stylesheet}, "PATCH /admin/website/versions/4/free_pages/home.json",
			map[string]any{"stylesheet": "h1 { color: var(--shop-text); }\n"}},
	} {
		requests = nil
		_, _, code = runCLI(t, append([]string{"free-pages", "set", "--json"}, check.args...), "", env, nil)
		if code != 0 || len(requests) != 1 || requests[0] != check.wantRequest || !reflect.DeepEqual(body, map[string]any{"free_page": check.wantBody}) {
			t.Fatalf("%v: %d %v %#v", check.args, code, requests, body)
		}
	}

	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "set", "home", "--template", template, "--hold"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Set the free page") || !strings.Contains(stdout, "State     being checked") || !strings.Contains(stdout, "Visitors  the usual page") {
		t.Fatalf("styled set: %d %q", code, stdout)
	}

	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"home"}, "nothing to set"},
		{[]string{"home", "--template", filepath.Join(directory, "missing.liquid")}, "read template:"},
		{[]string{"home", "--template", directory}, "is a directory"},
		{[]string{"home", "--template", blank}, "the template file is empty"},
		{[]string{"home", "--stylesheet", binary}, "the stylesheet file is not UTF-8 text"},
	} {
		requests = nil
		stdout, _, code = runCLI(t, append([]string{"free-pages", "set", "--json"}, check.args...), "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, check.want) {
			t.Fatalf("%v: %d %v %s", check.args, code, requests, stdout)
		}
	}
}

func TestFreePageSetReportsWhatCanNotBeKept(t *testing.T) {
	template := filepath.Join(t.TempDir(), "home.liquid")
	if err := os.WriteFile(template, []byte("{% if %}"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"errors":{"template":["Liquid syntax error: Syntax Error in tag 'if'"]}}`)
	})
	stdout, _, code := runCLI(t, []string{"free-pages", "set", "home", "--template", template, "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "template: Liquid syntax error") {
		t.Fatalf("validation: %d %s", code, stdout)
	}
}

func TestFreePageCheckAsksForAnotherCheck(t *testing.T) {
	var requests []string
	status := http.StatusAccepted
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if status == http.StatusAccepted {
			w.Header().Set("Location", "https://app.usetix.io/admin/website/free_pages/home.json")
		}
		w.WriteHeader(status)
	})

	stdout, stderr, code := runCLI(t, []string{"free-pages", "check", "home", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "POST /admin/website/free_pages/home/check.json" {
		t.Fatalf("check: %d %v %s %s", code, requests, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, `{"page":"home","checking":true}`)
	if !strings.Contains(stdout, `"location": "https://app.usetix.io/admin/website/free_pages/home.json"`) {
		t.Fatalf("check output: %s", stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"free-pages", "check", "--event", "summer-night", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/events/summer-night/page/free_page/check.json" {
		t.Fatalf("event check: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, `{"event":"summer-night","checking":true}`)

	requests = nil
	stdout, _, code = runCLI(t, []string{"free-pages", "check", "events", "--version", "4", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/website/versions/4/free_pages/events/check.json" {
		t.Fatalf("version check: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, `{"page":"events","version":4,"checking":true}`)

	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "check", "--event", "summer-night"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "usetix free-pages show --event summer-night") {
		t.Fatalf("styled check: %d %q", code, stdout)
	}

	status = http.StatusNotFound
	stdout, _, code = runCLI(t, []string{"free-pages", "check", "home", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "not_found"`) {
		t.Fatalf("check without a free page: %d %s", code, stdout)
	}
}

func TestFreePagePublishTakesTheWaitingVersionLive(t *testing.T) {
	var requests []string
	waiting := true
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if !waiting {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"errors":{"base":["No draft that passed the check is waiting here."]}}`)
			return
		}
		_, _ = io.WriteString(w, freePageJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"free-pages", "publish", "home", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "POST /admin/website/free_pages/home/publication.json" {
		t.Fatalf("publish: %d %v %s %s", code, requests, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, freePageJSON)

	requests = nil
	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "publish", "--event", "summer-night"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/events/summer-night/page/free_page/publication.json" ||
		!strings.Contains(stdout, "Took the free page live") || !strings.Contains(stdout, "State     live") {
		t.Fatalf("event publish: %d %v %q", code, requests, stdout)
	}

	waiting = false
	stdout, _, code = runCLI(t, []string{"free-pages", "publish", "home", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "No draft that passed the check is waiting here.") {
		t.Fatalf("nothing waits: %d %s", code, stdout)
	}
}

func TestFreePageDiscardAndRemoveRequireConfirmation(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})

	stdout, _, code := runCLI(t, []string{"free-pages", "discard", "home", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "requires explicit confirmation") || !strings.Contains(stdout, "Re-run with --yes") {
		t.Fatalf("unconfirmed discard: %d %v %s", code, requests, stdout)
	}
	stdout, _, code = runCLI(t, []string{"free-pages", "discard", "home", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/website/free_pages/home/publication.json" {
		t.Fatalf("discard: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, `{"page":"home","discarded":true}`)

	requests = nil
	stdout, _, code = runCLI(t, []string{"free-pages", "discard", "--event", "summer-night", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/events/summer-night/page/free_page/publication.json" {
		t.Fatalf("event discard: %d %v %s", code, requests, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"free-pages", "remove", "--event", "summer-night", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "requires explicit confirmation") || !strings.Contains(stdout, "Re-run with --yes") {
		t.Fatalf("unconfirmed remove: %d %v %s", code, requests, stdout)
	}
	stdout, _, code = runCLI(t, []string{"free-pages", "remove", "--event", "summer-night", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/events/summer-night/page/free_page.json" {
		t.Fatalf("remove: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, `{"event":"summer-night","deleted":true}`)

	requests = nil
	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "remove", "home", "--version", "4", "--yes"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/website/versions/4/free_pages/home.json" || !strings.Contains(stdout, "Removed the free page of home --version 4") {
		t.Fatalf("version remove: %d %v %q", code, requests, stdout)
	}
}

func TestFreePageShowLeavesTheNextDesignOutWhereTheServerNamesNoFee(t *testing.T) {
	version := strings.Replace(strings.Replace(freePageJSON, `"design_fee":{"amount":"3.00","currency":"EUR"}`, `"design_fee":null`, 1),
		`"state":"live","shows":"free_page"`, `"state":"ready","shows":"with_version"`, 1)
	older := strings.Replace(freePageJSON, `"design_fee":{"amount":"3.00","currency":"EUR"},`, ``, 1)
	payload := version
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		_, _ = io.WriteString(w, payload)
	})

	stdout, _, code := runCLI(t, []string{"free-pages", "show", "home", "--version", "4", "--json"}, "", env, nil)
	if code != 0 {
		t.Fatalf("version: %d %s", code, stdout)
	}
	assertGuestListJSON(t, stdout, version)

	// A server that doesn't name the fee yet reads like one that says null
	payload = older
	stdout, _, code = runCLI(t, []string{"free-pages", "show", "home", "--json"}, "", env, nil)
	if code != 0 {
		t.Fatalf("older server: %d %s", code, stdout)
	}
	assertGuestListJSON(t, stdout, strings.Replace(freePageJSON, `"design_fee":{"amount":"3.00","currency":"EUR"}`, `"design_fee":null`, 1))

	for _, current := range []string{version, older} {
		payload = current
		stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "show", "home"}, "", env, nil)
		if code != 0 || strings.Contains(stdout, "AI price") {
			t.Fatalf("styled output names a price: %d %q", code, stdout)
		}
	}
}

func TestFreePageDesignShowsThePriceBeforeAskingAndTheAgreedFeeAfter(t *testing.T) {
	var requests []string
	var body map[string]any
	page := freePageJSON
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPost {
			body = received
			w.Header().Set("Location", "https://app.usetix.io/admin/website/free_pages/home.json")
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_, _ = io.WriteString(w, page)
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "free-pages", "design", "home", "--prompt", "Dark and classy"}, "", env, nil)
	price := "Usetix's AI designs the page for 3.00 EUR plus VAT from Usetix Credit, taken once the page passed its check; a page that doesn't pass costs nothing. Re-run with --yes."
	if code != 1 || !reflect.DeepEqual(requests, []string{"GET /admin/website/free_pages/home.json"}) ||
		!strings.Contains(stdout+stderr, "requires explicit confirmation") || !strings.Contains(stdout+stderr, price) {
		t.Fatalf("price before asking: %d %v %q %q", code, requests, stdout, stderr)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"free-pages", "design", "--event", "summer-night", "--change", "--prompt", "Larger dates", "--json"}, "", env, nil)
	if code != 1 || !reflect.DeepEqual(requests, []string{"GET /admin/events/summer-night/page/free_page.json"}) ||
		!strings.Contains(stdout, `"code": "usage"`) || !strings.Contains(stdout, "3.00 EUR plus VAT from Usetix Credit") {
		t.Fatalf("event price before asking: %d %v %s", code, requests, stdout)
	}

	page = designingFreePageJSON("3.00")
	requests = nil
	stdout, stderr, code = runCLI(t, []string{"free-pages", "design", "home", "--prompt", "Dark and classy", "--yes", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || !reflect.DeepEqual(requests, []string{"POST /admin/website/free_pages/home/design.json", "GET /admin/website/free_pages/home.json"}) {
		t.Fatalf("design: %d %v %s %s", code, requests, stdout, stderr)
	}
	if !reflect.DeepEqual(body, map[string]any{"design": map[string]any{"prompt": "Dark and classy"}}) {
		t.Fatalf("body: %#v", body)
	}
	assertGuestListJSON(t, stdout, designingFreePageJSON("3.00"))
	if !strings.Contains(stdout, `"location": "https://app.usetix.io/admin/website/free_pages/home.json"`) {
		t.Fatalf("design output: %s", stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "design", "--event", "summer-night", "--change", "--prompt", "Make the dates larger", "--yes"}, "", env, nil)
	if code != 0 || !reflect.DeepEqual(requests, []string{"POST /admin/events/summer-night/page/free_page/design.json", "GET /admin/events/summer-night/page/free_page.json"}) ||
		!reflect.DeepEqual(body, map[string]any{"design": map[string]any{"prompt": "Make the dates larger", "change": true}}) {
		t.Fatalf("design change: %d %v %#v", code, requests, body)
	}
	if !strings.Contains(stdout, "Asked Usetix's AI for the page") ||
		!strings.Contains(stdout, "AI        pending · asked 2026-11-19T10:12:00Z · fee 3.00 EUR plus VAT from Usetix Credit once the page passed its check") ||
		strings.Contains(stdout, "AI price") {
		t.Fatalf("styled design: %q", stdout)
	}

	for _, args := range [][]string{{"home", "--yes"}, {"home", "--prompt", " ", "--yes"}, {"home", "--prompt", " "}} {
		requests = nil
		stdout, _, code = runCLI(t, append([]string{"free-pages", "design", "--json"}, args...), "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "prompt") {
			t.Fatalf("%v: %d %v %s", args, code, requests, stdout)
		}
	}
}

func TestFreePageDesignSaysTheShopsFirstPageIsFree(t *testing.T) {
	var requests []string
	page := strings.Replace(freePageJSON, `"design_fee":{"amount":"3.00","currency":"EUR"}`, `"design_fee":{"amount":"0.00","currency":"EUR"}`, 1)
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_, _ = io.WriteString(w, page)
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "free-pages", "design", "home", "--prompt", "Dark and classy"}, "", env, nil)
	if code != 1 || !reflect.DeepEqual(requests, []string{"GET /admin/website/free_pages/home.json"}) ||
		!strings.Contains(stdout+stderr, "Usetix's AI designs the page for free: your shop's first page. Re-run with --yes.") ||
		strings.Contains(stdout+stderr, "plus VAT") || strings.Contains(stdout+stderr, "costs nothing") {
		t.Fatalf("free before asking: %d %v %q %q", code, requests, stdout, stderr)
	}

	page = designingFreePageJSON("0.00")
	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "design", "home", "--prompt", "Dark and classy", "--yes"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "AI        pending · asked 2026-11-19T10:12:00Z · no fee: the shop's first page is free") {
		t.Fatalf("first page: %d %q", code, stdout)
	}
}

func TestFreePageDesignStatesTheTermsWhereTheServerNamesNoFee(t *testing.T) {
	var requests []string
	page := ""
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, page)
	})

	for name, current := range map[string]string{
		"null":    strings.Replace(freePageJSON, `"design_fee":{"amount":"3.00","currency":"EUR"}`, `"design_fee":null`, 1),
		"missing": strings.Replace(freePageJSON, `"design_fee":{"amount":"3.00","currency":"EUR"},`, ``, 1),
	} {
		page, requests = current, nil
		stdout, stderr, code := runCLI(t, []string{"--styled", "free-pages", "design", "home", "--prompt", "Dark and classy"}, "", env, nil)
		for _, want := range []string{
			"requires explicit confirmation",
			"The shop's first page from Usetix's AI is free.",
			"Each further page costs Usetix's fee plus VAT from Usetix Credit, taken once the page passed its check",
			"Re-run with --yes.",
		} {
			if code != 1 || !reflect.DeepEqual(requests, []string{"GET /admin/website/free_pages/home.json"}) || !strings.Contains(stdout+stderr, want) {
				t.Fatalf("%s: %d %v %q %q does not contain %q", name, code, requests, stdout, stderr, want)
			}
		}
	}
}

func TestFreePageDesignReportsAFailedReadBeforeAsking(t *testing.T) {
	var requests []string
	status := http.StatusNotFound
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.WriteHeader(status)
	})

	for _, check := range []struct {
		status int
		code   string
	}{{http.StatusNotFound, "not_found"}, {http.StatusUnauthorized, "auth_required"}, {http.StatusBadGateway, "api_error"}} {
		status, requests = check.status, nil
		stdout, _, code := runCLI(t, []string{"free-pages", "design", "no-such-page", "--prompt", "Dark", "--json"}, "", env, nil)
		if code == 0 || !reflect.DeepEqual(requests, []string{"GET /admin/website/free_pages/no-such-page.json"}) ||
			!strings.Contains(stdout, `"code": "`+check.code+`"`) || strings.Contains(stdout, "requires explicit confirmation") {
			t.Fatalf("%d: %d %v %s", check.status, code, requests, stdout)
		}
	}

	requests = nil
	stdout, _, code := runCLI(t, []string{"free-pages", "design", "home", "--prompt", "Dark", "--json"}, "", map[string]string{"USETIX_API_URL": env["USETIX_API_URL"]}, nil)
	if code == 0 || len(requests) != 0 || strings.Contains(stdout, "requires explicit confirmation") {
		t.Fatalf("without a token: %d %v %s", code, requests, stdout)
	}
}

func TestFreePageDesignReportsWhyTheServerTurnsItDown(t *testing.T) {
	var requests []string
	status := http.StatusUnprocessableEntity
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusBadGateway)
		case status == http.StatusAccepted:
			w.WriteHeader(status)
		default:
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"errors":{"base":["Your Usetix Credit doesn't cover the page. Top it up first."]}}`)
		}
	})

	stdout, _, code := runCLI(t, []string{"free-pages", "design", "home", "--prompt", "Dark", "--yes", "--json"}, "", env, nil)
	if code == 0 || len(requests) != 1 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "doesn't cover the page") {
		t.Fatalf("credit short: %d %v %s", code, requests, stdout)
	}

	// The design was accepted, only reading it back failed: nobody should ask again
	status = http.StatusAccepted
	requests = nil
	stdout, _, code = runCLI(t, []string{"free-pages", "design", "home", "--prompt", "Dark", "--yes", "--json"}, "", env, nil)
	if code == 0 || len(requests) != 2 || !strings.Contains(stdout, "Usetix's AI was asked and is designing the page") || !strings.Contains(stdout, "usetix free-pages show home") {
		t.Fatalf("read after asking: %d %v %s", code, requests, stdout)
	}
}

func TestFreePageBlocksListShowsKeysTextsAndMedia(t *testing.T) {
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method != http.MethodGet || r.URL.Path != "/admin/website/free_pages/home.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, freePageJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"free-pages", "blocks", "list", "home", "--json"}, "", env, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("json: %d %s %s", code, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, `[`+freePagePartJSON+`,`+freePageSlidesPartJSON+`]`)

	for _, check := range []struct{ flag, want string }{{"--count", "2\n"}, {"--ids-only", "3f9a1c0b7d2e\n77b0c2d4e6f8\n"}} {
		stdout, _, code = runCLI(t, []string{"free-pages", "blocks", "list", "home", check.flag}, "", env, nil)
		if code != 0 || stdout != check.want {
			t.Fatalf("%s: %d %q", check.flag, code, stdout)
		}
	}

	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "blocks", "list", "home"}, "", env, nil)
	for _, want := range []string{
		"3f9a1c0b7d2e  Hero  Summer Night\n",
		"    a81c4e02f6b9  heading  Summer Night\n",
		"    picture  poster.jpg\n",
		"77b0c2d4e6f8  Videos  (left out)\n",
		"    c0ffee112233  heading  Freitag  (design: Friday)\n",
		"    video  clip.mp4\n",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("styled output %q does not contain %q", stdout, want)
		}
	}
}

func TestFreePageBlocksHideUnhideAndReword(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		_, _ = io.WriteString(w, freePageSlidesPartJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"free-pages", "blocks", "hide", "home", "77b0c2d4e6f8", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "PATCH /admin/website/free_pages/home/parts/77b0c2d4e6f8.json" ||
		!reflect.DeepEqual(body, map[string]any{"part": map[string]any{"hidden": true}}) {
		t.Fatalf("hide: %d %v %#v %s %s", code, requests, body, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, freePageSlidesPartJSON)

	requests = nil
	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "blocks", "unhide", "--event", "summer-night", "77b0c2d4e6f8"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "PATCH /admin/events/summer-night/page/free_page/parts/77b0c2d4e6f8.json" ||
		!reflect.DeepEqual(body, map[string]any{"part": map[string]any{"hidden": false}}) || !strings.Contains(stdout, "Brought back the block") {
		t.Fatalf("unhide: %d %v %#v %q", code, requests, body, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "blocks", "reword", "home", "77b0c2d4e6f8", "--text", "c0ffee112233=Freitag = Tanz", "--text", "a81c4e02f6b9="}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "PATCH /admin/website/free_pages/home/parts/77b0c2d4e6f8.json" ||
		!reflect.DeepEqual(body, map[string]any{"part": map[string]any{"texts": map[string]any{"c0ffee112233": "Freitag = Tanz", "a81c4e02f6b9": ""}}}) {
		t.Fatalf("reword: %d %v %#v %q", code, requests, body, stdout)
	}
	if !strings.Contains(stdout, "Reworded the block") || !strings.Contains(stdout, "c0ffee112233  heading  Freitag  (design: Friday)") {
		t.Fatalf("reword output: %q", stdout)
	}

	requests = nil
	_, _, code = runCLI(t, []string{"free-pages", "blocks", "hide", "home", "77b0c2d4e6f8", "--version", "4", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "PATCH /admin/website/versions/4/free_pages/home/parts/77b0c2d4e6f8.json" {
		t.Fatalf("version hide: %d %v", code, requests)
	}

	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"home", "77b0c2d4e6f8"}, "required flag"},
		{[]string{"home", "77b0c2d4e6f8", "--text", "Freitag"}, "--text must be TEXT_KEY=WORDING"},
		{[]string{"home", "77b0c2d4e6f8", "--text", "=Freitag"}, "--text must be TEXT_KEY=WORDING"},
	} {
		requests = nil
		stdout, _, code = runCLI(t, append([]string{"free-pages", "blocks", "reword", "--json"}, check.args...), "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, check.want) {
			t.Fatalf("%v: %d %v %s", check.args, code, requests, stdout)
		}
	}
}

func TestFreePageBlocksReportAnUnknownBlockAndAFileThatDoesNotFit(t *testing.T) {
	status, payload := http.StatusNotFound, ""
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, payload)
	})

	stdout, _, code := runCLI(t, []string{"free-pages", "blocks", "hide", "home", "0123456789", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "not_found"`) {
		t.Fatalf("unknown block: %d %s", code, stdout)
	}

	status, payload = http.StatusUnprocessableEntity, `{"errors":{"template":["Liquid syntax error"]}}`
	stdout, _, code = runCLI(t, []string{"free-pages", "blocks", "reword", "home", "77b0c2d4e6f8", "--text", "c0ffee112233={{", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "template: Liquid syntax error") {
		t.Fatalf("validation: %d %s", code, stdout)
	}
}

func TestFreePageBlocksMoveAnswersWithTheBlocksInTheirOrder(t *testing.T) {
	var requests []string
	var body map[string]any
	moved := `[` + freePageSlidesPartJSON + `,` + freePagePartJSON + `]`
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		_, _ = io.WriteString(w, moved)
	})

	stdout, stderr, code := runCLI(t, []string{"free-pages", "blocks", "move", "home", "77b0c2d4e6f8", "--position", "1", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "PUT /admin/website/free_pages/home/parts/77b0c2d4e6f8/position.json" ||
		!reflect.DeepEqual(body, map[string]any{"position": float64(1)}) {
		t.Fatalf("move: %d %v %#v %s %s", code, requests, body, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, moved)

	requests = nil
	stdout, _, code = runCLI(t, []string{"free-pages", "blocks", "move", "--event", "summer-night", "77b0c2d4e6f8", "--position", "1", "--ids-only"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "PUT /admin/events/summer-night/page/free_page/parts/77b0c2d4e6f8/position.json" || stdout != "77b0c2d4e6f8\n3f9a1c0b7d2e\n" {
		t.Fatalf("event move: %d %v %q", code, requests, stdout)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "free-pages", "blocks", "move", "home", "77b0c2d4e6f8", "--position", "1"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Moved the block") || strings.Index(stdout, "77b0c2d4e6f8  Videos") > strings.Index(stdout, "3f9a1c0b7d2e  Hero") {
		t.Fatalf("styled move: %d %q", code, stdout)
	}

	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"home", "77b0c2d4e6f8"}, "required flag"},
		{[]string{"home", "77b0c2d4e6f8", "--position", "0"}, "--position must be 1 or more"},
	} {
		requests = nil
		stdout, _, code = runCLI(t, append([]string{"free-pages", "blocks", "move", "--json"}, check.args...), "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, check.want) {
			t.Fatalf("%v: %d %v %s", check.args, code, requests, stdout)
		}
	}
}
