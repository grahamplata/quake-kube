// Package quakenet provides a client for querying Quake 3 servers using the
// Quake UDP protocol. This is used to get real-time status information
// including player counts from running game servers.
package quakenet

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"time"
)

const (
	// OutOfBandHeader is the Quake 3 out-of-band packet marker
	OutOfBandHeader = "\xff\xff\xff\xff"
	// GetStatusCommand queries server status including players
	GetStatusCommand = "getstatus"
	// DefaultTimeout for UDP queries
	DefaultTimeout = 3 * time.Second
)

// Player represents a player connected to a Quake server
type Player struct {
	Name  string
	Ping  int
	Score int
}

// StatusResponse contains the server status and player information
type StatusResponse struct {
	Configuration map[string]string
	Players       []Player
}

// Client provides methods for querying Quake 3 servers
type Client struct {
	Timeout time.Duration
}

// NewClient creates a new Quake protocol client with default settings
func NewClient() *Client {
	return &Client{
		Timeout: DefaultTimeout,
	}
}

// GetStatus queries a Quake 3 server for its status including player list
func (c *Client) GetStatus(addr string) (*StatusResponse, error) {
	resp, err := c.sendCommand(addr, GetStatusCommand)
	if err != nil {
		return nil, err
	}
	return parseStatusResponse(resp)
}

// GetPlayerCount is a convenience method that returns just the player count
func (c *Client) GetPlayerCount(addr string) (int, error) {
	status, err := c.GetStatus(addr)
	if err != nil {
		return 0, err
	}
	return len(status.Players), nil
}

// sendCommand sends a UDP command to a Quake server and returns the response
func (c *Client) sendCommand(addr, cmd string) ([]byte, error) {
	raddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("resolving address %s: %w", addr, err)
	}

	conn, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		return nil, fmt.Errorf("creating UDP socket: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	timeout := c.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("setting deadline: %w", err)
	}

	packet := []byte(OutOfBandHeader + cmd)
	_, err = conn.WriteTo(packet, raddr)
	if err != nil {
		return nil, fmt.Errorf("sending command: %w", err)
	}

	buffer := make([]byte, 64*1024) // 64KB should be plenty
	n, _, err := conn.ReadFrom(buffer)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return buffer[:n], nil
}

// parseStatusResponse parses the getstatus response into a StatusResponse
func parseStatusResponse(resp []byte) (*StatusResponse, error) {
	data := bytes.TrimSuffix(resp, []byte("\n"))
	parts := bytes.SplitN(data, []byte("\n"), 3)

	switch len(parts) {
	case 2:
		// No players
		return &StatusResponse{
			Configuration: parseConfigMap(parts[1]),
			Players:       []Player{},
		}, nil
	case 3:
		// Has players
		return &StatusResponse{
			Configuration: parseConfigMap(parts[1]),
			Players:       parsePlayers(parts[2]),
		}, nil
	default:
		return nil, fmt.Errorf("unexpected response format: got %d parts", len(parts))
	}
}

// parseConfigMap parses the backslash-separated configuration string
func parseConfigMap(data []byte) map[string]string {
	// Skip any leading line (header)
	if i := bytes.Index(data, []byte("\n")); i >= 0 {
		data = data[i+1:]
	}
	data = bytes.TrimPrefix(data, []byte("\\"))
	data = bytes.TrimSuffix(data, []byte("\n"))

	parts := bytes.Split(data, []byte("\\"))
	m := make(map[string]string)
	for i := 0; i < len(parts)-1; i += 2 {
		m[string(parts[i])] = string(parts[i+1])
	}
	return m
}

// parsePlayers parses the player lines from the status response
func parsePlayers(data []byte) []Player {
	players := []Player{}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		// Format: <score> <ping> "<name>"
		parts := bytes.SplitN(line, []byte(" "), 3)
		if len(parts) != 3 {
			continue
		}

		score, err := strconv.Atoi(string(parts[0]))
		if err != nil {
			continue // Skip malformed lines
		}

		ping, err := strconv.Atoi(string(parts[1]))
		if err != nil {
			continue
		}

		name, err := strconv.Unquote(string(parts[2]))
		if err != nil {
			// Try without unquoting if it fails
			name = string(bytes.Trim(parts[2], "\""))
		}

		players = append(players, Player{
			Name:  name,
			Ping:  ping,
			Score: score,
		})
	}
	return players
}
