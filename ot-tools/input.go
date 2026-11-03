package main

import (
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/npillmayer/opentype/internal/proof"

	"github.com/thatisuday/commando"
)

func readToolInput(text string, flags map[string]commando.FlagValue) (string, error) {
	path, err := flags["input-file"].GetString()
	if err != nil {
		return "", err
	}
	fromStdin, err := flags["stdin"].GetBool()
	if err != nil {
		return "", err
	}
	cp, err := flags["codepoints"].GetString()
	if err != nil {
		return "", err
	}
	if cp == "-" {
		cp = ""
	}
	if fromStdin {
		if path != "" {
			return "", fmt.Errorf("choose --stdin or --input-file")
		}
		path = "-"
	}
	choices := 0
	for _, value := range []string{text, path, cp} {
		if value != "" {
			choices++
		}
	}
	if choices > 1 {
		return "", fmt.Errorf("choose one input source: text, --codepoints, --input-file or --stdin")
	}
	if cp != "" {
		runes, err := proof.ParseCodepoints(cp)
		return string(runes), err
	}
	if path != "" {
		var data []byte
		if path == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return "", fmt.Errorf("cannot read text input: %w", err)
		}
		text = string(data)
	}
	if !utf8.ValidString(text) {
		return "", fmt.Errorf("text input is not valid UTF-8")
	}
	return text, nil
}
