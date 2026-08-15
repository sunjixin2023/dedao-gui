package backend

import "testing"

func TestAppVersionReturnsBuildVersion(t *testing.T) {
	original := BuildVersion
	t.Cleanup(func() { BuildVersion = original })

	BuildVersion = "1.2.3"

	if got := NewApp().AppVersion(); got != "1.2.3" {
		t.Fatalf("AppVersion() = %q", got)
	}
}
