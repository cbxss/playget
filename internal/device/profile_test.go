package device

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeviceConfigOverlay(t *testing.T) {
	profile, err := Load(filepath.Join("..", "..", "device.properties"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := DeviceConfig(profile, []string{"android.software.companion_device_setup"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, feature := range cfg.GetSystemAvailableFeature() {
		if feature == "android.software.companion_device_setup" {
			found = true
		}
	}
	if !found {
		t.Fatalf("overlay feature missing from device config")
	}
	if got := UserAgent(profile); got == "" {
		t.Fatalf("empty user agent")
	}
}

func TestSummaryJSONMatchesFixture(t *testing.T) {
	profile, err := Load(filepath.Join("..", "..", "device.properties"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := SummaryJSON(profile, []string{"android.software.companion_device_setup"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0] != '{' {
		t.Fatalf("unexpected JSON: %q", string(got))
	}
	if os.Getenv("PLAYGET_UPDATE_FIXTURES") == "1" {
		t.Logf("%s", got)
	}
}
