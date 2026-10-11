package main

import (
	"embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webFiles embed.FS

const maxUpload = 5 << 20
const retention = 24 * time.Hour

type job struct {
	mu                                                     sync.RWMutex
	id, filename, sourcePath, outputPath, errorText, stage string
	preserveBytes                                          bool
	progress                                               int
	done                                                   bool
	updated                                                time.Time
}
type application struct {
	dir               string
	maxParagraphWords int
	mu                sync.RWMutex
	jobs              map[string]*job
}

func newApplication(dir string, maxParagraphWords int) *application {
	return &application{dir: dir, maxParagraphWords: maxParagraphWords, jobs: make(map[string]*job)}
}
func (a *application) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "interface unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
func (a *application) static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name != "style.css" && name != "app.js" {
		http.NotFound(w, r)
		return
	}
	data, err := webFiles.ReadFile("web/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if name == "style.css" {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}
func (a *application) convert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(1<<20))
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		http.Error(w, "Upload is too large or invalid", http.StatusBadRequest)
		return
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Choose a .txt file", 400)
		return
	}
	defer f.Close()
	if !strings.EqualFold(filepath.Ext(filepath.Base(header.Filename)), ".txt") {
		http.Error(w, "Only .txt files are accepted", 400)
		return
	}
	id, err := randomID()
	if err != nil {
		http.Error(w, "Could not create conversion", 500)
		return
	}
	base := safeBase(header.Filename)
	if base == "" {
		base = "converted.txt"
	}
	outName := strings.TrimSuffix(base, filepath.Ext(base)) + ".html"
	src := filepath.Join(a.dir, id+".txt")
	dst := filepath.Join(a.dir, id+".html")
	out, err := os.OpenFile(src, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		http.Error(w, "Could not store upload", 500)
		return
	}
	n, copyErr := io.Copy(out, io.LimitReader(f, maxUpload+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n > maxUpload {
		_ = os.Remove(src)
		http.Error(w, "Upload is too large or could not be read", 400)
		return
	}
	j := &job{id: id, filename: outName, sourcePath: src, outputPath: dst, stage: "Reading file", progress: 15, updated: time.Now(), preserveBytes: r.FormValue("encoding") == "preserve"}
	a.mu.Lock()
	a.jobs[id] = j
	a.mu.Unlock()
	go a.run(j)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"jobId": id})
}
func safeBase(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}
func (a *application) run(j *job) {
	set := func(stage string, progress int) {
		j.mu.Lock()
		j.stage = stage
		j.progress = progress
		j.updated = time.Now()
		j.mu.Unlock()
	}
	defer func() { j.mu.Lock(); j.done = true; j.updated = time.Now(); j.mu.Unlock() }()
	defer os.Remove(j.sourcePath)
	set("Reading file", 20)
	b, err := os.ReadFile(j.sourcePath)
	if err != nil {
		j.fail("Could not read uploaded file")
		return
	}
	set("Detecting paragraphs", 40)
	paragraphs := detectParagraphs(decodeUploadedText(b, j.preserveBytes))
	set("Splitting long paragraphs", 60)
	paragraphs = splitLongParagraphs(paragraphs, a.maxParagraphWords)
	set("Generating HTML", 80)
	body := toHTML(j.filename, paragraphs)
	j.mu.Lock()
	if err := os.WriteFile(j.outputPath, []byte(body), 0600); err != nil {
		j.errorText = "Could not save generated HTML"
		j.stage = "Error"
		j.done = true
		j.updated = time.Now()
		j.mu.Unlock()
		return
	}
	j.stage = "Complete"
	j.progress = 100
	j.done = true
	j.updated = time.Now()
	j.mu.Unlock()
}

func (a *application) deleteGenerated(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	removed := 0
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, j := range a.jobs {
		j.mu.Lock()
		if j.done {
			if j.errorText == "" {
				if err := os.Remove(j.outputPath); err == nil || os.IsNotExist(err) {
					removed++
				} else {
					j.mu.Unlock()
					http.Error(w, "Could not delete every generated file", http.StatusInternalServerError)
					return
				}
			}
			delete(a.jobs, id)
		}
		j.mu.Unlock()
	}
	entries, err := os.ReadDir(a.dir)
	if err != nil {
		http.Error(w, "Could not inspect generated files", http.StatusInternalServerError)
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".html") || !isJobFilename(name) {
			continue
		}
		if active := a.jobs[strings.TrimSuffix(name, ".html")]; active != nil {
			active.mu.RLock()
			inProgress := !active.done
			active.mu.RUnlock()
			if inProgress {
				continue
			}
		}
		if err := os.Remove(filepath.Join(a.dir, name)); err != nil && !os.IsNotExist(err) {
			http.Error(w, "Could not delete every generated file", http.StatusInternalServerError)
			return
		}
		removed++
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"deleted": removed})
}

func isJobFilename(name string) bool {
	if !strings.HasSuffix(name, ".html") {
		return false
	}
	id := strings.TrimSuffix(name, ".html")
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 24
}
func (j *job) fail(message string) {
	j.mu.Lock()
	j.errorText = message
	j.stage = "Error"
	j.mu.Unlock()
}
func (a *application) lookup(id string) *job { a.mu.RLock(); defer a.mu.RUnlock(); return a.jobs[id] }
func (a *application) status(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/status/")
	j := a.lookup(id)
	if j == nil {
		http.NotFound(w, r)
		return
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jobId": j.id, "stage": j.stage, "progress": j.progress, "done": j.done, "error": j.errorText, "filename": j.filename})
}
func (a *application) result(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/result/"), "/")
	if len(parts) != 2 || (parts[1] != "open" && parts[1] != "download") {
		http.NotFound(w, r)
		return
	}
	j := a.lookup(parts[0])
	if j == nil {
		http.NotFound(w, r)
		return
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	if !j.done || j.errorText != "" {
		http.Error(w, "Result is not ready", http.StatusConflict)
		return
	}
	if j.preserveBytes {
		w.Header().Set("Content-Type", "text/html")
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if parts[1] == "download" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": j.filename}))
	} else {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": j.filename}))
	}
	http.ServeFile(w, r, j.outputPath)
}
func (a *application) cleanupLoop() {
	t := time.NewTicker(time.Hour)
	for range t.C {
		cutoff := time.Now().Add(-retention)
		a.mu.Lock()
		for id, j := range a.jobs {
			j.mu.RLock()
			old := j.updated.Before(cutoff)
			j.mu.RUnlock()
			if old {
				_ = os.Remove(j.sourcePath)
				_ = os.Remove(j.outputPath)
				delete(a.jobs, id)
			}
		}
		a.mu.Unlock()
	}
}
