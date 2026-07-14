package arch

import "strings"

// roleFor infers a canonical architectural ROLE for a group label, using common
// developer vocabulary (the LOCKED architecture glossary v1). It returns:
//   - layer: a canonical_layers name that view-standards.json pins to a color
//     (core→blue, edge→cyan, integration→orange, data→green, external→grey,
//     supporting→neutral).
//   - ico:   an icon key the viewer resolves (hexagon/iface/plug/cylinder/cloud/gear).
//
// This is a best-effort, "just enough" heuristic (never 100%; default is core).
// It maps the readable dictionary word a developer already uses onto one of the
// six spine roles. Precedence matters: external → data → integration → edge →
// supporting → core, so "httpapi" (adapter) wins over the bare "api" (boundary).
func roleFor(label string) (layer, ico string) {
	l := strings.ToLower(strings.TrimPrefix(label, "@"))
	switch {
	case matchAny(l, "ext:", "external", "vendor", "third", "node_modules", "site-packages") || l == "std":
		return "external", "cloud"
	case matchAny(l, "store", "storage", "persist", "database", "db", "repo", "dao",
		"model", "entit", "cache", "queue", "eventbus", "msgbus", "pubsub", "broker",
		"data", "sql", "mongo", "redis", "vector", "embedding"):
		return "data", "cylinder"
	case matchAny(l, "adapter", "handler", "http", "grpc", "rest", "client", "gateway",
		"middleware", "transport", "rpc", "web", "driver", "connector",
		"integration", "consumer", "producer", "publisher", "subscriber"):
		return "integration", "plug"
	case matchAny(l, "port", "interface", "iface", "contract", "schema", "proto",
		"api", "openapi", "route", "controller", "endpoint", "resolver", "spec"):
		return "edge", "iface"
	case matchAny(l, "cmd", "app", "config", "wiring", "bootstrap", "setup",
		"container", "inject", "entrypoint"):
		return "supporting", "gear"
	case matchAny(l, "domain", "core", "service", "logic", "usecase", "use_case",
		"business", "engine", "kernel", "compute"):
		return "core", "hexagon"
	default:
		return "core", "hexagon"
	}
}

// matchAny reports whether s contains any of the given substrings.
func matchAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
