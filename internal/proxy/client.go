package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Route represents a registered route in Caddy.
type Route struct {
	ID       string `json:"@id,omitempty"`
	Hostname string `json:"-"`
	Port     int    `json:"-"`
}

type caddyRoute struct {
	ID     string         `json:"@id"`
	Match  []caddyMatch   `json:"match"`
	Handle []caddyHandler `json:"handle"`
}

type caddyMatch struct {
	Host []string `json:"host"`
}

type caddyHandler struct {
	Handler   string          `json:"handler"`
	Upstreams []caddyUpstream `json:"upstreams"`
}

type caddyUpstream struct {
	Dial string `json:"dial"`
}

// Client talks to the Caddy admin API.
type Client struct {
	adminAPI string
	http     *http.Client
}

// NewClient creates a client using the default admin address.
func NewClient() *Client {
	return NewClientWithAddr(DefaultConfig().adminURL())
}

// NewClientWithAddr creates a client with a custom admin address.
func NewClientWithAddr(adminAPI string) *Client {
	return &Client{adminAPI: adminAPI, http: &http.Client{}}
}

func (c *Client) routesURL() string { return c.adminAPI + "/config/apps/http/servers/devup/routes" }
func (c *Client) idURL(id string) string { return c.adminAPI + "/id/" + id }

// Register adds or updates a route in Caddy for the given hostname → port.
// Register adds a route using localhost as the dial address.
func (c *Client) Register(hostname string, port int) error {
	return c.RegisterDial(hostname, fmt.Sprintf("localhost:%d", port))
}

// RegisterDial adds a route with an explicit dial address (e.g. "[::1]:8080" or "127.0.0.1:8080").
func (c *Client) RegisterDial(hostname string, dial string) error {
	c.Deregister(hostname)

	route := caddyRoute{
		ID:    hostname,
		Match: []caddyMatch{{Host: []string{hostname, "*." + hostname}}},
		Handle: []caddyHandler{{
			Handler:   "reverse_proxy",
			Upstreams: []caddyUpstream{{Dial: dial}},
		}},
	}

	body, err := json.Marshal(route)
	if err != nil {
		return err
	}

	resp, err := c.http.Post(c.routesURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("register route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register route: status %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

// Deregister removes a route from Caddy by hostname (used as @id).
func (c *Client) Deregister(hostname string) error {
	req, err := http.NewRequest(http.MethodDelete, c.idURL(hostname), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("deregister route: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// ListRoutes returns all registered routes from Caddy.
func (c *Client) ListRoutes() ([]Route, error) {
	resp, err := c.http.Get(c.routesURL())
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	defer resp.Body.Close()

	var rawRoutes []caddyRoute
	if err := json.NewDecoder(resp.Body).Decode(&rawRoutes); err != nil {
		return nil, fmt.Errorf("decode routes: %w", err)
	}

	var routes []Route
	for _, r := range rawRoutes {
		hostname := ""
		if len(r.Match) > 0 && len(r.Match[0].Host) > 0 {
			hostname = r.Match[0].Host[0]
		}
		port := 0
		if len(r.Handle) > 0 && len(r.Handle[0].Upstreams) > 0 {
			fmt.Sscanf(r.Handle[0].Upstreams[0].Dial, "localhost:%d", &port)
		}
		routes = append(routes, Route{
			ID:       r.ID,
			Hostname: hostname,
			Port:     port,
		})
	}
	return routes, nil
}
