package cli

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const themeJSON = `{
  "key":"limelight","name":"Limelight","summary":"Big pictures, clean edges, the events right at the top.",
  "categories":["theater","comedy"],"pages":["home","events"],
  "website_pages":[{"key":"gallery","title":"Gallery"}],
  "highlights":[{"key":"events","title":"The programme as a poster wall","text":"Every event with flyer, date and price in a row to browse."}],
  "look":{"theme":"custom","accent_color":"#E0105A","shop_heading_font":"sora","shop_body_font":"work_sans",
    "shop_palette":{"page":"#f3f3f3","card":"#ffffff","text":"#1e1b1c"},"shop_style":{"corners":"sharp"}},
  "url":"https://app.usetix.io/admin/website/themes/limelight.json"
}`

const plainThemeJSON = `{
  "key":"bokeh","name":"Bokeh","summary":null,"categories":["club"],"pages":["home","events"],"website_pages":[],"highlights":[],
  "look":{"theme":"dark","accent_color":null,"shop_heading_font":"poppins","shop_body_font":"poppins","shop_palette":null,"shop_style":{}},
  "url":"https://app.usetix.io/admin/website/themes/bokeh.json"
}`

func TestThemesListNamesEachThemeAndFiltersByCategory(t *testing.T) {
	payload := `{"themes":[` + themeJSON + `,` + plainThemeJSON + `]}`
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		_, _ = io.WriteString(w, payload)
	})

	stdout, stderr, code := runCLI(t, []string{"themes", "list", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || !reflect.DeepEqual(requests, []string{"GET /admin/website/themes.json"}) {
		t.Fatalf("json: %d %v %s %s", code, requests, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, payload)

	for _, check := range []struct{ flag, want string }{{"--count", "2\n"}, {"--ids-only", "limelight\nbokeh\n"}} {
		stdout, _, code = runCLI(t, []string{"themes", "list", check.flag}, "", env, nil)
		if code != 0 || stdout != check.want {
			t.Fatalf("%s: %d %q", check.flag, code, stdout)
		}
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"--styled", "themes", "list", "--category", "theater"}, "", env, nil)
	if code != 0 || !reflect.DeepEqual(requests, []string{"GET /admin/website/themes.json?category=theater"}) {
		t.Fatalf("category: %d %v %s", code, requests, stdout)
	}
	for _, want := range []string{
		"limelight  Limelight\n",
		"   Big pictures, clean edges, the events right at the top.\n",
		"   Made for  theater, comedy\n",
		"   Look      custom, sora / work_sans, accent #E0105A\n",
		"   Designs   home, events\n",
		"   Pages     Gallery\n",
		"bokeh  Bokeh\n   Made for  club\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("styled output %q does not contain %q", stdout, want)
		}
	}
	if strings.Contains(stdout, "poster wall") {
		t.Fatalf("the list leaves the highlights to show: %q", stdout)
	}
}

func TestThemesShowSaysWhatItDoesBestAndHowToSetItUp(t *testing.T) {
	var requests []string
	status := http.StatusOK
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = io.WriteString(w, themeJSON)
		}
	})

	stdout, stderr, code := runCLI(t, []string{"themes", "show", "limelight", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || !reflect.DeepEqual(requests, []string{"GET /admin/website/themes/limelight.json"}) {
		t.Fatalf("show: %d %v %s %s", code, requests, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, themeJSON)

	stdout, _, code = runCLI(t, []string{"--styled", "themes", "show", "limelight"}, "", env, nil)
	for _, want := range []string{
		"limelight  Limelight\n",
		"   The programme as a poster wall: Every event with flyer, date and price in a row to browse.\n",
		"Set it up: usetix design-versions create --theme limelight\n",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("styled output %q does not contain %q", stdout, want)
		}
	}

	status = http.StatusNotFound
	stdout, _, code = runCLI(t, []string{"themes", "show", "nowhere", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"ok": false`) {
		t.Fatalf("a key Usetix doesn't offer: %d %s", code, stdout)
	}
}

func TestDesignVersionsCreateStartsAsAThemeAndSaysWhenTheThemeMovedOn(t *testing.T) {
	started := strings.Replace(designVersionJSON, `"shop_theme":null`, `"shop_theme":"limelight"`, 1)
	renewed := strings.Replace(started, `"shop_theme_renewed":false`, `"shop_theme_renewed":true`, 1)
	answer := started
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = io.WriteString(w, answer)
	})

	stdout, stderr, code := runCLI(t, []string{"design-versions", "create", "--theme", "limelight", "--name", "Spring programme", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || !reflect.DeepEqual(requests, []string{"POST /admin/website/versions.json"}) {
		t.Fatalf("create: %d %v %s %s", code, requests, stdout, stderr)
	}
	if !reflect.DeepEqual(body, map[string]any{"shop_version": map[string]any{"name": "Spring programme", "shop_theme": "limelight"}}) {
		t.Fatalf("body: %#v", body)
	}
	assertGuestListJSON(t, stdout, started)

	stdout, _, code = runCLI(t, []string{"--styled", "design-versions", "show", "4"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "   Theme    limelight\n") {
		t.Fatalf("a version names its theme: %d %q", code, stdout)
	}

	answer = renewed
	stdout, _, code = runCLI(t, []string{"--styled", "design-versions", "show", "4"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "   Theme    limelight, which has a newer release: design-versions create --theme limelight adds it as another version\n") {
		t.Fatalf("a version says its theme moved on: %d %q", code, stdout)
	}

	requests = nil
	for _, arguments := range [][]string{
		{"design-versions", "create", "--theme", "limelight", "--from", "3", "--json"},
		{"design-versions", "create", "--theme", " ", "--json"},
	} {
		stdout, _, code = runCLI(t, arguments, "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "--theme") {
			t.Fatalf("%v: %d %v %s", arguments, code, requests, stdout)
		}
	}
}
