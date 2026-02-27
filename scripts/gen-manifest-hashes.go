//go:build ignore

// gen-manifest-hashes walks dist/grammars/, computes SHA256 + file size per .so/.dylib,
// and generates manifest.json for GitHub Releases.
//
// Usage: go run scripts/gen-manifest-hashes.go [--dir dist/grammars] [--out manifest.json]
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PlatSize map[string]int64
type PlatHash map[string]string

type GrammarInfo struct {
	Name     string   `json:"name"`
	Sizes    PlatSize `json:"sizes"`
	SHA256   PlatHash `json:"sha256"`
}

type Manifest struct {
	Version  int                    `json:"version"`
	BaseURL  string                 `json:"base_url"`
	Grammars map[string]GrammarInfo `json:"grammars"`
}

func main() {
	dir := flag.String("dir", "dist/grammars", "Directory containing grammar .so/.dylib files")
	out := flag.String("out", "manifest.json", "Output manifest file")
	baseURL := flag.String("base-url", "https://github.com/corey/aoa/releases/download", "Base URL for downloads")
	flag.Parse()

	manifest := Manifest{
		Version:  1,
		BaseURL:  *baseURL,
		Grammars: make(map[string]GrammarInfo),
	}

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading directory %s: %v\n", *dir, err)
		os.Exit(1)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".so") && !strings.HasSuffix(name, ".dylib") {
			continue
		}

		// Parse: {lang}-{os}-{arch}.so
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)

		// Find the platform suffix (last two dash-separated parts).
		parts := strings.Split(base, "-")
		if len(parts) < 3 {
			fmt.Fprintf(os.Stderr, "skipping %s: unexpected name format\n", name)
			continue
		}

		platform := parts[len(parts)-2] + "-" + parts[len(parts)-1]
		lang := strings.Join(parts[:len(parts)-2], "-")

		path := filepath.Join(*dir, name)
		hash, size, err := hashFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error hashing %s: %v\n", path, err)
			continue
		}

		info, ok := manifest.Grammars[lang]
		if !ok {
			info = GrammarInfo{
				Name:   lang,
				Sizes:  make(PlatSize),
				SHA256: make(PlatHash),
			}
		}
		info.Sizes[platform] = size
		info.SHA256[platform] = hash
		manifest.Grammars[lang] = info
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling manifest: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", *out, err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s with %d grammars\n", *out, len(manifest.Grammars))
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(h.Sum(nil)), size, nil
}
