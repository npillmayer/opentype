package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/npillmayer/opentype/internal/proof"

	"github.com/thatisuday/commando"
)

//go:embed workbench/index.html workbench/app.js workbench/style.css workbench/source.mjs workbench/updates.mjs
var workbenchFiles embed.FS

const maxWorkbenchJSON = 1 << 20
const maxWorkbenchFont = 32 << 20
const maxWorkbenchScalars = 8192

type workbench struct {
	host, token string
	initial     *proof.Case
	gate        chan struct{}
}

func runServeCommand(_ map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
	port := mustFlagInt(flags["port"], "port")
	if port < 0 || port > 65535 {
		fatalf("port must be between 0 and 65535")
	}
	var initial *proof.Case
	path, _ := flags["case"].GetString()
	if path != "" {
		c, err := readWorkbenchCase(path)
		if err != nil {
			fatalf("open case: %v", err)
		}
		initial = &c
	}
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fatalf("serve: %v", err)
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		_ = listener.Close()
		fatalf("session token: %v", err)
	}
	w := &workbench{host: listener.Addr().String(), token: hex.EncodeToString(secret), initial: initial, gate: make(chan struct{}, 1)}
	server := &http.Server{Handler: w, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() { <-ctx.Done(); _ = server.Close() }()
	fmt.Printf("Open http://%s/#%s\nLocal shaping workbench; Ctrl-C stops the server.\n", w.host, w.token)
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fatalf("serve: %v", err)
	}
}

func (w *workbench) ServeHTTP(out http.ResponseWriter, r *http.Request) {
	out.Header().Set("Cache-Control", "no-store")
	out.Header().Set("X-Content-Type-Options", "nosniff")
	out.Header().Set("Referrer-Policy", "no-referrer")
	out.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	// Exact Host plus Origin and an unguessable session token prevent other sites
	// (including DNS rebinding) from driving this local file-reading interface.
	if r.Host != w.host {
		http.Error(out, "unexpected host", http.StatusForbidden)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		if r.Method != http.MethodPost {
			http.Error(out, "POST required", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Origin") != "http://"+w.host || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Workbench-Token")), []byte(w.token)) != 1 {
			http.Error(out, "invalid workbench session; use the URL printed by serve", http.StatusForbidden)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(out, "application/json required", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(out, r.Body, maxWorkbenchJSON)
		w.api(out, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(out, "GET required", http.StatusMethodNotAllowed)
		return
	}
	name := map[string]string{"/": "index.html", "/app.js": "app.js", "/style.css": "style.css", "/source.mjs": "source.mjs", "/updates.mjs": "updates.mjs"}[r.URL.Path]
	if name == "" {
		http.NotFound(out, r)
		return
	}
	data, err := workbenchFiles.ReadFile("workbench/" + name)
	if err != nil {
		http.Error(out, "asset unavailable", 500)
		return
	}
	out.Header().Set("Content-Type", map[string]string{"index.html": "text/html; charset=utf-8", "updates.mjs": "text/javascript; charset=utf-8", "source.mjs": "text/javascript; charset=utf-8", "app.js": "text/javascript; charset=utf-8", "style.css": "text/css; charset=utf-8"}[name])
	_, _ = out.Write(data)
}
func decodeRequest(r *http.Request, value any) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("request is not UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("expected one JSON object")
	}
	return nil
}
func apiJSON(out http.ResponseWriter, value any) {
	out.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(out).Encode(value)
}
func apiError(out http.ResponseWriter, err error) {
	http.Error(out, err.Error(), http.StatusBadRequest)
}
func (w *workbench) api(out http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/init":
		var empty struct{}
		if err := decodeRequest(r, &empty); err != nil {
			apiError(out, err)
			return
		}
		cwd, _ := os.Getwd()
		apiJSON(out, struct {
			Case      *proof.Case `json:"case"`
			Directory string      `json:"directory"`
		}{w.initial, cwd})
	case "/api/open":
		var req struct {
			Path string `json:"path"`
		}
		if err := decodeRequest(r, &req); err != nil {
			apiError(out, err)
			return
		}
		c, err := readWorkbenchCase(req.Path)
		if err != nil {
			apiError(out, err)
			return
		}
		apiJSON(out, c)
	case "/api/preview":
		var c proof.Case
		if err := decodeRequest(r, &c); err != nil {
			apiError(out, err)
			return
		}
		c, err := proof.NormalizeCase(c)
		if err != nil {
			apiError(out, err)
			return
		}
		if c.Script != "Latn" && c.Script != "Hebr" {
			apiError(out, fmt.Errorf("workbench supports Latn and Hebr runs"))
			return
		}
		if utf8.RuneCountInString(*c.Text) > maxWorkbenchScalars {
			apiError(out, fmt.Errorf("workbench input exceeds %d scalars", maxWorkbenchScalars))
			return
		}
		// One execution at a time bounds simultaneous raster/font allocations.
		// Waiting obsolete requests leave promptly when the browser aborts them.
		select {
		case w.gate <- struct{}{}:
		case <-r.Context().Done():
			return
		}
		defer func() { <-w.gate }()
		if err := r.Context().Err(); err != nil {
			return
		}
		c.Font, err = filepath.Abs(c.Font)
		if err != nil {
			apiError(out, err)
			return
		}
		font, err := readLimitedFile(c.Font, maxWorkbenchFont)
		if err != nil {
			apiError(out, fmt.Errorf("read font: %w", err))
			return
		}
		execution, err := proof.ExecuteContext(r.Context(), c, font)
		if err != nil {
			apiError(out, err)
			return
		}
		var pngBytes bytes.Buffer
		if err := png.Encode(&pngBytes, execution.Image); err != nil {
			apiError(out, err)
			return
		}
		if r.Context().Err() != nil {
			return
		}
		apiJSON(out, struct {
			Report   proof.Result   `json:"report"`
			Geometry proof.Geometry `json:"geometry"`
			PNG      []byte         `json:"png"`
		}{execution.Report, execution.Geometry, pngBytes.Bytes()})
	default:
		http.NotFound(out, r)
	}
}
func readLimitedFile(path string, limit int64) ([]byte, error) {
	// Stat before open rejects devices and FIFOs instead of hanging on them.
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("select a regular file: %s", path)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds %d-byte limit: %s", limit, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = fmt.Errorf("file exceeds %d-byte limit", limit)
	}
	return data, err
}
func readWorkbenchCase(path string) (proof.Case, error) {
	return readProofCaseWith(path, func(p string) ([]byte, error) { return readLimitedFile(p, maxWorkbenchJSON) })
}
