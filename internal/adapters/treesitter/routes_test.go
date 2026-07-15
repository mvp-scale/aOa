//go:build !lean

package treesitter

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractRoutesGo_Gin verifies gin idiom extraction: router.VERB(path,
// handler) calls inside func main(), including the "Any" catch-all.
func TestExtractRoutesGo_Gin(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/ping", pingHandler)
	r.POST("/users", createUser)
	r.Any("/health", healthHandler)
	r.Run()
}
`)

	tree, langName, err := p.ParseToTree("main.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	routes := extractRoutesGo(tree.RootNode(), source, "main.go")
	require.Len(t, routes, 3)

	byPath := make(map[string]ports.RouteEdge)
	for _, r := range routes {
		byPath[r.Path] = r
		assert.Equal(t, "main.go", r.FromFile)
		assert.Equal(t, "gin", r.Framework)
		assert.Greater(t, r.StartLine, uint32(0))
	}

	assert.Equal(t, "GET", byPath["/ping"].Method)
	assert.Equal(t, "pingHandler", byPath["/ping"].Handler)
	assert.Equal(t, "POST", byPath["/users"].Method)
	assert.Equal(t, "ANY", byPath["/health"].Method)
}

// TestExtractRoutesGo_NetHTTP verifies net/http mux idiom extraction:
// http.HandleFunc/mux.Handle calls carry no verb (honest — the source
// doesn't say one).
func TestExtractRoutesGo_NetHTTP(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", pingHandler)
	http.Handle("/static/", fileServer)
	http.ListenAndServe(":8080", mux)
}
`)

	tree, langName, err := p.ParseToTree("main.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	routes := extractRoutesGo(tree.RootNode(), source, "main.go")
	require.Len(t, routes, 2)

	for _, r := range routes {
		assert.Equal(t, "net/http", r.Framework)
		assert.Equal(t, "", r.Method, "net/http Handle/HandleFunc carry no explicit verb")
	}

	paths := map[string]bool{}
	for _, r := range routes {
		paths[r.Path] = true
	}
	assert.True(t, paths["/ping"])
	assert.True(t, paths["/static/"])
}

// TestExtractRoutesGo_NoFalsePositives verifies non-route calls (router
// construction, .Group(), .Run()) are not misclassified as routes, and that
// a route call inside a nested if-block (out of v1 bounded-descent scope)
// is honestly NOT extracted rather than silently mis-walked.
func TestExtractRoutesGo_NoFalsePositives(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	api := r.Group("/api")
	_ = api
	if true {
		r.GET("/nested", nestedHandler)
	}
	r.Run()
}
`)

	tree, langName, err := p.ParseToTree("main.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	routes := extractRoutesGo(tree.RootNode(), source, "main.go")
	assert.Empty(t, routes, "Group/Run are not routes; nested-if route is out of bounded-descent scope (documented v1 cut)")
}

// TestExtractRoutesGo_EmptyFile verifies no panic/edges on a route-less file.
func TestExtractRoutesGo_EmptyFile(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

func main() {}
`)

	tree, langName, err := p.ParseToTree("main.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	routes := extractRoutesGo(tree.RootNode(), source, "main.go")
	assert.Empty(t, routes)
}

// TestParser_ExtractRoutes_EndToEnd exercises the public Parser.ExtractRoutes
// entry point (its own parse pass, ports.RouteExtractor implementation).
func TestParser_ExtractRoutes_EndToEnd(t *testing.T) {
	p := NewParser()

	source := []byte(`package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/ping", pingHandler)
	r.Run()
}
`)

	routes, err := p.ExtractRoutes("main.go", source)
	require.NoError(t, err)
	require.Len(t, routes, 1)
	assert.Equal(t, "GET", routes[0].Method)
	assert.Equal(t, "/ping", routes[0].Path)
	assert.Equal(t, "gin", routes[0].Framework)
}

// TestParser_ExtractRoutes_UnknownLang verifies no error/panic for an
// unsupported language (route extraction is Go-only for v1).
func TestParser_ExtractRoutes_UnknownLang(t *testing.T) {
	p := NewParser()
	routes, err := p.ExtractRoutes("app.py", []byte("import flask\n"))
	require.NoError(t, err)
	assert.Nil(t, routes)
}
