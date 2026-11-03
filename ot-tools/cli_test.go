package main

import (
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the real command parser/actions in a child test process, since their
// error paths exit the process and commando owns global registration state.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("OT_TOOLS_CLI_TEST") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"ot-tools"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func TestCLI(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		cmd := exec.Command(exe, append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)...)
		cmd.Env = append(os.Environ(), "OT_TOOLS_CLI_TEST=1")
		b, err := cmd.CombinedOutput()
		return string(b), err
	}
	font := "../testdata/fonts/GentiumPlus-R.ttf"
	bare, err := run("shape", font, "office", "--features=liga")
	if err != nil {
		t.Fatal(bare, err)
	}
	explicit, err := run("shape", font, "office", "--features=+liga")
	if err != nil {
		t.Fatal(explicit, err)
	}
	if bare != explicit || !strings.Contains(bare, "741=1+1829") {
		t.Fatalf("feature command differs: %q / %q", bare, explicit)
	}
	disabled, err := run("shape", font, "office", "--features=liga=0")
	if err != nil || strings.Contains(disabled, "741=") {
		t.Fatalf("disable: %s %v", disabled, err)
	}
	for _, flags := range [][]string{{"--features", "-liga"}, {"--features=-liga"}, {"-f", "liga=0"}, {"-f=liga=0"}} {
		got, err := run(append([]string{"shape", font, "office"}, flags...)...)
		if err != nil || got != disabled {
			t.Fatalf("feature flags %v: %q %v; want %q", flags, got, err, disabled)
		}
	}
	out := filepath.Join(t.TempDir(), "proof.png")
	text, err := run("view", "../testdata/fonts/NotoSansHebrew-Regular.ttf", "--codepoints=05E9,05B8,05C1,05DC,05D5,05B9,05DD", "--script=Hebr", "--lang=he", "--direction=rtl", "--all", "--show-labels", "--show-baseline", "--show-origins", "--show-bboxes", "--flush=cluster", "--high-watermark=3500", "--low-watermark=1500", "--max-buffer=4096", "--output="+out)
	if err != nil {
		t.Fatal(text, err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil || img.Bounds().Dx() <= 48 || img.Bounds().Dy() <= 48 {
		t.Fatalf("PNG: %v %v", img, err)
	}
	badOut := filepath.Join(t.TempDir(), "bad.png")
	for _, args := range [][]string{
		{"view", font, "office", "--all", "--width=1", "--output=" + badOut},
		{"view", font, "office", "--index=999", "--output=" + badOut},
		{"view", font, "--codepoints=D800", "--output=" + badOut},
	} {
		output, err := run(args...)
		if err == nil || !strings.Contains(output, "ot-tools:") {
			t.Fatalf("expected diagnostic error: %v / %s", err, output)
		}
		if _, err := os.Stat(badOut); !os.IsNotExist(err) {
			t.Fatalf("failure wrote image: %v", err)
		}
	}
	info, err := run("font", font, "GSUB,GPOS")
	if err != nil || !strings.Contains(info, "Family: Gentium Plus") || !strings.Contains(info, "table GPOS:") {
		t.Fatalf("font command: %v / %s", err, info)
	}
}

func TestCLIFileInputPresetsAndCases(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func(stdin string, args ...string) (string, error) {
		cmd := exec.Command(exe, append([]string{"-test.run=^TestCLIProcess$", "--"}, args...)...)
		cmd.Env = append(os.Environ(), "OT_TOOLS_CLI_TEST=1")
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	font := "../testdata/fonts/GentiumPlus-R.ttf"
	text := "office cafe\u0301\n"
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	direct, err := run("", "shape", font, text)
	if err != nil {
		t.Fatal(direct, err)
	}
	for _, flags := range [][]string{{"--input-file", path}, {"--stdin"}, {"--input-file=-"}} {
		output, err := run(text, append([]string{"shape", font}, flags...)...)
		if err != nil || output != direct {
			t.Fatalf("file/stdin changed input: %v %q / %q", err, output, direct)
		}
	}
	out := filepath.Join(t.TempDir(), "full.png")
	output, err := run("", "view", font, "office", "--output="+out)
	if err != nil || !strings.Contains(output, "glyphs=4") {
		t.Fatalf("default full run: %s %v", output, err)
	}
	output, err = run("", "view", font, "office", "--index=0", "--output="+out)
	if err != nil || !strings.Contains(output, "glyphs=1") {
		t.Fatalf("explicit first glyph: %s %v", output, err)
	}
	output, err = run("", "view", "../testdata/fonts/NotoSansHebrew-Regular.ttf", "שָׁלוֹם", "--preset=hebrew", "--diagnostics", "--output="+out)
	if err != nil || !strings.Contains(output, "glyphs=7") {
		t.Fatalf("Hebrew diagnostics: %s %v", output, err)
	}
	if err := os.WriteFile(path, []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err = run("", "shape", font, "--input-file="+path)
	if err == nil || !strings.Contains(output, "UTF-8") {
		t.Fatalf("invalid input: %s %v", output, err)
	}
	output, err = run("", "shape", font, "office", "--input-file="+path)
	if err == nil || !strings.Contains(output, "one input source") {
		t.Fatalf("ambiguous input: %s %v", output, err)
	}
	prefix := filepath.Join(t.TempDir(), "comparison")
	output, err = run("", "proof", "../testdata/proof-cases/latin-features.json", "--output="+prefix)
	if err != nil {
		t.Fatal(output, err)
	}
	for _, suffix := range []string{".png", ".json", ".case.json"} {
		if _, err := os.Stat(prefix + suffix); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := os.ReadFile(prefix + ".case.json")
	if err != nil {
		t.Fatal(err)
	}
	// The automatic snapshot must not be overwritten when it is the input.
	output, err = run("", "proof", prefix+".case.json", "--output="+prefix)
	if err == nil || !strings.Contains(output, "overwrite the proof case") {
		t.Fatalf("overwrite guard: %s %v", output, err)
	}
	after, err := os.ReadFile(prefix + ".case.json")
	if err != nil || string(after) != string(snapshot) {
		t.Fatal("modified source case")
	}
}
