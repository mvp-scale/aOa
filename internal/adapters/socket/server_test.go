package socket

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// S-07: Unix Socket Daemon — JSON-over-socket protocol for search, health, shutdown
// =============================================================================

// testFixtures creates a search engine and index for testing.
func testFixtures() (*index.SearchEngine, *ports.Index) {
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"login": {
				{FileID: 1, Line: 10},
				{FileID: 2, Line: 25},
			},
			"handler": {
				{FileID: 1, Line: 10},
			},
			"session": {
				{FileID: 3, Line: 5},
			},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {
				Name:      "login",
				Signature: "login(self, user, password)",
				Kind:      "function",
				StartLine: 10,
				EndLine:   25,
				Parent:    "AuthHandler",
				Tags:      []string{"auth", "login"},
			},
			{FileID: 2, Line: 25}: {
				Name:      "login_view",
				Signature: "login_view(request)",
				Kind:      "function",
				StartLine: 25,
				EndLine:   40,
				Tags:      []string{"login", "view"},
			},
			{FileID: 3, Line: 5}: {
				Name:      "SessionManager",
				Signature: "class SessionManager(object)",
				Kind:      "class",
				StartLine: 5,
				EndLine:   80,
			},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "services/auth/handler.py", Language: "python", Domain: "@authentication"},
			2: {Path: "views/login.py", Language: "python", Domain: "@api"},
			3: {Path: "services/session/manager.py", Language: "python", Domain: "@authentication"},
		},
	}

	domains := map[string]index.Domain{
		"@authentication": {Terms: map[string][]string{"auth": {"login", "session"}}},
		"@api":            {Terms: map[string][]string{"api": {"view", "handler"}}},
	}

	engine := index.NewSearchEngine(idx, domains)
	return engine, idx
}

// testSocketPath returns a unique socket path for a test.
func testSocketPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.sock")
}

func TestServer_SearchRoundtrip(t *testing.T) {
	engine, idx := testFixtures()
	sockPath := testSocketPath(t)

	srv := NewServer(engine, idx, sockPath, nil)
	require.NoError(t, srv.Start())
	defer srv.Stop()

	client := NewClient(sockPath)

	// Search for "login" — should get 2 hits
	result, err := client.Search("login", ports.SearchOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, len(result.Hits))
	assert.Equal(t, "services/auth/handler.py", result.Hits[0].File)
	assert.Equal(t, 10, result.Hits[0].Line)

	// Search for nonexistent term
	result, err = client.Search("nonexistent", ports.SearchOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, len(result.Hits))

	// Count mode
	result, err = client.Search("login", ports.SearchOptions{CountOnly: true})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Count)
}

func TestServer_Health(t *testing.T) {
	engine, idx := testFixtures()
	sockPath := testSocketPath(t)

	srv := NewServer(engine, idx, sockPath, nil)
	require.NoError(t, srv.Start())
	defer srv.Stop()

	client := NewClient(sockPath)

	health, err := client.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", health.Status)
	assert.Equal(t, 3, health.FileCount)
	assert.Equal(t, 3, health.TokenCount)
	assert.NotEmpty(t, health.Uptime)
}

func TestServer_Shutdown(t *testing.T) {
	engine, idx := testFixtures()
	sockPath := testSocketPath(t)

	srv := NewServer(engine, idx, sockPath, nil)
	require.NoError(t, srv.Start())

	client := NewClient(sockPath)

	// Verify it's running
	assert.True(t, client.Ping())

	// Send shutdown request — this closes shutdownCh (signals the daemon).
	err := client.Shutdown()
	require.NoError(t, err)

	// ShutdownCh should be closed.
	select {
	case <-srv.ShutdownCh():
		// Good — channel is closed, daemon would call Stop() here.
	default:
		t.Fatal("ShutdownCh should be closed after Shutdown request")
	}

	// The daemon is responsible for calling Stop() after receiving the signal.
	srv.Stop()

	// Socket file should be removed.
	_, err = os.Stat(sockPath)
	assert.True(t, os.IsNotExist(err), "socket file should be removed after shutdown")
}

func TestServer_ConcurrentClients(t *testing.T) {
	engine, idx := testFixtures()
	sockPath := testSocketPath(t)

	srv := NewServer(engine, idx, sockPath, nil)
	require.NoError(t, srv.Start())
	defer srv.Stop()

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	// 10 clients x 10 requests each
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := NewClient(sockPath)
			for j := 0; j < 10; j++ {
				result, err := client.Search("login", ports.SearchOptions{})
				if err != nil {
					errs <- err
					return
				}
				if len(result.Hits) != 2 {
					errs <- assert.AnError
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent client error: %v", err)
	}
}

func TestServer_StaleSocket(t *testing.T) {
	sockPath := testSocketPath(t)

	// Create a stale socket file (not a real listener)
	require.NoError(t, os.WriteFile(sockPath, []byte("stale"), 0600))

	engine, idx := testFixtures()
	srv := NewServer(engine, idx, sockPath, nil)
	err := srv.Start()
	require.NoError(t, err, "should replace stale socket")
	defer srv.Stop()

	// Verify it works
	client := NewClient(sockPath)
	health, err := client.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", health.Status)
}
