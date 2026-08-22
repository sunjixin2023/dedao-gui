package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVolcSignedGETPreservesRawQuery(t *testing.T) {
	want := "Action=GetPlayInfo&Version=2020-08-01&Vid=video-1&X-SignedQueries=Action%3BVersion%3BVid&X-Signature=sig+plus"
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"1"},"Result":{}}`))
	}))
	t.Cleanup(server.Close)

	body, err := volcSignedGET(server.URL, want, nil)
	if err != nil {
		t.Fatalf("volcSignedGET() error = %v", err)
	}
	defer body.Close()
	if _, err := io.ReadAll(body); err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if gotQuery != want {
		t.Fatalf("RawQuery = %q, want %q", gotQuery, want)
	}
}

func TestVolcSignedGETRejectsNewlines(t *testing.T) {
	_, err := volcSignedGET("https://vod.example.invalid/", "Action=GetPlayInfo\nHost: evil", nil)
	if err == nil {
		t.Fatal("expected newline rejection")
	}
}

func TestVolcSignedGETMaps496(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(496)
	}))
	t.Cleanup(server.Close)
	_, err := volcSignedGET(server.URL, "Action=GetPlayInfo&Version=2020-08-01", nil)
	if err == nil || !strings.Contains(err.Error(), "496") {
		t.Fatalf("error = %v, want 496", err)
	}
}
