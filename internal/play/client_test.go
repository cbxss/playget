package play

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cbxss/playget/internal/assets"
	"github.com/cbxss/playget/internal/device"
)

const (
	withingsPkg    = "com.withings.wiscale2"
	iataPkg        = "org.iata.iataconnect"
	companionSetup = "android.software.companion_device_setup"
)

func testProfile(t *testing.T) device.Profile {
	t.Helper()
	profile, err := assets.DeviceProfile()
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestProfileCandidatesAutoIncludesOverlay(t *testing.T) {
	cache := Cache{Version: CacheVersion, Packages: map[string]CacheEntry{}}
	candidates, err := ProfileCandidates(withingsPkg, "auto", nil, cache, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) < 2 {
		t.Fatalf("expected base and overlay candidates, got %d", len(candidates))
	}
	if candidates[0].Name != DefaultProfile || len(candidates[0].Extra) != 0 {
		t.Fatalf("first candidate should be base profile: %+v", candidates[0])
	}
	found := false
	for _, candidate := range candidates {
		for _, feature := range candidate.Extra {
			if feature == companionSetup {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("auto candidates did not include %s", companionSetup)
	}
}

func TestCheckinRequestEmbedsDeviceConfig(t *testing.T) {
	client := NewClient(testProfile(t))
	req, err := client.checkinRequest([]string{companionSetup})
	if err != nil {
		t.Fatal(err)
	}
	if req.GetLocale() != Locale {
		t.Fatalf("locale = %q", req.GetLocale())
	}
	found := false
	for _, feature := range req.GetDeviceConfiguration().GetSystemAvailableFeature() {
		if feature == companionSetup {
			found = true
		}
	}
	if !found {
		t.Fatalf("checkin request missing overlay feature")
	}
}

func TestLiveCheckinAndDeviceConfigUpload(t *testing.T) {
	liveOnly(t)
	ctx := context.Background()
	token := liveToken(t, ctx)
	client := NewClient(testProfile(t))
	checkin, err := retry(ctx, func() (struct {
		gsfid string
		token string
	}, error) {
		gsfid, checkinToken, err := client.Checkin(ctx, nil)
		return struct {
			gsfid string
			token string
		}{gsfid: gsfid, token: checkinToken}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if checkin.gsfid == "" || checkin.token == "" {
		t.Fatalf("empty checkin result: %+v", checkin)
	}
	configToken, err := retry(ctx, func() (string, error) {
		return client.UploadDeviceConfig(ctx, token, checkin.gsfid, checkin.token, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if configToken == "" {
		t.Fatalf("empty config token")
	}
}

func TestLiveWithingsBaseProfileUnavailable(t *testing.T) {
	liveOnly(t)
	ctx := context.Background()
	token := liveToken(t, ctx)
	client := NewClient(testProfile(t))
	err := withUploadedConfig(ctx, client, token, nil, func(gsfid, checkinToken, configToken string) error {
		_, _, err := client.AppDetails(ctx, token, gsfid, checkinToken, configToken, withingsPkg)
		return err
	})
	var unavailable *PlayUnavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected PlayUnavailable, got %T: %v", err, err)
	}
	if unavailable.Restriction != 9 {
		t.Fatalf("restriction = %d, want 9", unavailable.Restriction)
	}
}

func TestLiveWithingsOverlayDeliveryMetadata(t *testing.T) {
	liveOnly(t)
	ctx := context.Background()
	token := liveToken(t, ctx)
	client := NewClient(testProfile(t))
	result, err := retry(ctx, func() (*ProbeResult, error) {
		return client.Probe(ctx, token, withingsPkg, 0, Candidate{Name: DefaultProfile, Extra: []string{companionSetup}, Source: "test"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "Withings" || result.VersionCode <= 0 || len(result.Files) != 4 {
		t.Fatalf("unexpected Withings result: %+v", result)
	}
}

func TestLiveIATABaseProfileDeliveryMetadata(t *testing.T) {
	liveOnly(t)
	ctx := context.Background()
	token := liveToken(t, ctx)
	client := NewClient(testProfile(t))
	result, err := retry(ctx, func() (*ProbeResult, error) {
		return client.Probe(ctx, token, iataPkg, 0, Candidate{Name: DefaultProfile, Source: "test"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "IATA Connect" || result.VersionCode != 78 || len(result.Files) != 3 {
		t.Fatalf("unexpected IATA result: %+v", result)
	}
}

func liveOnly(t *testing.T) {
	t.Helper()
	if os.Getenv("PLAYGET_LIVE") != "1" {
		t.Skip("set PLAYGET_LIVE=1 for live Play smoke tests")
	}
}

func liveToken(t *testing.T, ctx context.Context) string {
	t.Helper()
	auth, err := retry(ctx, func() (struct {
		email string
		token string
	}, error) {
		email, token, err := Dispenser(ctx, nil)
		return struct {
			email string
			token string
		}{email: email, token: token}, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if auth.email == "" || auth.token == "" {
		t.Fatalf("empty auth: %+v", auth)
	}
	return auth.token
}

func withUploadedConfig(ctx context.Context, client *Client, token string, extra []string, fn func(gsfid, checkinToken, configToken string) error) error {
	return retryErr(ctx, func() error {
		gsfid, checkinToken, err := client.Checkin(ctx, extra)
		if err != nil {
			return err
		}
		configToken, err := client.UploadDeviceConfig(ctx, token, gsfid, checkinToken, extra)
		if err != nil {
			return err
		}
		return fn(gsfid, checkinToken, configToken)
	})
}

func retryErr(ctx context.Context, fn func() error) error {
	_, err := retry(ctx, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func retry[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		value, err := fn()
		if err == nil {
			return value, nil
		}
		last = err
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return zero, last
}
