package socket

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/corey/aoa/internal/ports"
)

// Client connects to the aOa daemon over a Unix socket.
type Client struct {
	sockPath string
}

// NewClient creates a client that will connect to the given socket path.
func NewClient(sockPath string) *Client {
	return &Client{sockPath: sockPath}
}

// Search sends a search request and returns the result.
func (c *Client) Search(query string, opts ports.SearchOptions) (*SearchResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodSearch,
		Params: SearchParams{Query: query, Options: opts},
	})
	if err != nil {
		return nil, err
	}
	var result SearchResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal search result: %w", err)
	}
	return &result, nil
}

// Health sends a health check request.
func (c *Client) Health() (*HealthResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodHealth,
	})
	if err != nil {
		return nil, err
	}
	var result HealthResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal health result: %w", err)
	}
	return &result, nil
}

// Shutdown sends a shutdown request to the daemon.
func (c *Client) Shutdown() error {
	_, err := c.call(Request{
		ID:     "1",
		Method: MethodShutdown,
	})
	return err
}

// Files sends a files request with optional glob or name filter.
func (c *Client) Files(glob, name string) (*FilesResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodFiles,
		Params: FilesParams{Glob: glob, Name: name},
	})
	if err != nil {
		return nil, err
	}
	var result FilesResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal files result: %w", err)
	}
	return &result, nil
}

// Domains sends a domains request.
func (c *Client) Domains() (*DomainsResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodDomains,
	})
	if err != nil {
		return nil, err
	}
	var result DomainsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal domains result: %w", err)
	}
	return &result, nil
}

// Bigrams sends a bigrams request.
func (c *Client) Bigrams() (*BigramsResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodBigrams,
	})
	if err != nil {
		return nil, err
	}
	var result BigramsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal bigrams result: %w", err)
	}
	return &result, nil
}

// Stats sends a stats request.
func (c *Client) Stats() (*StatsResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodStats,
	})
	if err != nil {
		return nil, err
	}
	var result StatsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal stats result: %w", err)
	}
	return &result, nil
}

// Reindex sends a reindex request to the daemon with an extended timeout.
func (c *Client) Reindex() (*ReindexResult, error) {
	raw, err := c.callWithTimeout(Request{
		ID:     "1",
		Method: MethodReindex,
	}, 120*time.Second)
	if err != nil {
		return nil, err
	}
	var result ReindexResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal reindex result: %w", err)
	}
	return &result, nil
}

// Peek sends a peek request to resolve codes to method source.
func (c *Client) Peek(codes []string) (*PeekResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodPeek,
		Params: PeekParams{Codes: codes},
	})
	if err != nil {
		return nil, err
	}
	var result PeekResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal peek result: %w", err)
	}
	return &result, nil
}

// Wipe sends a wipe request to clear all project data.
func (c *Client) Wipe() error {
	_, err := c.call(Request{
		ID:     "1",
		Method: MethodWipe,
	})
	return err
}

// ── L19.16 arch client methods ────────────────────────────────────────────────

// ArchViews sends an arch.views request and returns the manifest result.
func (c *Client) ArchViews(scope string) (*ArchViewsResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodArchViews,
		Params: ArchViewsParams{Scope: scope},
	})
	if err != nil {
		return nil, err
	}
	var result ArchViewsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal arch.views result: %w", err)
	}
	return &result, nil
}

// ArchView sends an arch.view request and returns the shard result.
//
// PF2 (L19.19 byte-parity AC): callWithTimeout decodes the response into a
// wireResponse with Result as json.RawMessage. This preserves the stored shard
// bytes verbatim so ArchViewResult.Raw is byte-identical to the HTTP shard body.
// The two-step marshal→unmarshal via interface{} (which reordered JSON keys) is gone.
func (c *Client) ArchView(scope, view string) (*ArchViewResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodArchView,
		Params: ArchViewParams{Scope: scope, View: view},
	})
	if err != nil {
		return nil, err
	}
	var result ArchViewResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal arch.view result: %w", err)
	}
	return &result, nil
}

// ArchFindings sends an arch.findings request.
func (c *Client) ArchFindings(scope string) (*ArchFindingsResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodArchFindings,
		Params: ArchFindingsParams{Scope: scope},
	})
	if err != nil {
		return nil, err
	}
	var result ArchFindingsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal arch.findings result: %w", err)
	}
	return &result, nil
}

// ArchJourney sends an arch.journey request (stub — not yet implemented).
func (c *Client) ArchJourney(id string, list bool) error {
	_, err := c.call(Request{
		ID:     "1",
		Method: MethodArchJourney,
		Params: ArchJourneyParams{ID: id, List: list},
	})
	return err
}

// ArchDerive sends an arch.derive request and returns the path result.
// from/to are unit IDs (e.g. "u_internal_app").
func (c *Client) ArchDerive(scope, from, to string, k int) (*ArchDeriveResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodArchDerive,
		Params: ArchDeriveParams{Scope: scope, From: from, To: to, K: k},
	})
	if err != nil {
		return nil, err
	}
	var result ArchDeriveResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal arch.derive result: %w", err)
	}
	return &result, nil
}

// ArchFacts sends an arch.facts request and returns the facts result.
func (c *Client) ArchFacts(scope, subject string, limit int) (*ArchFactsResult, error) {
	raw, err := c.call(Request{
		ID:     "1",
		Method: MethodArchFacts,
		Params: ArchFactsParams{Scope: scope, Subject: subject, Limit: limit},
	})
	if err != nil {
		return nil, err
	}
	var result ArchFactsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal arch.facts result: %w", err)
	}
	return &result, nil
}

// Ping checks if the daemon is alive via an application-level health round-trip.
// Returns true only if the daemon responds to a health request within 1 second.
// Detects zombie daemons where the socket accepts connections but the accept loop is dead.
func (c *Client) Ping() bool {
	_, err := c.callWithTimeout(Request{
		ID:     "ping",
		Method: MethodHealth,
	}, 1*time.Second)
	return err == nil
}

// PingTCP checks if the daemon socket is connectable (TCP-level only).
// Faster than Ping() but cannot detect zombie daemons. Used for startup
// polling where speed matters and the daemon was just spawned.
func (c *Client) PingTCP() bool {
	conn, err := net.DialTimeout("unix", c.sockPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// wireResponse is the client-side decoder for daemon wire responses.
// Result is json.RawMessage to preserve byte order and content verbatim —
// the two-step marshal(interface{})→unmarshal that reordered JSON keys is gone.
// This is the PF2 fix: L19.19 byte-parity AC (CLI JSON == browser shard).
type wireResponse struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (c *Client) call(req Request) (json.RawMessage, error) {
	return c.callWithTimeout(req, 5*time.Second)
}

func (c *Client) callWithTimeout(req Request, timeout time.Duration) (json.RawMessage, error) {
	conn, err := net.DialTimeout("unix", c.sockPath, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	// Set deadline for the whole request/response
	conn.SetDeadline(time.Now().Add(timeout))

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// Read response
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return nil, fmt.Errorf("empty response")
	}

	var resp wireResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("server error: %s", resp.Error)
	}
	return resp.Result, nil
}
