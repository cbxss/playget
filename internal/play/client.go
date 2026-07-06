package play

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cbxss/playget/internal/device"
	"github.com/cbxss/playget/internal/playproto"
	"google.golang.org/protobuf/proto"
)

const (
	BaseURL        = "https://android.clients.google.com"
	DispenserURL   = "https://auroraoss.com/api/auth"
	DispenserUA    = "com.aurora.store-4.7.4"
	Locale         = "en_US"
	DefaultProfile = "pixel_7a"
	CacheVersion   = 1
)

const dfeTargets = "CAESN/qigQYC2AMBFfUbyA7SM5Ij/CvfBoIDgxHqGP8R3xzIBvoQtBKFDZ4HAY4FrwSVMasHBO0O2Q8akgYRAQECAQO7AQEpKZ0CnwECAwRrAQYBr9PPAoK7sQMBAQMCBAkIDAgBAwEDBAICBAUZEgMEBAMLAQEBBQEBAcYBARYED+cBfS8CHQEKkAEMMxcBIQoUDwYHIjd3DQ4MFk0JWGYZEREYAQOLAYEBFDMIEYMBAgICAgICOxkCD18LGQKEAcgDBIQBAgGLARkYCy8oBTJlBCUocxQn0QUBDkkGxgNZQq0BZSbeAmIDgAEBOgGtAaMCDAOQAZ4BBIEBKUtQUYYBQscDDxPSARA1oAEHAWmnAsMB2wFyywGLAxol+wImlwOOA80CtwN26A0WjwJVbQEJPAH+BRDeAfkHK/ABASEBCSAaHQemAzkaRiu2Ad8BdXeiAwEBGBUBBN4LEIABK4gB2AFLfwECAdoENq0CkQGMBsIBiQEtiwGgA1zyAUQ4uwS8AwhsvgPyAcEDF27vApsBHaICGhl3GSKxAR8MC6cBAgItmQYG9QIeywLvAeYBDArLAh8HASI4ELICDVmVBgsY/gHWARtcAsMBpALiAdsBA7QBpAJmIArpByn0AyAKBwHTARIHAX8D+AMBcRIBBbEDmwUBMacCHAciNp0BAQF0OgQLJDuSAh54kwFSP0eeAQQ4M5EBQgMEmwFXywFo0gFyWwMcapQBBugBPUW2AVgBKmy3AR6PAbMBGQxrUJECvQR+8gFoWDsYgQNwRSczBRXQAgtRswEW0ALMAREYAUEBIG6yATYCRE8OxgER8gMBvQEDRkwLc8MBTwHZAUOnAXiiBakDIbYBNNcCIUmuArIBSakBrgFHKs0EgwV/G3AD0wE6LgECtQJ4xQFwFbUCjQPkBS6vAQqEAUZF3QIM9wEhCoYCQhXsBCyZArQDugIziALWAdIBlQHwBdUErQE6qQaSA4EEIvYBHir9AQVLmgMCApsCKAwHuwgrENsBAjNYswEVmgIt7QJnN4wDEnta+wGfAcUBxgEtEFXQAQWdAUAeBcwBAQM7rAEJATJ0LENrdh73A6UBhAE+qwEeASxLZUMhDREuH0CGARbd7K0GlQo"

const dfePhenotype = "H4sIAAAAAAAAAB3OO3KjMAAA0KRNuWXukBkBQkAJ2MhgAZb5u2GCwQZbCH_EJ77QHmgvtDtbv-Z9_H63zXXU0NVPB1odlyGy7751Q3CitlPDvFd8lxhz3tpNmz7P92CFw73zdHU2Ie0Ad2kmR8lxhiErTFLt3RPGfJQHSDy7Clw10bg8kqf2owLokN4SecJTLoSwBnzQSd652_MOf2d1vKBNVedzg4ciPoLz2mQ8efGAgYeLou-l-PXn_7Sna1MfhHuySxt-4esulEDp8Sbq54CPPKjpANW-lkU2IZ0F92LBI-ukCKSptqeq1eXU96LD9nZfhKHdtjSWwJqUm_2r6pMHOxk01saVanmNopjX3YxQafC4iC6T55aRbC8nTI98AF_kItIQAJb5EQxnKTO7TZDWnr01HVPxelb9A2OWX6poidMWl16K54kcu_jhXw-JSBQkVcD_fPsLSZu6joIBAAA"

