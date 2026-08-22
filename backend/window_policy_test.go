package backend

import "testing"

func TestDesktopMacWindowPolicyIsOpaque(t *testing.T) {
	got := DesktopMacWindowPolicy()
	if got.WebviewIsTransparent || got.WindowIsTranslucent {
		t.Fatalf("opaque window required, got %+v", got)
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
