package utils

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestM3u8URLsResolvesMediaSegments(t *testing.T) {
	var server *httptest.Server
	requestPaths := make(chan string, 1)
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths <- r.URL.Path
		if r.URL.Path != "/media/index.m3u8" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n# a comment\nsegment-1.ts\n" + server.URL + "/absolute.ts\n"))
	}))
	defer server.Close()

	urls, err := M3u8URLs(server.URL + "/media/index.m3u8")
	if err != nil {
		t.Fatalf("M3u8URLs returned error: %v", err)
	}
	if got := <-requestPaths; got != "/media/index.m3u8" {
		t.Fatalf("unexpected request path: %s", got)
	}

	expected := []string{
		server.URL + "/media/segment-1.ts",
		server.URL + "/absolute.ts",
	}
	if !reflect.DeepEqual(urls, expected) {
		t.Fatalf("unexpected urls: got %v want %v", urls, expected)
	}
}

func TestM3u8URLsRejectsEmptyAddress(t *testing.T) {
	if _, err := M3u8URLs(""); err == nil {
		t.Fatal("expected error for empty address")
	}
}
