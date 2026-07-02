package features

import "testing"

func TestEnabled_MarketplaceExtensionSourceDefaultsOff(t *testing.T) {
	t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", "")

	if Enabled(MarketplaceExtensionSource) {
		t.Fatal("expected marketplace_extension_source_v1 to default off")
	}
}

func TestEnabled_MarketplaceExtensionSourceExplicitEnable(t *testing.T) {
	cases := []string{"1", "true", "on", "yes"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", v)

			if !Enabled(MarketplaceExtensionSource) {
				t.Fatalf("expected marketplace_extension_source_v1 to be enabled for %q", v)
			}
		})
	}
}

func TestEnabled_MarketplaceExtensionSourceExplicitDisable(t *testing.T) {
	cases := []string{"0", "false", "off", "no"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", v)

			if Enabled(MarketplaceExtensionSource) {
				t.Fatalf("expected marketplace_extension_source_v1 to stay disabled for %q", v)
			}
		})
	}
}

func TestEnabled_MarketplaceExtensionSourceUnrecognizedValueStaysOff(t *testing.T) {
	cases := []string{"maybe", "enabled", "disable", "2", "yeah"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			t.Setenv("FEST_FEATURE_MARKETPLACE_EXTENSION_SOURCE", v)

			if Enabled(MarketplaceExtensionSource) {
				t.Fatalf("expected an unrecognized value %q to fall back to the default (off), not enable the flag", v)
			}
		})
	}
}

func TestEnabled_UnknownCapability(t *testing.T) {
	if Enabled("not_a_real_capability") {
		t.Fatal("expected unknown capabilities to report disabled")
	}
}