var autoFeatureOverlays = [][]string{{"android.software.companion_device_setup"}}

var featureRE = regexp.MustCompile(`(?m)^\s*uses-feature: name='([^']+)'`)

type Candidate struct {
	Name   string
	Extra  []string
	Source string
}

type File struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type DownloadedFile struct {
	Name string
	Path string
}

type ProbeResult struct {
	Title       string            `json:"title"`
	VersionCode int32             `json:"versionCode"`
	Files       []File            `json:"files"`
	Cookies     map[string]string `json:"-"`
}

type Cache struct {
	Version  int                   `json:"version"`
	Packages map[string]CacheEntry `json:"packages"`
}

type CacheEntry struct {
	Profile       string   `json:"profile"`
	ExtraFeatures []string `json:"extra_features"`
	Updated       int64    `json:"updated"`
}

type Options struct {
	Package      string
	VersionCode  int
	OutDir       string
	Profile      device.Profile
	ProfileName  string
	ExtraFeature []string
	UseCache     bool
	Retries      int
	Log          io.Writer
}

type PlayUnavailable struct {
	Title       string
	Restriction int32
}

func (e *PlayUnavailable) Error() string {
	return fmt.Sprintf("%s unavailable (availability restriction=%d)", e.Title, e.Restriction)
}

type EmptyDelivery struct{}

func (e *EmptyDelivery) Error() string {
	return "delivery returned no files"
}

type Client struct {
	HTTP    *http.Client
	Profile device.Profile
}

func NewClient(profile device.Profile) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 180 * time.Second},
		Profile: profile,
	}
}

func CachePath() string {
	if path := os.Getenv("PLAYGET_CACHE"); path != "" {
		return path
	}
	root := os.Getenv("XDG_CACHE_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			root = filepath.Join(home, ".cache")
		}
	}
	if root == "" {
		return "profile-cache.json"
	}
	return filepath.Join(root, "playget", "profile-cache.json")
}

func ReadCache(enabled bool) Cache {
	cache := Cache{Version: CacheVersion, Packages: map[string]CacheEntry{}}
	if !enabled {
		return cache
	}
	data, err := os.ReadFile(CachePath())
	if err != nil {
		return cache
	}
	if err := json.Unmarshal(data, &cache); err != nil || cache.Version != CacheVersion || cache.Packages == nil {
		return Cache{Version: CacheVersion, Packages: map[string]CacheEntry{}}
	}
	return cache
}

