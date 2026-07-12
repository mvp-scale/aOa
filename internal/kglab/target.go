package kglab

import "fmt"

// target.go — authoring the TARGET world: where-we-need-to-be, declared by a
// human or agent (NOT parsed from code, NOT a PRD parser). Every target fact is
// DECLARED provenance with no file:line — it is intent, not measured reality.

// TargetFact is one authored intent edge. Validated against the ontology.
type TargetFact struct {
	Concept  string `json:"concept"`
	FromUnit string `json:"from_unit"`
	ToUnit   string `json:"to_unit"`
}

// LoadTarget validates authored facts against the ontology and returns a DECLARED
// FactSet. It rejects any concept not in the 16-concept ontology — you cannot
// declare a target in a vocabulary the graph does not understand.
func LoadTarget(name string, facts []TargetFact) (FactSet, error) {
	fs := NewFactSet(name)
	for _, t := range facts {
		if _, ok := OntologyByName(t.Concept); !ok {
			return FactSet{}, fmt.Errorf("kglab: target declares unknown concept %q (not in the 16-concept ontology)", t.Concept)
		}
		fs.Add(ConceptFact{
			Concept:  t.Concept,
			FromUnit: t.FromUnit,
			ToUnit:   t.ToUnit,
			Prov:     ProvDECLARED, // intent — no file:line, it is not real yet
		})
	}
	return fs, nil
}
