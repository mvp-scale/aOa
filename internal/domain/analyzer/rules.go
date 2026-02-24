package analyzer

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlRule is the YAML-serialized form of a Rule.
type yamlRule struct {
	ID              string   `yaml:"id"`
	Label           string   `yaml:"label"`
	Dimension       string   `yaml:"dimension"`
	Tier            string   `yaml:"tier"`
	Bit             int      `yaml:"bit"`
	Severity        string   `yaml:"severity"`
	Kind            string   `yaml:"kind"`
	StructuralCheck string   `yaml:"structural_check,omitempty"`
	TextPatterns    []string `yaml:"text_patterns,omitempty"`
	SkipTest        bool     `yaml:"skip_test,omitempty"`
	SkipMain        bool     `yaml:"skip_main,omitempty"`
	CodeOnly        bool     `yaml:"code_only,omitempty"`
}

// LoadRulesFromFS loads all YAML rule files from an embedded filesystem.
// Follows the same pattern as enricher.LoadAtlas(): read dir, sort, unmarshal.
func LoadRulesFromFS(fsys fs.FS, dir string) ([]Rule, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read rules dir %q: %w", dir, err)
	}

	// Sort for deterministic load order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var allRules []Rule
	seenIDs := make(map[string]string)        // id → source file
	seenTierBit := make(map[string]string) // "tier:bit" → id

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := dir + "/" + entry.Name()
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		var yamlRules []yamlRule
		if err := yaml.Unmarshal(data, &yamlRules); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}

		for _, yr := range yamlRules {
			rule, err := convertRule(yr)
			if err != nil {
				return nil, fmt.Errorf("%s: rule %q: %w", entry.Name(), yr.ID, err)
			}

			// Validate unique ID
			if prev, ok := seenIDs[rule.ID]; ok {
				return nil, fmt.Errorf("duplicate rule ID %q (first in %s, again in %s)", rule.ID, prev, entry.Name())
			}
			seenIDs[rule.ID] = entry.Name()

			// Validate unique (tier, bit)
			tbKey := fmt.Sprintf("%d:%d", rule.Tier, rule.Bit)
			if prevID, ok := seenTierBit[tbKey]; ok {
				return nil, fmt.Errorf("duplicate (tier=%s, bit=%d): %q and %q",
					yr.Tier, rule.Bit, prevID, rule.ID)
			}
			seenTierBit[tbKey] = rule.ID

			allRules = append(allRules, rule)
		}
	}

	return allRules, nil
}

// convertRule converts a yamlRule to a Rule.
func convertRule(yr yamlRule) (Rule, error) {
	if yr.ID == "" {
		return Rule{}, fmt.Errorf("missing id")
	}

	tier := TierFromName(yr.Tier)
	if tier < 0 {
		return Rule{}, fmt.Errorf("unknown tier %q", yr.Tier)
	}

	sev := SeverityFromName(yr.Severity)
	if sev < 0 {
		return Rule{}, fmt.Errorf("unknown severity %q", yr.Severity)
	}

	kind := RuleKindFromName(yr.Kind)
	if kind < 0 {
		return Rule{}, fmt.Errorf("unknown kind %q", yr.Kind)
	}

	if yr.Bit < 0 || yr.Bit > 63 {
		return Rule{}, fmt.Errorf("bit %d out of range 0-63", yr.Bit)
	}

	// Validate kind-specific fields
	if kind == RuleText && len(yr.TextPatterns) == 0 {
		return Rule{}, fmt.Errorf("text rule must have text_patterns")
	}
	if kind == RuleStructural && yr.StructuralCheck == "" {
		return Rule{}, fmt.Errorf("structural rule must have structural_check")
	}

	return Rule{
		ID:              yr.ID,
		Label:           yr.Label,
		Dimension:       yr.Dimension,
		StructuralCheck: yr.StructuralCheck,
		Tier:            tier,
		Bit:             yr.Bit,
		Severity:        sev,
		Kind:            kind,
		TextPatterns:    yr.TextPatterns,
		SkipTest:        yr.SkipTest,
		SkipMain:        yr.SkipMain,
		CodeOnly:        yr.CodeOnly,
	}, nil
}
