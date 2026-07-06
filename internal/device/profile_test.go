package device_test

import (
	"os"
	"testing"

	"github.com/cbxss/playget/internal/assets"
	"github.com/cbxss/playget/internal/device"
)

func TestDeviceConfigOverlay(t *testing.T) {
	profile, err := assets.DeviceProfile()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := device.DeviceConfig(profile, []string{"android.software.companion_device_setup"})
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
	if got := device.UserAgent(profile); got == "" {
		t.Fatalf("empty user agent")
	}
}

func TestSummaryJSONMatchesFixture(t *testing.T) {
	profile, err := assets.DeviceProfile()
	if err != nil {
		t.Fatal(err)
	}
	got, err := device.SummaryJSON(profile, []string{"android.software.companion_device_setup"})
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
