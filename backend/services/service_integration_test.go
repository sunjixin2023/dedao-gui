//go:build integration

package services

import (
	"os"
	"testing"
)

func TestIntegrationUserProfile(t *testing.T) {
	cookie := os.Getenv("DEDAO_TEST_COOKIE")
	if cookie == "" {
		t.Skip("DEDAO_TEST_COOKIE is not set")
	}

	opts := CookieOptions{}
	if err := ParseCookies(cookie, &opts); err != nil {
		t.Fatalf("ParseCookies: %v", err)
	}

	user, err := NewService(&opts).User()
	if err != nil {
		t.Fatalf("User: %v", err)
	}

	if user == nil {
		t.Fatal("User: got nil user")
	}

	if user.UIDHazy == "" {
		t.Fatal("User: got empty UIDHazy")
	}
}
