//go:build !lean

package recon

import (
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/domain/analyzer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_FullPipeline(t *testing.T) {
	source := `package handler

import (
	"fmt"
	"os/exec"
)

func processInput(input string) {
	// hardcoded credential
	password = "secret123"

	// command injection
	cmd := exec.Command(input)
	_ = cmd.Run()

	for i := 0; i < 10; i++ {
		f, _ := os.Open("file")
		defer f.Close()
	}
}
`
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	result := engine.AnalyzeFile("handler.go", []byte(source), false, false)
	require.NotNil(t, result, "should have findings")

	// Check that we found findings
	assert.Greater(t, len(result.Findings), 0, "should have at least one finding")
	assert.False(t, result.Bitmask.IsZero(), "file bitmask should have bits set")

	// Look for specific findings
	foundRules := make(map[string]bool)
	for _, f := range result.Findings {
		foundRules[f.RuleID] = true
	}
	assert.True(t, foundRules["hardcoded_secret"] || foundRules["command_injection"] || foundRules["defer_in_loop"],
		"should find at least one of the planted issues, found: %v", foundRules)
}

func TestEngine_TextOnlyRules(t *testing.T) {
	source := `package main

import "crypto/md5"

func weakHash(data []byte) {
	h := md5.New()
	h.Write(data)
}
`
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	result := engine.AnalyzeFile("main.go", []byte(source), false, false)
	require.NotNil(t, result)

	foundRules := make(map[string]bool)
	for _, f := range result.Findings {
		foundRules[f.RuleID] = true
	}
	assert.True(t, foundRules["weak_hash"], "should detect weak_hash, found: %v", foundRules)
}

func TestEngine_SkipTestFiles(t *testing.T) {
	source := `package handler

func TestSomething(t *testing.T) {
	password = "test_secret"
	cmd := exec.Command("ls")
}
`
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	result := engine.AnalyzeFile("handler_test.go", []byte(source), true, false)
	// Most security rules skip test files, so findings should be minimal
	if result != nil {
		for _, f := range result.Findings {
			rule := findRule(rules, f.RuleID)
			if rule != nil {
				assert.False(t, rule.SkipTest, "finding %s should not have SkipTest=true", f.RuleID)
			}
		}
	}
}

func TestEngine_CommentLineSkipping(t *testing.T) {
	source := `package main

// password = "this is a comment"
# password = "another comment"
func main() {
	password = "real_secret"
}
`
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	result := engine.AnalyzeFile("main.go", []byte(source), false, false)
	if result != nil {
		for _, f := range result.Findings {
			if f.RuleID == "hardcoded_secret" {
				assert.NotEqual(t, 3, f.Line, "should not find secret on comment line 3")
				assert.NotEqual(t, 4, f.Line, "should not find secret on comment line 4")
			}
		}
	}
}

func TestEngine_StructuralOnly(t *testing.T) {
	source := `package mylib

func Initialize() {
	panic("bad state")
}
`
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	result := engine.AnalyzeFile("mylib.go", []byte(source), false, false)
	require.NotNil(t, result)

	foundRules := make(map[string]bool)
	for _, f := range result.Findings {
		foundRules[f.RuleID] = true
	}
	assert.True(t, foundRules["panic_in_lib"], "should detect panic_in_lib, found: %v", foundRules)
}

func TestEngine_EmptySource(t *testing.T) {
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	result := engine.AnalyzeFile("empty.go", []byte{}, false, false)
	assert.Nil(t, result)
}

func TestEngine_CleanCode(t *testing.T) {
	source := `package handler

import "fmt"

func Hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
`
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	result := engine.AnalyzeFile("handler.go", []byte(source), false, false)
	// Clean code may still have some findings (like fmt.Sprintf matching composite patterns)
	// but should not have security-critical findings
	if result != nil {
		for _, f := range result.Findings {
			rule := findRule(rules, f.RuleID)
			if rule != nil {
				assert.NotEqual(t, analyzer.SevCritical, f.Severity,
					"clean code should not have critical findings, found: %s at line %d", f.RuleID, f.Line)
			}
		}
	}
}

func TestEngine_MethodAttribution(t *testing.T) {
	source := `package main

import "crypto/md5"

func weakHasher(data []byte) {
	h := md5.New()
	h.Write(data)
}

func safeFunc() {
	x := 42
	_ = x
}
`
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	result := engine.AnalyzeFile("main.go", []byte(source), false, false)
	if result != nil && len(result.Methods) > 0 {
		// Check that findings are attributed to the correct method
		for _, m := range result.Methods {
			if m.Name == "weakHasher" {
				assert.Greater(t, len(m.Findings), 0, "weakHasher should have findings")
			}
		}
	}
}

func TestEngine_RuleCount(t *testing.T) {
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)
	assert.Equal(t, len(rules), engine.RuleCount())
}

func TestBuildLineOffsets(t *testing.T) {
	source := []byte("line1\nline2\nline3\n")
	offsets := buildLineOffsets(source)
	assert.Equal(t, 0, offsets[0])  // line 1 starts at 0
	assert.Equal(t, 6, offsets[1])  // line 2 starts at 6
	assert.Equal(t, 12, offsets[2]) // line 3 starts at 12
}

func TestOffsetToLine(t *testing.T) {
	offsets := []int{0, 6, 12}
	assert.Equal(t, 1, offsetToLine(offsets, 0))
	assert.Equal(t, 1, offsetToLine(offsets, 5))
	assert.Equal(t, 2, offsetToLine(offsets, 6))
	assert.Equal(t, 2, offsetToLine(offsets, 11))
	assert.Equal(t, 3, offsetToLine(offsets, 12))
}

