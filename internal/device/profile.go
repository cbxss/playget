package device

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cbxss/playget/internal/playproto"
	"google.golang.org/protobuf/proto"
)

type Profile map[string]string

type Summary struct {
	TouchScreen            int32    `json:"touchScreen"`
	Keyboard               int32    `json:"keyboard"`
	Navigation             int32    `json:"navigation"`
	ScreenLayout           int32    `json:"screenLayout"`
	HasHardKeyboard        bool     `json:"hasHardKeyboard"`
	HasFiveWayNavigation   bool     `json:"hasFiveWayNavigation"`
	ScreenDensity          int32    `json:"screenDensity"`
	ScreenWidth            int32    `json:"screenWidth"`
	ScreenHeight           int32    `json:"screenHeight"`
	GlEsVersion            int32    `json:"glEsVersion"`
	NativePlatform         []string `json:"nativePlatform"`
	SystemSharedLibrary    []string `json:"systemSharedLibrary"`
	SystemAvailableFeature []string `json:"systemAvailableFeature"`
	SystemSupportedLocale  []string `json:"systemSupportedLocale"`
	GlExtension            []string `json:"glExtension"`
	UserAgent              string   `json:"userAgent"`
}

func Load(path string) (Profile, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	profile := Profile{}
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		profile[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return profile, nil
}

func (p Profile) Value(key string) string {
	return p[strings.ToLower(key)]
}

func (p Profile) Bool(key string) bool {
	return p.Value(key) == "true"
}

func (p Profile) Int32(key string) (int32, error) {
	value := p.Value(key)
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an int: %w", key, value, err)
	}
	return int32(n), nil
}

func (p Profile) Split(key string) []string {
	var out []string
	for _, part := range strings.Split(p.Value(key), ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func unique(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func mergedFeatures(profile Profile, extra []string) []string {
	return unique(append(profile.Split("features"), extra...))
}

func DeviceConfig(profile Profile, extraFeatures []string) (*playproto.DeviceConfigurationProto, error) {
	touchScreen, err := profile.Int32("touchscreen")
	if err != nil {
		return nil, err
	}
	keyboard, err := profile.Int32("keyboard")
	if err != nil {
		return nil, err
	}
	navigation, err := profile.Int32("navigation")
	if err != nil {
		return nil, err
	}
	screenLayout, err := profile.Int32("screenlayout")
	if err != nil {
		return nil, err
	}
	screenDensity, err := profile.Int32("screen.density")
	if err != nil {
		return nil, err
	}
	screenWidth, err := profile.Int32("screen.width")
	if err != nil {
		return nil, err
	}
	screenHeight, err := profile.Int32("screen.height")
	if err != nil {
		return nil, err
	}
	glVersion, err := profile.Int32("gl.version")
	if err != nil {
		return nil, err
	}

	return &playproto.DeviceConfigurationProto{
		TouchScreen:            proto.Int32(touchScreen),
		Keyboard:               proto.Int32(keyboard),
		Navigation:             proto.Int32(navigation),
		ScreenLayout:           proto.Int32(screenLayout),
		HasHardKeyboard:        proto.Bool(profile.Bool("hashardkeyboard")),
		HasFiveWayNavigation:   proto.Bool(profile.Bool("hasfivewaynavigation")),
		ScreenDensity:          proto.Int32(screenDensity),
		ScreenWidth:            proto.Int32(screenWidth),
		ScreenHeight:           proto.Int32(screenHeight),
		GlEsVersion:            proto.Int32(glVersion),
		NativePlatform:         profile.Split("platforms"),
		SystemSharedLibrary:    profile.Split("sharedlibraries"),
		SystemAvailableFeature: mergedFeatures(profile, extraFeatures),
		SystemSupportedLocale:  profile.Split("locales"),
		GlExtension:            profile.Split("gl.extensions"),
	}, nil
}

func UserAgent(profile Profile) string {
	return fmt.Sprintf("Android-Finsky/%s (api=3,versionCode=%s,sdk=%s,device=%s,hardware=%s,product=%s,platformVersionRelease=%s,model=%s,buildId=%s,isWideScreen=0,supportedAbis=%s)",
		profile.Value("vending.versionstring"),
		profile.Value("vending.version"),
		profile.Value("build.version.sdk_int"),
		profile.Value("build.device"),
		profile.Value("build.hardware"),
		profile.Value("build.product"),
		profile.Value("build.version.release"),
		profile.Value("build.model"),
		profile.Value("build.id"),
		strings.ReplaceAll(profile.Value("platforms"), ",", ";"),
	)
}

func SummaryJSON(profile Profile, extraFeatures []string) ([]byte, error) {
	cfg, err := DeviceConfig(profile, extraFeatures)
	if err != nil {
		return nil, err
	}
	summary := Summary{
		TouchScreen:            cfg.GetTouchScreen(),
		Keyboard:               cfg.GetKeyboard(),
		Navigation:             cfg.GetNavigation(),
		ScreenLayout:           cfg.GetScreenLayout(),
		HasHardKeyboard:        cfg.GetHasHardKeyboard(),
		HasFiveWayNavigation:   cfg.GetHasFiveWayNavigation(),
		ScreenDensity:          cfg.GetScreenDensity(),
		ScreenWidth:            cfg.GetScreenWidth(),
		ScreenHeight:           cfg.GetScreenHeight(),
		GlEsVersion:            cfg.GetGlEsVersion(),
		NativePlatform:         cfg.GetNativePlatform(),
		SystemSharedLibrary:    cfg.GetSystemSharedLibrary(),
		SystemAvailableFeature: cfg.GetSystemAvailableFeature(),
		SystemSupportedLocale:  cfg.GetSystemSupportedLocale(),
		GlExtension:            cfg.GetGlExtension(),
		UserAgent:              UserAgent(profile),
	}
	return json.Marshal(summary)
}
