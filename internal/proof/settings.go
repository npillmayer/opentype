package proof

import (
	"fmt"
	"strings"

	"golang.org/x/text/language"
)

type Settings struct {
	Preset    string `json:"preset,omitempty"`
	Script    string `json:"script,omitempty"`
	Language  string `json:"language,omitempty"`
	Direction string `json:"direction,omitempty"`
}

func (s Settings) Resolve() (Settings, error) {
	defaults := Settings{Script: "Latn", Language: "en", Direction: "ltr"}
	switch strings.ToLower(s.Preset) {
	case "", "latin":
	case "hebrew":
		defaults = Settings{Script: "Hebr", Language: "he", Direction: "rtl"}
	default:
		return s, fmt.Errorf("unknown preset %q (use latin or hebrew)", s.Preset)
	}
	if s.Script == "" {
		s.Script = defaults.Script
	}
	if s.Language == "" {
		s.Language = defaults.Language
	}
	if s.Direction == "" {
		s.Direction = defaults.Direction
	}
	script, err := language.ParseScript(s.Script)
	if err != nil {
		return s, fmt.Errorf("invalid script: %w", err)
	}
	s.Script = script.String()
	lang, err := language.Parse(s.Language)
	if err != nil {
		return s, fmt.Errorf("invalid language: %w", err)
	}
	s.Language = lang.String()
	s.Direction = strings.ToLower(s.Direction)
	switch s.Direction {
	case "ltr", "left-to-right":
		s.Direction = "ltr"
	case "rtl", "right-to-left":
		s.Direction = "rtl"
	default:
		return s, fmt.Errorf("unsupported direction %q", s.Direction)
	}
	s.Preset = "" // resolved snapshots are explicit, independent of preset defaults
	return s, nil
}
