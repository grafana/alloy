package metadata

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Metadata struct {
	Name         string        `yaml:"name"`
	Platforms    []Platform    `yaml:"platforms"`
	Requirements []Requirement `yaml:"requirements"`
}

type Requirement struct {
	Description string `yaml:"description"`
	Reference   string `yaml:"reference"`
}

type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformWindows Platform = "windows"
	PlatformDarwin  Platform = "darwin"
	PlatformFreeBSD Platform = "freebsd"
)

var validPlatforms = map[Platform]bool{
	PlatformLinux:   true,
	PlatformWindows: true,
	PlatformDarwin:  true,
	PlatformFreeBSD: true,
}

func (p *Platform) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	platform := Platform(s)
	if !validPlatforms[platform] {
		return fmt.Errorf("line %d: invalid platform %q (must be one of: linux, windows, darwin, freebsd)", value.Line, s)
	}
	*p = platform
	return nil
}
