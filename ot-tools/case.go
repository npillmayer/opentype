package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/npillmayer/opentype/internal/proof"

	"github.com/thatisuday/commando"
)

func readProofCase(path string) (proof.Case, error) {
	return readProofCaseWith(path, os.ReadFile)
}
func readProofCaseWith(path string, readFile func(string) ([]byte, error)) (proof.Case, error) {
	data, err := readFile(path)
	if err != nil {
		return proof.Case{}, err
	}
	c, err := proof.DecodeCase(data)
	if err != nil {
		return c, err
	}
	if (c.Text == nil) == (c.TextFile == "") {
		return c, fmt.Errorf("case requires exactly one of text or text_file")
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return c, err
	}
	if c.Font != "" && !filepath.IsAbs(c.Font) {
		c.Font = filepath.Join(base, c.Font)
	}
	if c.TextFile != "" {
		p := c.TextFile
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		data, err := readFile(p)
		if err != nil {
			return c, fmt.Errorf("read text_file: %w", err)
		}
		text := string(data)
		c.Text = &text
		c.TextFile = ""
	}
	return proof.NormalizeCase(c)
}
func prepareProofCase(path string) (proof.Result, *image.RGBA, error) {
	c, err := readProofCase(path)
	if err != nil {
		return proof.Result{}, nil, err
	}
	data, err := os.ReadFile(c.Font)
	if err != nil {
		return proof.Result{}, nil, fmt.Errorf("read font: %w", err)
	}
	execution, err := proof.Execute(c, data)
	if err != nil {
		return proof.Result{}, nil, err
	}
	return execution.Report, execution.Image, nil
}
func runProofCommand(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
	path := args["case"].Value
	prefix, err := flags["output"].GetString()
	if err != nil || strings.TrimSpace(prefix) == "" {
		fatalf("proof requires an output prefix")
	}
	inputAbs, err := filepath.Abs(path)
	if err != nil {
		fatalf("%v", err)
	}
	for _, suffix := range []string{".json", ".png", ".case.json"} {
		outputAbs, err := filepath.Abs(prefix + suffix)
		if err != nil {
			fatalf("%v", err)
		}
		if outputAbs == inputAbs {
			fatalf("output would overwrite the proof case; choose a different --output prefix")
		}
	}
	result, img, err := prepareProofCase(path)
	if err != nil {
		fatalf("proof failed: %v", err)
	}
	report, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fatalf("%v", err)
	}
	if err := writeProofPNG(prefix+".png", img); err != nil {
		fatalf("%v", err)
	}
	if err := os.WriteFile(prefix+".json", append(report, '\n'), 0o644); err != nil {
		fatalf("cannot write glyph report: %v", err)
	}
	snapshot, err := json.MarshalIndent(result.Case, "", "  ")
	if err != nil {
		fatalf("%v", err)
	}
	if err := os.WriteFile(prefix+".case.json", append(snapshot, '\n'), 0o644); err != nil {
		fatalf("cannot write resolved case: %v", err)
	}
	fmt.Printf("wrote %s.png, %s.json and %s.case.json (variants=%d)\n", prefix, prefix, prefix, len(result.Rows))
}
