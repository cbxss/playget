package assets

import (
	"bytes"
	_ "embed"
	"strings"

	"github.com/cbxss/playget/internal/device"
)

//go:embed device.properties
var deviceProperties []byte

//go:embed VERSION
var version string

func DeviceProfile() (device.Profile, error) {
	return device.Parse(bytes.NewReader(deviceProperties))
}

func Version() string {
	return strings.TrimSpace(version)
}
