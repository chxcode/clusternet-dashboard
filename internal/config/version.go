package config

import "runtime/debug"

// Release builds set this value to the same tag that identifies the image.
var releaseVersion string

func BuildVersion() string {
	if releaseVersion != "" {
		return releaseVersion
	}
	version := "dev"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			version += "+" + setting.Value[:min(8, len(setting.Value))]
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if modified {
		version += ".dirty"
	}
	return version
}
