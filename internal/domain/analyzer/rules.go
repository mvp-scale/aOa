package analyzer

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlRule is the YAML-serialized form of a Rule.
// Kind is inferred from which detection blocks are present (ADR §2).
type yamlRule struct {
	ID           string                       `yaml:"id"`
	Label        string                       `yaml:"label"`
	Dimension    string                       `yaml:"dimension"`
	Tier         string                       `yaml:"tier"`
	Bit          int                          `yaml:"bit"`
	Severity     string                       `yaml:"severity"`
	Structural   *yamlStructuralBlock         `yaml:"structural,omitempty"`
	Regex        string                       `yaml:"regex,omitempty"`
	TextPatterns []string                     `yaml:"text_patterns,omitempty"`
	SkipTest     bool                         `yaml:"skip_test,omitempty"`
	SkipMain     bool                         `yaml:"skip_main,omitempty"`
	CodeOnly     bool                         `yaml:"code_only,omitempty"`
	SkipLangs    []string                     `yaml:"skip_langs,omitempty"`
}

// yamlStructuralBlock is the YAML form of StructuralBlock.
type yamlStructuralBlock struct {
	Match               string       `yaml:"match"`
	ReceiverContains    []string     `yaml:"receiver_contains,omitempty"`
	Inside              string       `yaml:"inside,omitempty"`
	HasArg              *yamlArgSpec `yaml:"has_arg,omitempty"`
	NameContains        []string     `yaml:"name_contains,omitempty"`
	ValueType           string       `yaml:"value_type,omitempty"`
	WithoutSibling      string       `yaml:"without_sibling,omitempty"`
	NestingThreshold    int          `yaml:"nesting_threshold,omitempty"`
	ChildCountThreshold int          `yaml:"child_count_threshold,omitempty"`
	ParentKinds         []string     `yaml:"parent_kinds,omitempty"`
	TextContains        []string     `yaml:"text_contains,omitempty"`
	LineThreshold       int          `yaml:"line_threshold,omitempty"`
}

// yamlArgSpec is the YAML form of ArgSpec.
type yamlArgSpec struct {
	Type         []string `yaml:"type,omitempty"`
	TextContains []string `yaml:"text_contains,omitempty"`
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
	seenIDs := make(map[string]string)    // id → source file
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
// Kind is inferred from which detection blocks are present:
//   - text_patterns only → RuleText
//   - structural only → RuleStructural
//   - both → RuleComposite
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

	if yr.Bit < 0 || yr.Bit > 63 {
		return Rule{}, fmt.Errorf("bit %d out of range 0-63", yr.Bit)
	}

	// Infer kind from present blocks (ADR §2)
	hasText := len(yr.TextPatterns) > 0
	hasStructural := yr.Structural != nil

	var kind RuleKind
	switch {
	case hasText && hasStructural:
		kind = RuleComposite
	case hasStructural:
		kind = RuleStructural
	case hasText:
		kind = RuleText
	default:
		return Rule{}, fmt.Errorf("rule must have text_patterns, structural block, or both")
	}

	// Convert structural block
	var sb *StructuralBlock
	if yr.Structural != nil {
		sb = &StructuralBlock{
			Match:               yr.Structural.Match,
			ReceiverContains:    yr.Structural.ReceiverContains,
			Inside:              yr.Structural.Inside,
			NameContains:        yr.Structural.NameContains,
			ValueType:           yr.Structural.ValueType,
			WithoutSibling:      yr.Structural.WithoutSibling,
			NestingThreshold:    yr.Structural.NestingThreshold,
			ChildCountThreshold: yr.Structural.ChildCountThreshold,
			ParentKinds:         yr.Structural.ParentKinds,
			TextContains:        yr.Structural.TextContains,
			LineThreshold:       yr.Structural.LineThreshold,
		}
		if yr.Structural.HasArg != nil {
			sb.HasArg = &ArgSpec{
				Type:         yr.Structural.HasArg.Type,
				TextContains: yr.Structural.HasArg.TextContains,
			}
		}
	}

	return Rule{
		ID:           yr.ID,
		Label:        yr.Label,
		Dimension:    yr.Dimension,
		Structural:   sb,
		Regex:        yr.Regex,
		Tier:         tier,
		Bit:          yr.Bit,
		Severity:     sev,
		Kind:         kind,
		TextPatterns: yr.TextPatterns,
		SkipTest:     yr.SkipTest,
		SkipMain:     yr.SkipMain,
		CodeOnly:     yr.CodeOnly,
		SkipLangs:    yr.SkipLangs,
	}, nil
}
