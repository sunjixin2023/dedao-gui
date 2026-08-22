package backend

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type MacWindowPolicy struct {
	WebviewIsTransparent bool
	WindowIsTranslucent  bool
}

func DesktopMacWindowPolicy() MacWindowPolicy {
	return MacWindowPolicy{
		WebviewIsTransparent: false,
		WindowIsTranslucent:  false,
	}
}

func focusExistingWindow(ctx context.Context) {
	if ctx == nil {
		return
	}
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
}