func WriteCache(cache Cache, enabled bool) error {
	if !enabled {
		return nil
	}
	path := CachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Unique(values []string) []string {
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

func ProfileLabel(name string, extra []string) string {
	if len(extra) == 0 {
		return name
	}
	return name + "+" + strings.Join(extra, "+")
}

func ProfileCandidates(pkg, profileName string, cliExtra []string, cache Cache, useCache bool) ([]Candidate, error) {
	if profileName == "" {
		profileName = "auto"
	}
	if profileName != "auto" && profileName != DefaultProfile {
		return nil, fmt.Errorf("unknown profile %q", profileName)
	}
	if profileName != "auto" {
		return []Candidate{{Name: profileName, Extra: Unique(cliExtra), Source: "cli"}}, nil
	}

	var candidates []Candidate
	seen := map[string]bool{}
	add := func(candidate Candidate) {
		candidate.Extra = Unique(candidate.Extra)
		key := candidate.Name + "\x00" + strings.Join(candidate.Extra, "\x00")
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, candidate)
	}
	if useCache {
		if entry, ok := cache.Packages[pkg]; ok {
			extra := append([]string{}, entry.ExtraFeatures...)
			extra = append(extra, cliExtra...)
			name := entry.Profile
			if name == "" {
				name = DefaultProfile
			}
			add(Candidate{Name: name, Extra: extra, Source: "cache"})
		}
	}
	add(Candidate{Name: DefaultProfile, Extra: cliExtra, Source: "base"})
	for _, overlay := range autoFeatureOverlays {
		extra := append([]string{}, overlay...)
		extra = append(extra, cliExtra...)
		add(Candidate{Name: DefaultProfile, Extra: extra, Source: "auto"})
	}
	return candidates, nil
}

func (c *Client) androidBuild() (*playproto.AndroidBuildProto, error) {
	sdk, err := c.Profile.Int32("build.version.sdk_int")
	if err != nil {
		return nil, err
	}
	gsf, err := c.Profile.Int32("gsf.version")
	if err != nil {
		return nil, err
	}
	return &playproto.AndroidBuildProto{
		Id:             proto.String(c.Profile.Value("build.fingerprint")),
		Product:        proto.String(c.Profile.Value("build.hardware")),
		Carrier:        proto.String(c.Profile.Value("build.brand")),
		Radio:          proto.String(c.Profile.Value("build.radio")),
		Bootloader:     proto.String(c.Profile.Value("build.bootloader")),
		Device:         proto.String(c.Profile.Value("build.device")),
		SdkVersion:     proto.Int32(sdk),
		Model:          proto.String(c.Profile.Value("build.model")),
		Manufacturer:   proto.String(c.Profile.Value("build.manufacturer")),
		BuildProduct:   proto.String(c.Profile.Value("build.product")),
		Client:         proto.String(c.Profile.Value("client")),
		OtaInstalled:   proto.Bool(false),
		Timestamp:      proto.Int64(time.Now().Unix()),
		GoogleServices: proto.Int32(gsf),
	}, nil
}

func (c *Client) checkinRequest(extra []string) (*playproto.AndroidCheckinRequest, error) {
	build, err := c.androidBuild()
	if err != nil {
		return nil, err
	}
	cfg, err := device.DeviceConfig(c.Profile, extra)
	if err != nil {
		return nil, err
	}
	timezone := c.Profile.Value("timezone")
	if timezone == "" {
		timezone = "America/Los_Angeles"
	}
	return &playproto.AndroidCheckinRequest{
		Id: proto.Int64(0),
		Checkin: &playproto.AndroidCheckinProto{
			Build:           build,
			LastCheckinMsec: proto.Int64(0),
			CellOperator:    proto.String(c.Profile.Value("celloperator")),
			SimOperator:     proto.String(c.Profile.Value("simoperator")),
			Roaming:         proto.String(c.Profile.Value("roaming")),
			UserNumber:      proto.Int32(0),
		},
		Locale:              proto.String(Locale),
		TimeZone:            proto.String(timezone),
		Version:             proto.Int32(3),
		DeviceConfiguration: cfg,
		Fragment:            proto.Int32(0),
	}, nil
}

func (c *Client) Checkin(ctx context.Context, extra []string) (string, string, error) {
	reqMsg, err := c.checkinRequest(extra)
	if err != nil {
		return "", "", err
	}
	body, err := proto.Marshal(reqMsg)
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/checkin", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("app", "com.google.android.gms")
	req.Header.Set("User-Agent", fmt.Sprintf("GoogleAuth/1.4 (%s %s)", c.Profile.Value("build.device"), c.Profile.Value("build.id")))
	req.Header.Set("Content-Type", "application/x-protobuffer")
	req.Header.Set("Host", "android.clients.google.com")
	data, err := c.do(req)
	if err != nil {
		return "", "", err
	}
	var resp playproto.AndroidCheckinResponse
	if err := proto.Unmarshal(data, &resp); err != nil {
		return "", "", err
	}
	return fmt.Sprintf("%x", resp.GetAndroidId()), resp.GetDeviceCheckinConsistencyToken(), nil
}

func (c *Client) UploadDeviceConfig(ctx context.Context, token, gsfid, checkinToken string, extra []string) (string, error) {
	cfg, err := device.DeviceConfig(c.Profile, extra)
	if err != nil {
		return "", err
	}
	body, err := proto.Marshal(&playproto.UploadDeviceConfigRequest{DeviceConfiguration: cfg})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/fdfe/uploadDeviceConfig", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	applyHeaders(req, c.fdfeHeaders(token, gsfid, checkinToken, ""))
	data, err := c.do(req)
	if err != nil {
		return "", err
	}
	wrapper, err := parseWrapper(data)
	if err != nil {
		return "", err
	}
	return wrapper.GetPayload().GetUploadDeviceConfigResponse().GetUploadDeviceConfigToken(), nil
}

func (c *Client) AppDetails(ctx context.Context, token, gsfid, checkinToken, configToken, pkg string) (int32, string, error) {
	values := url.Values{"doc": []string{pkg}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+"/fdfe/details?"+values.Encode(), nil)
	if err != nil {
		return 0, "", err
	}
	applyHeaders(req, c.fdfeHeaders(token, gsfid, checkinToken, configToken))
	data, err := c.do(req)
	if err != nil {
		return 0, "", err
	}
	wrapper, err := parseWrapper(data)
	if err != nil {
		return 0, "", err
	}
	doc := wrapper.GetPayload().GetDetailsResponse().GetDocV2()
	title := doc.GetTitle()
	if title == "" {
		title = pkg
	}
	versionCode := doc.GetDetails().GetAppDetails().GetVersionCode()
	if versionCode == 0 {
		return 0, title, &PlayUnavailable{Title: title, Restriction: doc.GetAvailability().GetRestriction()}
	}
	return versionCode, title, nil
}

func (c *Client) DeliveryToken(ctx context.Context, token, gsfid, checkinToken, configToken, pkg string, versionCode int32) (string, error) {
	values := url.Values{"ot": []string{"1"}, "doc": []string{pkg}, "vc": []string{fmt.Sprint(versionCode)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/fdfe/purchase?"+values.Encode(), strings.NewReader(""))
	if err != nil {
		return "", err
	}
	applyHeaders(req, c.fdfeHeaders(token, gsfid, checkinToken, configToken))
	data, err := c.do(req)
	if err != nil {
		return "", err
	}
	wrapper, err := parseWrapper(data)
	if err != nil {
		return "", err
	}
	return wrapper.GetPayload().GetBuyResponse().GetDownloadToken(), nil
}

func (c *Client) Delivery(ctx context.Context, token, gsfid, checkinToken, configToken, pkg string, versionCode int32, deliveryToken string) ([]File, map[string]string, error) {
	values := url.Values{"ot": []string{"1"}, "doc": []string{pkg}, "vc": []string{fmt.Sprint(versionCode)}}
	if deliveryToken != "" {
		values.Set("dtok", deliveryToken)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+"/fdfe/delivery?"+values.Encode(), nil)
	if err != nil {
		return nil, nil, err
	}
	applyHeaders(req, c.fdfeHeaders(token, gsfid, checkinToken, configToken))
	data, err := c.do(req)
	if err != nil {
		return nil, nil, err
	}
	wrapper, err := parseWrapper(data)
	if err != nil {
		return nil, nil, err
	}
	delivery := wrapper.GetPayload().GetDeliveryResponse().GetAppDeliveryData()
	var files []File
	if delivery.GetDownloadUrl() != "" {
		files = append(files, File{Name: "base.apk", URL: delivery.GetDownloadUrl()})
	}
	for _, split := range delivery.GetSplit() {
		name := split.GetName()
		if !strings.HasSuffix(name, ".apk") {
			name += ".apk"
		}
		files = append(files, File{Name: name, URL: split.GetDownloadUrl()})
	}
	cookies := map[string]string{}
	for _, cookie := range delivery.GetDownloadAuthCookie() {
		cookies[cookie.GetName()] = cookie.GetValue()
	}
	return files, cookies, nil
}

func (c *Client) Probe(ctx context.Context, token, pkg string, requestedVersion int, candidate Candidate) (*ProbeResult, error) {
	gsfid, checkinToken, err := c.Checkin(ctx, candidate.Extra)
	if err != nil {
		return nil, err
	}
	configToken, err := c.UploadDeviceConfig(ctx, token, gsfid, checkinToken, candidate.Extra)
	if err != nil {
		return nil, err
	}
	latestVersion, title, err := c.AppDetails(ctx, token, gsfid, checkinToken, configToken, pkg)
	if err != nil {
		return nil, err
	}
	versionCode := latestVersion
	if requestedVersion != 0 {
		versionCode = int32(requestedVersion)
	}
	deliveryToken, err := c.DeliveryToken(ctx, token, gsfid, checkinToken, configToken, pkg, versionCode)
	if err != nil {
		return nil, err
	}
	files, cookies, err := c.Delivery(ctx, token, gsfid, checkinToken, configToken, pkg, versionCode, deliveryToken)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, &EmptyDelivery{}
	}
	return &ProbeResult{Title: title, VersionCode: versionCode, Files: files, Cookies: cookies}, nil
}

func Dispenser(ctx context.Context, client *http.Client) (string, string, error) {
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DispenserURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", DispenserUA)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("GET %s: %s: %s", DispenserURL, resp.Status, strings.TrimSpace(string(data)))
	}
	var out struct {
		Email string `json:"email"`
		Auth  string `json:"auth"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", "", err
	}
	if out.Auth == "" {
		return "", "", errors.New("dispenser returned empty auth token")
	}
	return out.Email, out.Auth, nil
}

func Resolve(ctx context.Context, opts Options) (*ProbeResult, Candidate, error) {
	if opts.Retries <= 0 {
		opts.Retries = 4
	}
	cache := ReadCache(opts.UseCache)
	candidates, err := ProfileCandidates(opts.Package, opts.ProfileName, opts.ExtraFeature, cache, opts.UseCache)
	if err != nil {
		return nil, Candidate{}, err
	}
	var last error
	for attempt := 1; attempt <= opts.Retries; attempt++ {
		email, token, err := Dispenser(ctx, nil)
		if err != nil {
			last = err
			logf(opts.Log, "[!] attempt %d failed: %s\n", attempt, err)
			time.Sleep(1500 * time.Millisecond)
			continue
		}
		logf(opts.Log, "[*] dispenser: %s\n", email)
		var profileErrors []string
		for _, candidate := range candidates {
			logf(opts.Log, "[*] trying profile: %s\n", ProfileLabel(candidate.Name, candidate.Extra))
			client := NewClient(opts.Profile)
			result, err := client.Probe(ctx, token, opts.Package, opts.VersionCode, candidate)
			if err == nil {
				logf(opts.Log, "[*] %s  versionCode=%d\n", result.Title, result.VersionCode)
				logf(opts.Log, "[*] files: %d\n", len(result.Files))
				return result, candidate, nil
			}
			var unavailable *PlayUnavailable
			var empty *EmptyDelivery
			if errors.As(err, &unavailable) || errors.As(err, &empty) {
				profileErrors = append(profileErrors, fmt.Sprintf("%s: %s", ProfileLabel(candidate.Name, candidate.Extra), err))
				logf(opts.Log, "[!] profile failed: %s: %s\n", ProfileLabel(candidate.Name, candidate.Extra), err)
				continue
			}
			last = err
			break
		}
		if len(profileErrors) > 0 {
			last = fmt.Errorf("no working profile; tried %s", strings.Join(profileErrors, "; "))
		}
		logf(opts.Log, "[!] attempt %d failed: %s\n", attempt, last)
		time.Sleep(1500 * time.Millisecond)
	}
	return nil, Candidate{}, fmt.Errorf("all %d attempts failed: %w", opts.Retries, last)
}

func Fetch(ctx context.Context, opts Options) (string, error) {
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join("play_out", opts.Package)
	}
	result, candidate, err := Resolve(ctx, opts)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return "", err
	}
	var downloaded []DownloadedFile
	client := NewClient(opts.Profile)
	for _, file := range result.Files {
		dest := filepath.Join(opts.OutDir, file.Name)
		got, err := client.Download(ctx, file.URL, result.Cookies, dest)
		if err != nil {
			return "", err
		}
		downloaded = append(downloaded, DownloadedFile{Name: file.Name, Path: dest})
		logf(opts.Log, "    -> %s (%d bytes)\n", file.Name, got)
	}
	cache := ReadCache(opts.UseCache)
	if err := UpdateProfileCache(ctx, cache, opts.Package, opts.Profile, candidate, downloaded, opts.UseCache); err != nil {
		return "", err
	}
	if len(candidate.Extra) > 0 {
		logf(opts.Log, "[*] cached profile %s for %s\n", ProfileLabel(candidate.Name, candidate.Extra), opts.Package)
	}
	return opts.OutDir, nil
}

func (c *Client) Download(ctx context.Context, rawURL string, cookies map[string]string, dest string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	for name, value := range cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("GET %s: %s: %s", rawURL, resp.Status, strings.TrimSpace(string(data)))
	}
	fh, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer fh.Close()
	return io.Copy(fh, resp.Body)
}

func UpdateProfileCache(ctx context.Context, cache Cache, pkg string, profile device.Profile, candidate Candidate, downloaded []DownloadedFile, enabled bool) error {
	if !enabled {
		return nil
	}
	extra := append([]string{}, candidate.Extra...)
	var baseAPK string
	for _, file := range downloaded {
		if file.Name == "base.apk" {
			baseAPK = file.Path
			break
		}
	}
	for _, feature := range learnRequiredFeatures(ctx, baseAPK) {
		if !contains(deviceFeatures(profile, extra), feature) {
			extra = append(extra, feature)
		}
	}
	if cache.Packages == nil {
		cache.Packages = map[string]CacheEntry{}
	}
	cache.Packages[pkg] = CacheEntry{
		Profile:       candidate.Name,
		ExtraFeatures: Unique(extra),
		Updated:       time.Now().Unix(),
	}
	return WriteCache(cache, enabled)
}

func learnRequiredFeatures(ctx context.Context, baseAPK string) []string {
	if baseAPK == "" {
		return nil
	}
	aapt, err := exec.LookPath("aapt")
	if err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, aapt, "dump", "badging", baseAPK).Output()
	if err != nil {
		return nil
	}
	matches := featureRE.FindAllStringSubmatch(string(out), -1)
	features := make([]string, 0, len(matches))
	for _, match := range matches {
		features = append(features, match[1])
	}
	sort.Strings(features)
	return Unique(features)
}

func deviceFeatures(profile device.Profile, extra []string) []string {
	features := append([]string{}, profile.Split("features")...)
	features = append(features, extra...)
	return Unique(features)
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func (c *Client) fdfeHeaders(token, gsfid, checkinToken, configToken string) map[string]string {
	headers := map[string]string{
		"Authorization":               "Bearer " + token,
		"User-Agent":                  device.UserAgent(c.Profile),
		"X-DFE-Device-Id":             gsfid,
		"Accept-Language":             strings.ReplaceAll(Locale, "_", "-"),
		"X-DFE-Encoded-Targets":       dfeTargets,
		"X-DFE-Phenotype":             dfePhenotype,
		"X-DFE-Client-Id":             "am-android-google",
		"X-DFE-Network-Type":          "4",
		"X-DFE-Content-Filters":       "",
		"X-Limit-Ad-Tracking-Enabled": "false",
		"X-Ad-Id":                     "",
		"X-DFE-UserLanguages":         Locale,
		"X-DFE-Request-Params":        "timeoutMs=4000",
		"X-DFE-MCCMNC":                c.Profile.Value("simoperator"),
	}
	if checkinToken != "" {
		headers["X-DFE-Device-Checkin-Consistency-Token"] = checkinToken
	}
	if configToken != "" {
		headers["X-DFE-Device-Config-Token"] = configToken
	}
	return headers
}

func applyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

func parseWrapper(data []byte) (*playproto.ResponseWrapper, error) {
	var wrapper playproto.ResponseWrapper
	if err := proto.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper, nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: %s: %s", req.Method, req.URL, resp.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func logf(w io.Writer, format string, args ...interface{}) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, format, args...)
}
