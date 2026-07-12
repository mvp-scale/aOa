package kglab

import "github.com/corey/aoa/internal/domain/arch"

// factset.go — provenance-stamped, concept-typed facts grouped into named sets
// ("real", "target"). A FactSet is one WORLD; provenance is HOW-TRUE each fact is.
// These two axes are independent: the "real" set holds REAL facts today, the
// "target" set holds DECLARED facts — same shape, different producer.

// FactProv is a fact's trust stamp.
type FactProv string

const (
	ProvREAL     FactProv = "REAL"     // parsed/derived from the actual code
	ProvDECLARED FactProv = "DECLARED" // authored by a human/agent as intent
)

// ConceptFact is one edge fact, typed by its ontology concept.
type ConceptFact struct {
	Concept  string   `json:"concept"`   // an Ontology Name (e.g. "import")
	FromUnit string   `json:"from_unit"`
	ToUnit   string   `json:"to_unit"`   // "" for RoleNode concepts
	Prov     FactProv `json:"prov"`
	File     string   `json:"file,omitempty"`
	Line     uint32   `json:"line,omitempty"`
}

// key uniquely identifies a fact by concept + endpoints (ignores provenance).
func (f ConceptFact) key() string { return f.Concept + "|" + f.FromUnit + "|" + f.ToUnit }

// FactSet is a named world of concept facts.
type FactSet struct {
	Name  string        `json:"name"`
	Facts []ConceptFact `json:"facts"`
}

// NewFactSet returns an empty named set.
func NewFactSet(name string) FactSet { return FactSet{Name: name} }

// Add appends a fact.
func (fs *FactSet) Add(f ConceptFact) { fs.Facts = append(fs.Facts, f) }

// Len reports the fact count.
func (fs FactSet) Len() int { return len(fs.Facts) }

// depKind returns the ontology edge token for a DepFact ("" means "imports").
func depKind(d arch.DepFact) string {
	if d.Kind == "" {
		return "imports"
	}
	return d.Kind
}

// tokenToConcept maps an edge wire token to its ontology concept name.
func tokenToConcept(token string) string {
	if c, ok := OntologyByEdgeToken(token); ok {
		return c.Name
	}
	return token
}

// FactSetFromDeps bridges arch.DepFact edges into a REAL, concept-typed fact set.
// Every DepFact becomes a ConceptFact carrying its backing file:line, so a drift
// VIOLATION on a real edge is actionable (an agent navigates straight to it).
func FactSetFromDeps(name string, deps []arch.DepFact) FactSet {
	fs := NewFactSet(name)
	for _, d := range deps {
		fs.Add(ConceptFact{
			Concept:  tokenToConcept(depKind(d)),
			FromUnit: d.FromUnit,
			ToUnit:   d.ToUnit,
			Prov:     ProvREAL,
			File:     d.File,
			Line:     d.Line,
		})
	}
	return fs
}