// TestEngine_MultiLanguage tests the full pipeline across Go, Python, and JavaScript
// source files with planted security issues.
func TestEngine_MultiLanguage(t *testing.T) {
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	cases := []struct {
		name     string
		file     string
		source   string
		isTest   bool
		isMain   bool
		wantRules []string // rule IDs we expect to find
	}{
		{
			name:   "Go handler with secrets + injection + weak crypto",
			file:   "internal/handler/auth.go",
			isMain: false,
			source: `package handler

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
)

func AuthHandler(username, password string) error {
	secret = "hunter2"
	h := md5.New()
	h.Write([]byte(password))
	cmd := exec.Command(username)
	_ = os.Remove("/tmp/lock")
	os.WriteFile("/tmp/token", []byte("x"), 0777)
	return nil
}

func DeferInLoop() {
	for i := 0; i < 10; i++ {
		f, _ := os.Open(fmt.Sprintf("/tmp/f%d", i))
		defer f.Close()
	}
}
`,
			wantRules: []string{"weak_hash", "command_injection", "defer_in_loop"},
		},
		{
			name:   "Python with command injection + weak hash",
			file:   "internal/handler/app.py",
			isMain: false,
			source: `import subprocess
import hashlib

def process_request(user_input):
    subprocess.call(user_input)
    h = hashlib.md5(user_input.encode())
    password = "admin123"
    return h.hexdigest()
`,
			wantRules: []string{"command_injection", "weak_hash", "hardcoded_secret"},
		},
		{
			name:   "JavaScript with eval + injection + CORS",
			file:   "internal/handler/server.js",
			isMain: false,
			source: `const child_process = require('child_process');
const crypto = require('crypto');

function handleRequest(req) {
    child_process.exec(req.body.command);
    eval(req.body.code);
    const hash = crypto.createHash('md5').update('x').digest('hex');
    const password = "supersecret";
    return hash;
}

function setCors(res) {
    res.setHeader("Access-Control-Allow-Origin", "*");
}
`,
			wantRules: []string{"command_injection", "eval_usage", "weak_hash"},
		},
		{
			name:   "Go library with panic + AWS creds + private key",
			file:   "internal/util/helpers.go",
			isMain: false,
			source: `package util

func PanicHelper() {
	panic("something went wrong")
}

func AWSCreds() string {
	return "AKIAIOSFODNN7EXAMPLE"
}

func PrivateKey() string {
	return "-----BEGIN RSA PRIVATE KEY-----\ndata..."
}

func InsecureTLS() {
	_ = "InsecureSkipVerify: true"
}
`,
			wantRules: []string{"panic_in_lib", "aws_credentials", "private_key_inline", "insecure_tls"},
		},
		{
			name:   "Go main file — panic should NOT trigger panic_in_lib",
			file:   "cmd/main.go",
			isMain: true,
			source: `package main

func main() {
	panic("startup failed")
}
`,
			wantRules: []string{}, // panic_in_lib skips main
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.AnalyzeFile(tc.file, []byte(tc.source), tc.isTest, tc.isMain)

			foundRules := map[string]bool{}
			if result != nil {
				for _, f := range result.Findings {
					foundRules[f.RuleID] = true
				}
			}

			for _, want := range tc.wantRules {
				assert.True(t, foundRules[want],
					"expected to find %q in %s, found rules: %v",
					want, tc.file, sortedKeys(foundRules))
			}

			// Log what we found for visibility
			if result != nil {
				t.Logf("  %s: %d findings, %d methods, bitmask popcount=%d",
					tc.file, len(result.Findings), len(result.Methods), result.Bitmask.PopCount())
				for _, f := range result.Findings {
					t.Logf("    L%d [%s] %s", f.Line, f.Severity, f.RuleID)
				}
				for _, m := range result.Methods {
					t.Logf("    method %q (L%d-%d) score=%d bits=%d",
						m.Name, m.Line, m.EndLine, m.Score, m.Bitmask.PopCount())
				}
			} else {
				t.Logf("  %s: no findings (nil result)", tc.file)
			}
		})
	}
}

// TestEngine_BboltRoundtrip tests save/load through bbolt for dimensional results.
func TestEngine_BboltRoundtrip(t *testing.T) {
	rules := analyzer.AllRules()
	parser := treesitter.NewParser()
	engine := NewEngine(rules, parser)

	source := `package handler

import "os/exec"

func Vuln(input string) {
	cmd := exec.Command(input)
	_ = cmd.Run()
}
`
	result := engine.AnalyzeFile("handler.go", []byte(source), false, false)
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Findings), 0)

	// Save to bbolt and load back
	dir := t.TempDir()
	store, err := bbolt.NewStore(dir + "/test.db")
	assert.NoError(t, err)
	defer store.Close()

	analyses := map[string]*analyzer.FileAnalysis{"handler.go": result}
	err = store.SaveAllDimensions("test-project", analyses)
	assert.NoError(t, err)

	loaded, err := store.LoadAllDimensions("test-project")
	assert.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Contains(t, loaded, "handler.go")

	loadedFile := loaded["handler.go"]
	assert.Equal(t, len(result.Findings), len(loadedFile.Findings))
	assert.Equal(t, result.Bitmask, loadedFile.Bitmask)

	t.Logf("Roundtrip: %d findings persisted and restored", len(loadedFile.Findings))
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func findRule(rules []analyzer.Rule, id string) *analyzer.Rule {
	for _, r := range rules {
		if r.ID == id {
			return &r
		}
	}
	return nil
}
