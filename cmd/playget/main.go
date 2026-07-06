package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/cbxss/playget/internal/device"
)

const defaultProfile = "pixel_7a"

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readVersion(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return "0.0.0+unknown"
	}
	return string(bytesTrimSpace(data))
}

func bytesTrimSpace(data []byte) []byte {
	start, end := 0, len(data)
	for start < end && (data[start] == ' ' || data[start] == '\n' || data[start] == '\t' || data[start] == '\r') {
		start++
	}
	for end > start && (data[end-1] == ' ' || data[end-1] == '\n' || data[end-1] == '\t' || data[end-1] == '\r') {
		end--
	}
	return data[start:end]
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
	toolVersion := flag.Bool("tool-version", false, "print the playget tool version and exit")
	dumpConfig := flag.Bool("dump-device-config-json", false, "print deterministic device config JSON and exit")
	profileName := flag.String("profile", "auto", "device profile to use")
	flag.Var(&extras, "extra-feature", "temporary Android feature to advertise; repeatable")
	flag.Parse()

	root := repoRoot()
	if *toolVersion {
		fmt.Printf("playget %s\n", readVersion(root))
		return
	}

	if *profileName != "auto" && *profileName != defaultProfile {
		fmt.Fprintf(os.Stderr, "unknown profile %q\n", *profileName)
		os.Exit(2)
	}

	profile, err := device.Load(filepath.Join(root, "device.properties"))
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

	fmt.Fprintln(os.Stderr, "native Go downloader is not protocol-complete yet; use playget.py until parity lands")
	os.Exit(2)
}
