// Package features declares the runtime capabilities this fest build advertises
// and lets callers query whether a capability is enabled.
package features

import (
	"os"
	"strings"
)

// MarketplaceExtensionSource is the capability of loading extensions from the
// obey-installer-managed marketplace path (~/.obey/fest/marketplace/extensions).
const MarketplaceExtensionSource = "marketplace_extension_source_v1"

// Supported lists every capability this fest build advertises. The obey
// installer reads this set (via fest version --json) to gate host-feature
// requirements for extensions it activates.
func Supported() []string {
	return []string{MarketplaceExtensionSource}
}

// Enabled reports whether a capability is active. The marketplace extension
// source defaults on (a missing marketplace dir is a harmless no-op for users
// who never run the installer) and can be turned off with
// FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE=0.
func Enabled(name string) bool {
	switch name {
	case MarketplaceExtensionSource:
		return envFlag("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", true)
	default:
		return false
	}
}

func envFlag(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}
