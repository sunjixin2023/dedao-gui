package backend

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yann0917/dedao-gui/backend/config"
	"github.com/yann0917/dedao-gui/backend/services"
)

type fakeSessionSource struct {
	activeUser     *config.Dedao
	loginUserCount int
	recovery       *config.RecoveryInfo
}

func (f fakeSessionSource) ActiveUser() *config.Dedao {
	return f.activeUser
}

func (f fakeSessionSource) LoginUserCount() int {
	return f.loginUserCount
}

func (f fakeSessionSource) Recovery() *config.RecoveryInfo {
	return f.recovery
}

func TestBuildSessionStatusLoggedOut(t *testing.T) {
	status := buildSessionStatus(fakeSessionSource{})
	if status.LoggedIn {
		t.Fatalf("expected logged out status, got loggedIn=%v", status.LoggedIn)
	}
	if status.User != nil {
		t.Fatalf("expected no display user when logged out, got %#v", status.User)
	}
	if status.Recovery != nil {
		t.Fatalf("expected no recovery metadata, got %#v", status.Recovery)
	}
}

func TestBuildSessionStatusUsesStoredDisplayUserWithMeaningfulToken(t *testing.T) {
	status := buildSessionStatus(fakeSessionSource{
		loginUserCount: 1,
		activeUser: &config.Dedao{
			User: config.User{
				UIDHazy: "user-1",
				Name:    "stored name",
				Avatar:  "https://example.com/avatar.png",
			},
			CookieOptions: services.CookieOptions{
				Token: "sensitive-cookie",
			},
		},
	})

	if !status.LoggedIn {
		t.Fatalf("expected logged in status")
	}
	if status.User == nil {
		t.Fatalf("expected display user")
	}
	if status.User.UIDHazy != "user-1" {
		t.Fatalf("expected uid_hazy to come from stored config, got %q", status.User.UIDHazy)
	}
	if status.User.Nickname != "stored name" {
		t.Fatalf("expected nickname to come from stored config name, got %q", status.User.Nickname)
	}
	if status.User.Avatar != "https://example.com/avatar.png" {
		t.Fatalf("expected avatar to come from stored config, got %q", status.User.Avatar)
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	assertNoSensitiveFields(t, data)
}

func TestBuildSessionStatusUsesStoredDisplayUserWithMeaningfulSID(t *testing.T) {
	status := buildSessionStatus(fakeSessionSource{
		loginUserCount: 1,
		activeUser: &config.Dedao{
			User: config.User{
				UIDHazy: "user-3",
				Name:    "stored sid user",
				Avatar:  "https://example.com/avatar-3.png",
			},
			CookieOptions: services.CookieOptions{
				SID: "session-id",
			},
		},
	})

	if !status.LoggedIn {
		t.Fatalf("expected logged in status when SID is present")
	}
	if status.User == nil || status.User.UIDHazy != "user-3" {
		t.Fatalf("expected display user from stored profile, got %#v", status.User)
	}
}

func TestBuildSessionStatusRequiresMeaningfulAuthMaterial(t *testing.T) {
	status := buildSessionStatus(fakeSessionSource{
		loginUserCount: 1,
		activeUser: &config.Dedao{
			User: config.User{
				UIDHazy: "legacy-user",
				Name:    "legacy name",
				Avatar:  "https://example.com/legacy.png",
			},
		},
	})

	if status.LoggedIn {
		t.Fatalf("expected legacy/restored profile without auth material to be logged out")
	}
	if status.User != nil {
		t.Fatalf("expected no public display user when auth material is absent, got %#v", status.User)
	}
}

func TestLoginResultJSONDoesNotContainCookie(t *testing.T) {
	result := LoginResult{
		Status: 1,
		User: &services.User{
			UIDHazy:  "user-2",
			Nickname: "visible user",
			Avatar:   "https://example.com/user.png",
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal login result: %v", err)
	}
	assertNoSensitiveFields(t, data)
}

func TestSessionStatusIncludesConfigRecovery(t *testing.T) {
	status := buildSessionStatus(fakeSessionSource{
		recovery: &config.RecoveryInfo{
			BackupPath: "/tmp/config.json.bak",
			Message:    "恢复成功，需要重新登录",
		},
	})

	if status.Recovery == nil {
		t.Fatalf("expected recovery info in session status")
	}
	if status.Recovery.BackupPath != "/tmp/config.json.bak" {
		t.Fatalf("unexpected backup path %q", status.Recovery.BackupPath)
	}
	if status.Recovery.Message != "恢复成功，需要重新登录" {
		t.Fatalf("unexpected recovery message %q", status.Recovery.Message)
	}
}

func assertNoSensitiveFields(t *testing.T, data []byte) {
	t.Helper()

	lower := strings.ToLower(string(data))
	for _, field := range []string{"cookie", "token", "csrf", "sid"} {
		if strings.Contains(lower, field) {
			t.Fatalf("unexpected sensitive field %q in json: %s", field, data)
		}
	}
}
