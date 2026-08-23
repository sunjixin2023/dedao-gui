package services

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestParseAllowedMediaURLAcceptsUmiwiHTTPS(t *testing.T) {
	got, err := parseAllowedMediaURL("https://bd-vod.umiwi.com/path/seg.m4s?sig=1")
	if err != nil {
		t.Fatalf("parseAllowedMediaURL() error = %v", err)
	}
	if got.Host != "bd-vod.umiwi.com" || got.Path != "/path/seg.m4s" {
		t.Fatalf("parsed = %s", got.String())
	}
}

func TestParseAllowedMediaURLRejectsUnknownHostsAndHTTP(t *testing.T) {
	if _, err := parseAllowedMediaURL("https://evil.example/x"); err == nil {
		t.Fatal("expected unknown host rejection")
	}
	if _, err := parseAllowedMediaURL("https://notumiwi.com/x"); err == nil {
		t.Fatal("expected lookalike host rejection")
	}
}

func TestParseAllowedMediaURLRewritesHTTPToHTTPS(t *testing.T) {
	got, err := parseAllowedMediaURL("http://bd-vod.umiwi.com/path/file.mpd?sig=1")
	if err != nil {
		t.Fatalf("parseAllowedMediaURL() error = %v", err)
	}
	if got.Scheme != "https" || got.Host != "bd-vod.umiwi.com" {
		t.Fatalf("parsed = %s", got.String())
	}
}

func TestAllowedMediaHostAcceptsVolcCDNs(t *testing.T) {
	if !allowedMediaHost("vod.volces.com") || !allowedMediaHost("cdn.volccdn.com") {
		t.Fatal("expected volc CDN hosts to be allowed")
	}
}

func TestShouldAppendPlaybackProbeDropsSuccessfulMediaHits(t *testing.T) {
	if shouldAppendPlaybackProbe(`{"kind":"media_proxy","ok":true,"host":"bd-vod.umiwi.com"}`) {
		t.Fatal("successful media_proxy lines should be dropped")
	}
	if !shouldAppendPlaybackProbe(`{"kind":"media_proxy","ok":false,"host":"bd-vod.umiwi.com"}`) {
		t.Fatal("failed media_proxy lines should be kept")
	}
	if !shouldAppendPlaybackProbe(`{"kind":"play_info","ok":true}`) {
		t.Fatal("play_info lines should be kept")
	}
}

func TestShouldRotatePlaybackProbeAfterCap(t *testing.T) {
	if shouldRotatePlaybackProbe(1024) {
		t.Fatal("small probe files should not rotate")
	}
	if !shouldRotatePlaybackProbe(300 << 10) {
		t.Fatal("probe files over 256KiB should rotate")
	}
}

func TestMediaGETForwardsRangeAndStatus(t *testing.T) {
	var gotRange string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRange = r.Header.Get("Range")
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/8")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("abcd"))
	}))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	got, err := mediaGET(parsed, "bytes=0-3")
	if err != nil {
		t.Fatalf("mediaGET() error = %v", err)
	}
	if gotRange != "bytes=0-3" {
		t.Fatalf("Range = %q", gotRange)
	}
	if got.Status != http.StatusPartialContent || got.BodyB64 == "" {
		t.Fatalf("result = %+v", got)
	}
}

func TestSummarizeVolcVodProxyBodyReportsAuthListWithoutCopyingContent(t *testing.T) {
	got := SummarizeVolcVodProxyBody(`{"ResponseMetadata":{"Action":"GetPrivateDrmPlayAuth","Error":null},"Result":{"PlayAuthInfoList":[{"PlayAuthContent":"secret-key"}]}}`)
	if !got.JSON || !got.HasResult || !got.HasPlayAuthList || got.PlayAuthCount != 1 || got.HasError {
		t.Fatalf("summary = %+v", got)
	}
	if strings.Contains(fmt.Sprintf("%+v", got), "secret-key") {
		t.Fatal("summary leaked PlayAuthContent")
	}
}
