package quakenet

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.Timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, client.Timeout)
	}
}

func TestParseStatusResponse_NoPlayers(t *testing.T) {
	// Simulated response with no players (2 parts)
	// Format: <header>\n<config>
	resp := []byte("\xff\xff\xff\xffstatusResponse\n\\sv_hostname\\Test Server\\mapname\\q3dm1")

	status, err := parseStatusResponse(resp)
	if err != nil {
		t.Fatalf("parseStatusResponse failed: %v", err)
	}

	if len(status.Players) != 0 {
		t.Errorf("expected 0 players, got %d", len(status.Players))
	}

	if status.Configuration["sv_hostname"] != "Test Server" {
		t.Errorf("expected hostname 'Test Server', got '%s'", status.Configuration["sv_hostname"])
	}
}

func TestParseStatusResponse_WithPlayers(t *testing.T) {
	// Simulated response with players (3 parts)
	// Format: <header>\n<config>\n<player1>\n<player2>...
	resp := []byte("\xff\xff\xff\xffstatusResponse\n\\sv_hostname\\Test Server\\mapname\\q3dm1\n10 50 \"Player1\"\n5 30 \"Player2\"")

	status, err := parseStatusResponse(resp)
	if err != nil {
		t.Fatalf("parseStatusResponse failed: %v", err)
	}

	if len(status.Players) != 2 {
		t.Errorf("expected 2 players, got %d", len(status.Players))
	}

	if status.Players[0].Name != "Player1" {
		t.Errorf("expected player 1 name 'Player1', got '%s'", status.Players[0].Name)
	}
	if status.Players[0].Score != 10 {
		t.Errorf("expected player 1 score 10, got %d", status.Players[0].Score)
	}
	if status.Players[0].Ping != 50 {
		t.Errorf("expected player 1 ping 50, got %d", status.Players[0].Ping)
	}

	if status.Players[1].Name != "Player2" {
		t.Errorf("expected player 2 name 'Player2', got '%s'", status.Players[1].Name)
	}
}

func TestParsePlayers(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []Player
	}{
		{
			name:     "empty input",
			input:    []byte(""),
			expected: []Player{},
		},
		{
			name:  "single player",
			input: []byte("10 50 \"TestPlayer\""),
			expected: []Player{
				{Name: "TestPlayer", Score: 10, Ping: 50},
			},
		},
		{
			name:  "multiple players",
			input: []byte("10 50 \"Player1\"\n20 30 \"Player2\"\n-5 100 \"Player3\""),
			expected: []Player{
				{Name: "Player1", Score: 10, Ping: 50},
				{Name: "Player2", Score: 20, Ping: 30},
				{Name: "Player3", Score: -5, Ping: 100},
			},
		},
		{
			name:  "player with spaces in name",
			input: []byte("15 45 \"Player With Spaces\""),
			expected: []Player{
				{Name: "Player With Spaces", Score: 15, Ping: 45},
			},
		},
		{
			name:     "malformed line skipped",
			input:    []byte("10 50\n20 30 \"ValidPlayer\""),
			expected: []Player{{Name: "ValidPlayer", Score: 20, Ping: 30}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			players := parsePlayers(tt.input)

			if len(players) != len(tt.expected) {
				t.Fatalf("expected %d players, got %d", len(tt.expected), len(players))
			}

			for i, p := range players {
				if p.Name != tt.expected[i].Name {
					t.Errorf("player %d: expected name %q, got %q", i, tt.expected[i].Name, p.Name)
				}
				if p.Score != tt.expected[i].Score {
					t.Errorf("player %d: expected score %d, got %d", i, tt.expected[i].Score, p.Score)
				}
				if p.Ping != tt.expected[i].Ping {
					t.Errorf("player %d: expected ping %d, got %d", i, tt.expected[i].Ping, p.Ping)
				}
			}
		})
	}
}

func TestParseConfigMap(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected map[string]string
	}{
		{
			name:     "empty input",
			input:    []byte(""),
			expected: map[string]string{},
		},
		{
			name:  "simple config",
			input: []byte("\\key1\\value1\\key2\\value2"),
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:  "config with header line",
			input: []byte("statusResponse\n\\sv_hostname\\Test Server\\mapname\\q3dm1"),
			expected: map[string]string{
				"sv_hostname": "Test Server",
				"mapname":     "q3dm1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseConfigMap(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d keys, got %d", len(tt.expected), len(result))
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("key %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}
