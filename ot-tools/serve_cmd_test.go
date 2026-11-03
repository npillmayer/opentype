package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/npillmayer/opentype/internal/proof"
)

func testWorkbench() *workbench {
	return &workbench{host: "127.0.0.1:8080", token: "test-token", gate: make(chan struct{}, 1)}
}
func workbenchRequest(w *workbench, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "http://"+w.host+path, bytes.NewReader(body))
	req.Header.Set("Origin", "http://"+w.host)
	req.Header.Set("X-Workbench-Token", w.token)
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	w.ServeHTTP(out, req)
	return out
}
func TestWorkbenchSessionBoundary(t *testing.T) {
	w := testWorkbench()
	for _, c := range []struct {
		name, host, origin, token, method, content string
		code                                       int
	}{
		{"ok", w.host, "http://" + w.host, w.token, "POST", "application/json", 200},
		{"host", "evil.test", "http://" + w.host, w.token, "POST", "application/json", 403},
		{"origin", w.host, "https://evil.test", w.token, "POST", "application/json", 403},
		{"no origin", w.host, "", w.token, "POST", "application/json", 403},
		{"token", w.host, "http://" + w.host, "wrong", "POST", "application/json", 403},
		{"get", w.host, "http://" + w.host, w.token, "GET", "application/json", 405},
		{"form", w.host, "http://" + w.host, w.token, "POST", "text/plain", 415},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "http://"+c.host+"/api/init", strings.NewReader("{}"))
			req.Header.Set("Origin", c.origin)
			req.Header.Set("X-Workbench-Token", c.token)
			req.Header.Set("Content-Type", c.content)
			out := httptest.NewRecorder()
			w.ServeHTTP(out, req)
			if out.Code != c.code {
				t.Fatalf("status %d: %s", out.Code, out.Body)
			}
			if out.Header().Get("Access-Control-Allow-Origin") != "" || !strings.Contains(out.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Fatal("unsafe response headers")
			}
		})
	}
	for _, path := range []string{"/", "/app.js", "/source.mjs", "/updates.mjs", "/style.css"} {
		req := httptest.NewRequest("GET", "http://"+w.host+path, nil)
		out := httptest.NewRecorder()
		w.ServeHTTP(out, req)
		if out.Code != 200 || out.Body.Len() == 0 {
			t.Fatalf("asset %s: %d", path, out.Code)
		}
	}
	for _, path := range []string{"/../../go.mod", "/go.mod", "/workbench/../serve_cmd.go"} {
		req := httptest.NewRequest("GET", "http://"+w.host+path, nil)
		out := httptest.NewRecorder()
		w.ServeHTTP(out, req)
		if out.Code != 404 {
			t.Fatalf("unexpected file access %s", path)
		}
	}
}
func TestWorkbenchCasesAndExports(t *testing.T) {
	w := testWorkbench()
	for _, name := range []string{"latin-features", "hebrew-marks"} {
		t.Run(name, func(t *testing.T) {
			path := "../testdata/proof-cases/" + name + ".json"
			body, _ := json.Marshal(map[string]string{"path": path})
			opened := workbenchRequest(w, "/api/open", body)
			if opened.Code != 200 {
				t.Fatal(opened.Body.String())
			}
			preview := workbenchRequest(w, "/api/preview", opened.Body.Bytes())
			if preview.Code != 200 {
				t.Fatal(preview.Body.String())
			}
			var response struct {
				Report   proof.Result   `json:"report"`
				Geometry proof.Geometry `json:"geometry"`
				PNG      string         `json:"png"`
			}
			if err := json.Unmarshal(preview.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			cli, img, err := prepareProofCase(path)
			if err != nil {
				t.Fatal(err)
			}
			cliJSON, _ := json.Marshal(cli)
			httpJSON, _ := json.Marshal(response.Report)
			if !bytes.Equal(cliJSON, httpJSON) {
				t.Fatal("HTTP and CLI glyph report/case differ")
			}
			pixels, err := base64.StdEncoding.DecodeString(response.PNG)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := png.Decode(bytes.NewReader(pixels))
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Bounds() != img.Bounds() || response.Geometry.Width != img.Bounds().Dx() || response.Geometry.Height != img.Bounds().Dy() {
				t.Fatal("PNG/geometry dimensions differ")
			}
			var cliPNG bytes.Buffer
			if err := png.Encode(&cliPNG, img); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(pixels, cliPNG.Bytes()) {
				t.Fatal("HTTP PNG differs from CLI")
			}
			snapshot, _ := json.Marshal(response.Report.Case)
			saved := filepath.Join(t.TempDir(), "saved.case.json")
			if err := os.WriteFile(saved, snapshot, 0600); err != nil {
				t.Fatal(err)
			}
			replay, _, err := prepareProofCase(saved)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cli.Rows, replay.Rows) {
				t.Fatal("browser snapshot not CLI replayable")
			}
		})
	}
}
func TestWorkbenchValidationAndLimits(t *testing.T) {
	w := testWorkbench()
	c, err := readProofCase("../testdata/proof-cases/latin-features.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		change  func(*proof.Case)
		message string
	}{
		{"hash", func(c *proof.Case) { c.FontSHA256 = strings.Repeat("0", 64) }, "SHA-256 mismatch"},
		{"feature", func(c *proof.Case) { c.Features = []string{"liga[4:2]=1"} }, "range end"},
		{"buffer", func(c *proof.Case) { c.Buffer = proof.Buffer{High: 1, Low: 2} }, "invalid buffer configuration"},
		{"ppem", func(c *proof.Case) { c.Render.PPEM = 4097 }, "ppem"},
		{"script", func(c *proof.Case) { c.Script = "Arab" }, "supports Latn and Hebr"},
		{"font", func(c *proof.Case) { c.Font = "missing-font" }, "read font"},
		{"long", func(c *proof.Case) { s := strings.Repeat("a", 8193); c.Text = &s }, "8192 scalars"},
		{"group", func(c *proof.Case) { s := "a" + strings.Repeat("\u0301", 256); c.Text = &s }, "limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			copy := c
			tt.change(&copy)
			body, _ := json.Marshal(copy)
			out := workbenchRequest(w, "/api/preview", body)
			if out.Code != 400 || !strings.Contains(out.Body.String(), tt.message) {
				t.Fatalf("%d: %s", out.Code, out.Body)
			}
			if strings.Contains(out.Body.String(), `"png"`) {
				t.Fatal("partial proof published")
			}
		})
	}
	for _, body := range []string{`{} {}`, `{"unknown":1}`, strings.Repeat(" ", maxWorkbenchJSON+1)} {
		if out := workbenchRequest(w, "/api/init", []byte(body)); out.Code != 400 {
			t.Fatalf("accepted invalid body: %d", out.Code)
		}
	}
	if _, err := readLimitedFile(".", 1024); err == nil {
		t.Fatal("accepted directory")
	}
	path := filepath.Join(t.TempDir(), "large")
	if err := os.WriteFile(path, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readLimitedFile(path, 4); err == nil {
		t.Fatal("file limit bypassed")
	}
	// A canceled request waiting on the execution gate must never read a font or
	// publish a result. No sleeps or arbitrary scheduling assumptions required.
	w.gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body, _ := json.Marshal(c)
	req := httptest.NewRequest("POST", "http://"+w.host+"/api/preview", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Origin", "http://"+w.host)
	req.Header.Set("X-Workbench-Token", w.token)
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	w.ServeHTTP(out, req)
	<-w.gate
	if out.Body.Len() != 0 {
		t.Fatal("canceled request published output")
	}
}
