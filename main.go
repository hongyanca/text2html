package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	config, err := loadConfig("config.toml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	dataDir := config.DataDir
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatal(err)
	}
	app := newApplication(dataDir, config.MaxParagraphWords)
	go app.cleanupLoop()
	mux := http.NewServeMux()
	mux.HandleFunc("/", app.index)
	mux.HandleFunc("/style.css", app.static)
	mux.HandleFunc("/app.js", app.static)
	mux.HandleFunc("/convert", app.convert)
	mux.HandleFunc("/delete-generated", app.deleteGenerated)
	mux.HandleFunc("/status/", app.status)
	mux.HandleFunc("/result/", app.result)
	server := &http.Server{Addr: config.Address, Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("Access http://localhost:%d", config.Port)
	log.Fatal(server.ListenAndServe())
}

type appConfig struct {
	DataDir           string
	Address           string
	Port              int
	MaxParagraphWords int
}

func loadConfig(path string) (appConfig, error) {
	config := appConfig{DataDir: "/Volumes/RAM-Disk/text2html", Address: ":8080", Port: 8080, MaxParagraphWords: 300}
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return appConfig{}, err
	}
	for lineNo, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return appConfig{}, fmt.Errorf("config.toml:%d: expected key = value", lineNo+1)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		switch key {
		case "data_dir":
			value, err = strconv.Unquote(value)
			if err != nil || value == "" {
				return appConfig{}, fmt.Errorf("config.toml:%d: data_dir must be a quoted non-empty string", lineNo+1)
			}
			config.DataDir = value
		case "port":
			config.Port, err = strconv.Atoi(value)
			if err != nil || config.Port < 1 || config.Port > 65535 {
				return appConfig{}, fmt.Errorf("config.toml:%d: port must be between 1 and 65535", lineNo+1)
			}
			config.Address = fmt.Sprintf(":%d", config.Port)
		case "max_paragraph_words":
			config.MaxParagraphWords, err = strconv.Atoi(value)
			if err != nil || config.MaxParagraphWords < 1 {
				return appConfig{}, fmt.Errorf("config.toml:%d: max_paragraph_words must be a positive integer", lineNo+1)
			}
		default:
			return appConfig{}, fmt.Errorf("config.toml:%d: unknown setting %q", lineNo+1, key)
		}
	}
	return config, nil
}
