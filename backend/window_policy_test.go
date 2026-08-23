package backend

import "testing"

func TestDesktopMacWindowPolicyIsOpaque(t *testing.T) {
	got := DesktopMacWindowPolicy()
	if got.WebviewIsTransparent || got.WindowIsTranslucent {
		t.Fatalf("opaque window required, got %+v", got)
	}
}

func TestDesktopMacWindowPolicyEnablesElementFullscreen(t *testing.T) {
	got := DesktopMacWindowPolicy()
	if !got.FullscreenEnabled {
		t.Fatal("WKWebView element fullscreen must be enabled for VePlayer")
	}
}

func TestFocusExistingWindowNilContextDoesNotPanic(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Fatal("focusExistingWindow(nil) panicked")
		}
	}()
	focusExistingWindow(nil)
}
