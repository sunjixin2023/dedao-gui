package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseCookiesPreservesEqualsInValue(t *testing.T) {
	var got CookieOptions
	if err := ParseCookies("token=alpha=beta==; csrfToken=csrf-value", &got); err != nil {
		t.Fatal(err)
	}
	if got.Token != "alpha=beta==" {
		t.Fatalf("Token = %q", got.Token)
	}
	if got.CsrfToken != "csrf-value" {
		t.Fatalf("CsrfToken = %q", got.CsrfToken)
	}
}

func TestParseCookiesMatchesTagsCaseInsensitively(t *testing.T) {
	var got CookieOptions
	if err := ParseCookies("CSRFTOKEN=csrf-value; GAT=gat-value; _SID=session-id", &got); err != nil {
		t.Fatal(err)
	}
	if got.CsrfToken != "csrf-value" {
		t.Fatalf("CsrfToken = %q", got.CsrfToken)
	}
	if got.GAT != "gat-value" {
		t.Fatalf("GAT = %q", got.GAT)
	}
	if got.SID != "session-id" {
		t.Fatalf("SID = %q", got.SID)
	}
}

func TestParseCookiesSupportsMapOutput(t *testing.T) {
	var got map[string]string
	if err := ParseCookies("token=alpha=beta==; GAT=gat-value; isid=isid-value; csrfToken=csrf-value; ignored", &got); err != nil {
		t.Fatal(err)
	}
	if got["token"] != "alpha=beta==" {
		t.Fatalf("token = %q", got["token"])
	}
	if got["GAT"] != "gat-value" {
		t.Fatalf("GAT = %q", got["GAT"])
	}
	if got["isid"] != "isid-value" {
		t.Fatalf("isid = %q", got["isid"])
	}
	if got["csrfToken"] != "csrf-value" {
		t.Fatalf("csrfToken = %q", got["csrfToken"])
	}
	if got["ignored"] != "" {
		t.Fatalf("ignored = %q, want empty value", got["ignored"])
	}
}

func TestParseCookiesCaseCollisionsLastSeenWins(t *testing.T) {
	tests := []struct {
		name      string
		cookie    string
		wantToken string
		wantKey   string
	}{
		{
			name:      "canonical name wins when later",
			cookie:    "token=first; TOKEN=second",
			wantToken: "second",
			wantKey:   "TOKEN",
		},
		{
			name:      "non canonical name wins when later",
			cookie:    "TOKEN=first; token=second",
			wantToken: "second",
			wantKey:   "token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got CookieOptions
			if err := ParseCookies(tt.cookie, &got); err != nil {
				t.Fatal(err)
			}
			if got.Token != tt.wantToken {
				t.Fatalf("Token = %q, want %q", got.Token, tt.wantToken)
			}

			var cookieMap map[string]string
			if err := ParseCookies(tt.cookie, &cookieMap); err != nil {
				t.Fatal(err)
			}
			if len(cookieMap) != 1 {
				t.Fatalf("len(map) = %d, want 1 (%#v)", len(cookieMap), cookieMap)
			}
			if cookieMap[tt.wantKey] != tt.wantToken {
				t.Fatalf("map[%q] = %q, want %q (%#v)", tt.wantKey, cookieMap[tt.wantKey], tt.wantToken, cookieMap)
			}
		})
	}
}

func TestParseCookiesMapOutputPreservesUnknownNames(t *testing.T) {
	var got map[string]string
	if err := ParseCookies("known=value; X-Custom=alpha; x-custom=beta; other_name=final", &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len(map) = %d, want 3 (%#v)", len(got), got)
	}
	if got["known"] != "value" {
		t.Fatalf("known = %q", got["known"])
	}
	if got["x-custom"] != "beta" {
		t.Fatalf("x-custom = %q, want beta", got["x-custom"])
	}
	if got["other_name"] != "final" {
		t.Fatalf("other_name = %q, want final", got["other_name"])
	}
	if _, exists := got["X-Custom"]; exists {
		t.Fatalf("stale collision key preserved: %#v", got)
	}
}

func TestParseCookiesRejectsUnsupportedTarget(t *testing.T) {
	var got struct {
		Token string `json:"token"`
	}
	err := ParseCookies("token=value", &got)
	if err == nil {
		t.Fatal("expected error for unsupported target")
	}
}

func TestCookieHeaderFromSetCookiesDropsAttributes(t *testing.T) {
	got, err := CookieHeaderFromSetCookies([]string{
		"csrfToken=csrf-value; Path=/; HttpOnly; Secure",
		"token=alpha=beta==; Path=/; SameSite=Lax",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "csrfToken=csrf-value; token=alpha=beta==" {
		t.Fatalf("header = %q", got)
	}

	for _, attribute := range []string{"Path", "Domain", "Expires", "HttpOnly", "Secure", "SameSite"} {
		if contains := containsString(got, attribute); contains {
			t.Fatalf("header %q unexpectedly contains attribute %q", got, attribute)
		}
	}
}

func TestCookieHeaderFromSetCookiesRejectsInvalidLines(t *testing.T) {
	invalid := "not-a-cookie"
	_, err := CookieHeaderFromSetCookies([]string{invalid})
	if err == nil {
		t.Fatal("expected invalid Set-Cookie error")
	}
	if containsString(err.Error(), invalid) {
		t.Fatalf("error %q should not include raw cookie input", err.Error())
	}
}

func TestMergeCookieHeadersUsesStableOrderAndReplacesValues(t *testing.T) {
	got := MergeCookieHeaders(
		"token=old-token; csrfToken=old-csrf; ignored=drop-me; _sid=old-session",
		"_sid=new-session; token=new-token; gat=gat-value; ISID=isid-value; also_ignored=drop-me-too",
	)
	want := "GAT=gat-value; ISID=isid-value; token=new-token; csrfToken=old-csrf; _sid=new-session"
	if got != want {
		t.Fatalf("merged header = %q, want %q", got, want)
	}
}

func TestCookieMapReturnsStructuredFields(t *testing.T) {
	got := CookieMap(&CookieOptions{
		GAT:       "gat-value",
		ISID:      "isid-value",
		Token:     "token-value",
		CsrfToken: "csrf-value",
	})
	want := map[string]string{
		"GAT":       "gat-value",
		"ISID":      "isid-value",
		"token":     "token-value",
		"csrfToken": "csrf-value",
	}
	if len(got) != len(want) {
		t.Fatalf("len(map) = %d, want %d (%#v)", len(got), len(want), got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s = %q, want %q", key, got[key], value)
		}
	}
}

func TestCookieOptionsJSONDoesNotIncludeCookieStr(t *testing.T) {
	data, err := json.Marshal(CookieOptions{
		Token:     "token-value",
		CsrfToken: "csrf-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if containsString(string(data), "cookieStr") {
		t.Fatalf("marshaled cookie options unexpectedly contain cookieStr: %s", data)
	}
}

func containsString(s, needle string) bool {
	return strings.Contains(s, needle)
}
