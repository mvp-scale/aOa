package test

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
)

// --- Index State Fixture ---

type fixtureFile struct {
	Files   map[string]fixtureFileMeta `json:"files"`
	Symbols []fixtureSymbol            `json:"symbols"`
	Domains map[string]fixtureDomain   `json:"domains"`
}

type fixtureFileMeta struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Domain   string `json:"domain"`
}

type fixtureSymbol struct {
	FileID    uint32   `json:"file_id"`
	Name      string   `json:"name"`
	Signature string   `json:"signature"`
	Kind      string   `json:"kind"`
	StartLine uint16   `json:"start_line"`
	EndLine   uint16   `json:"end_line"`
	Parent    string   `json:"parent"`
	Tokens    []string `json:"tokens"`
	Tags      []string `json:"tags"`
}

type fixtureDomain struct {
	Terms map[string][]string `json:"terms"`
}

func loadIndexFixture(path string) (*ports.Index, map[string]index.Domain, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read index fixture: %w", err)
	}

	var f fixtureFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, nil, fmt.Errorf("parse index fixture: %w", err)
	}

	// Build ports.Index
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	// Load files
	for idStr, fm := range f.Files {
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return nil, nil, fmt.Errorf("parse file id %q: %w", idStr, err)
		}
		idx.Files[uint32(id)] = &ports.FileMeta{
			Path:     fm.Path,
			Language: fm.Language,
			Domain:   fm.Domain,
		}
	}

	// Load symbols + build token map
	for _, sym := range f.Symbols {
		ref := ports.TokenRef{
			FileID: sym.FileID,
			Line:   sym.StartLine,
		}

		idx.Metadata[ref] = &ports.SymbolMeta{
			Name:      sym.Name,
			Signature: sym.Signature,
			Kind:      sym.Kind,
			StartLine: sym.StartLine,
			EndLine:   sym.EndLine,
			Parent:    sym.Parent,
			Tags:      sym.Tags,
		}

		// Add each token to the inverted index
		for _, tok := range sym.Tokens {
			idx.Tokens[tok] = append(idx.Tokens[tok], ref)
		}
	}

	// Build domain map
	domains := make(map[string]index.Domain, len(f.Domains))
	for name, fd := range f.Domains {
		domains[name] = index.Domain{
			Terms: fd.Terms,
		}
	}

	return idx, domains, nil
}

// --- Search Fixtures ---

type SearchFixture struct {
	Comment              string        `json:"_comment,omitempty"`
	Query                string        `json:"query"`
	Mode                 string        `json:"mode"`
	Flags                SearchFlags   `json:"flags"`
	Expected             []SearchHit   `json:"expected,omitempty"`
	ExpectedTokenization []string      `json:"expected_tokenization,omitempty"`
	ExpectedCount        *int          `json:"expected_count,omitempty"`
	ExpectedExitCode     *int          `json:"expected_exit_code,omitempty"`
}

type SearchFlags struct {
	AndMode         bool   `json:"and_mode,omitempty"`
	CaseInsensitive bool   `json:"case_insensitive,omitempty"`
	WordBoundary    bool   `json:"word_boundary,omitempty"`
	CountOnly       bool   `json:"count_only,omitempty"`
	Quiet           bool   `json:"quiet,omitempty"`
	MaxCount        int    `json:"max_count,omitempty"`
	IncludeGlob     string `json:"include_glob,omitempty"`
	ExcludeGlob     string `json:"exclude_glob,omitempty"`
}

type SearchHit struct {
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Symbol string   `json:"symbol"`
	Range  [2]int   `json:"range"`
	Domain string   `json:"domain"`
	Tags   []string `json:"tags"`
}

func loadSearchFixtures(path string) ([]SearchFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fixtures []SearchFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		return nil, err
	}

	return fixtures, nil
}
