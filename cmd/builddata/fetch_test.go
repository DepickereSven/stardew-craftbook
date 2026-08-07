package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchWikitext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "Modding:Recipe_data" {
			t.Errorf("wrong page param: %s", r.URL.Query().Get("page"))
		}
		w.Write([]byte(`{"parse":{"wikitext":{"*":"hello wikitext"}}}`))
	}))
	defer srv.Close()
	got, err := fetchWikitextFrom(srv.URL, "Modding:Recipe_data")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello wikitext" {
		t.Errorf("got %q", got)
	}
}

func TestFetchPages(t *testing.T) {
	const body = `{"query":{
		"normalized":[{"from":"omelet","to":"Omelet"}],
		"redirects":[{"from":"Cookies","to":"Cookie"}],
		"pages":{
			"1":{"title":"Omelet","revisions":[{"slots":{"main":{"*":"omelet text"}}}]},
			"2":{"title":"Cookie","revisions":[{"slots":{"main":{"*":"cookie text"}}}]},
			"3":{"title":"Ghost","missing":""}
		}}}`
	var gotTitles string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTitles = r.URL.Query().Get("titles")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	pages, err := fetchPagesFrom(srv.URL, []string{"omelet", "Cookies", "Ghost"})
	if err != nil {
		t.Fatal(err)
	}
	if gotTitles != "omelet|Cookies|Ghost" {
		t.Errorf("titles param = %q", gotTitles)
	}
	// Requested titles must resolve even when the wiki normalized or
	// redirected them, since callers look pages up by the title they asked for.
	if pages["omelet"] != "omelet text" {
		t.Errorf("normalized title lost: %q", pages["omelet"])
	}
	if pages["Cookies"] != "cookie text" {
		t.Errorf("redirected title lost: %q", pages["Cookies"])
	}
	if _, ok := pages["Ghost"]; ok {
		t.Error("missing page should be absent, not empty")
	}
}

func TestFetchPagesBatches(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"query":{"pages":{}}}`))
	}))
	defer srv.Close()

	titles := make([]string, 120)
	for i := range titles {
		titles[i] = fmt.Sprintf("P%d", i)
	}
	if _, err := fetchPagesFrom(srv.URL, titles); err != nil {
		t.Fatal(err)
	}
	if calls != 3 { // MediaWiki caps titles at 50 per request
		t.Errorf("made %d requests, want 3", calls)
	}
}
