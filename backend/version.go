package backend

var BuildVersion = "0.0.0-dev"

func (a *App) AppVersion() string {
	return BuildVersion
}
