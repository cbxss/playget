package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cbxss/playget/internal/assets"
	"github.com/cbxss/playget/internal/device"
	"github.com/cbxss/playget/internal/play"
)

const defaultProfile = "pixel_7a"

var version = assets.Version()

func readVersion() string {
	if version == "" {
		return "0.0.0+unknown"
	}
	return version
}

type featureFlags []string

func (f *featureFlags) String() string {
	return fmt.Sprint([]string(*f))
}

func (f *featureFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	var extras featureFlags
	var versionCode int
	toolVersion := flag.Bool("tool-version", false, "print the playget tool version and exit")
	dumpConfig := flag.Bool("dump-device-config-json", false, "print deterministic device config JSON and exit")
	probeOnly := flag.Bool("probe-only", false, "resolve delivery metadata without downloading APK files")
	profileName := flag.String("profile", "auto", "device profile to use")
	outDir := flag.String("out", "", "output dir (default: play_out/<package>)")
	noCache := flag.Bool("no-cache", false, "disable the per-package profile cache")
	flag.IntVar(&versionCode, "version", 0, "versionCode (default: latest)")
	flag.IntVar(&versionCode, "v", 0, "versionCode (default: latest)")
	flag.Var(&extras, "extra-feature", "temporary Android feature to advertise; repeatable")
	flag.Parse()

	if *toolVersion {
		fmt.Printf("playget %s\n", readVersion())
		return
	}

	if *profileName != "auto" && *profileName != defaultProfile {
		fmt.Fprintf(os.Stderr, "unknown profile %q\n", *profileName)
		os.Exit(2)
	}

	profile, err := assets.DeviceProfile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load profile: %v\n", err)
		os.Exit(1)
	}

	if *dumpConfig {
		out, err := device.SummaryJSON(profile, extras)
		if err != nil {
			fmt.Fprintf(os.Stderr, "device config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		return
	}

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: playget [options] <package>")
		os.Exit(2)
	}
	pkg := flag.Arg(0)
	out := *outDir
	if out == "" {
		out = filepath.Join("play_out", pkg)
	}
	opts := play.Options{
		Package:      pkg,
		VersionCode:  versionCode,
		OutDir:       out,
		Profile:      profile,
		ProfileName:  *profileName,
		ExtraFeature: extras,
		UseCache:     !*noCache,
		Log:          os.Stderr,
	}
	ctx := context.Background()
	if *probeOnly {
		result, candidate, err := play.Resolve(ctx, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		payload := struct {
			Profile      string      `json:"profile"`
			ExtraFeature []string    `json:"extra_features"`
			Result       interface{} `json:"result"`
		}{
			Profile:      candidate.Name,
			ExtraFeature: candidate.Extra,
			Result:       result,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}
	done, err := play.Fetch(ctx, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("DONE: %s\n", done)
}
