package controller

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	quakekubeiov1alpha1 "github.com/grahamplata/quake-kube/operator/api/v1alpha1"
)

func TestIsPaused(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name:        "no annotations",
			annotations: nil,
			want:        false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name: "pause annotation set to true",
			annotations: map[string]string{
				"quakekube.io/paused": "true",
			},
			want: true,
		},
		{
			name: "pause annotation set to false",
			annotations: map[string]string{
				"quakekube.io/paused": "false",
			},
			want: false,
		},
		{
			name: "pause annotation with wrong value",
			annotations: map[string]string{
				"quakekube.io/paused": "yes",
			},
			want: false,
		},
		{
			name: "pause annotation empty string",
			annotations: map[string]string{
				"quakekube.io/paused": "",
			},
			want: false,
		},
		{
			name: "other annotations present without pause",
			annotations: map[string]string{
				"other-annotation": "value",
			},
			want: false,
		},
		{
			name: "pause annotation among other annotations",
			annotations: map[string]string{
				"other-annotation":    "value",
				"quakekube.io/paused": "true",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-server",
					Annotations: tt.annotations,
				},
			}

			if got := isPaused(server); got != tt.want {
				t.Errorf("isPaused() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

func TestMergeServerConfig(t *testing.T) {
	tests := []struct {
		name     string
		base     *quakekubeiov1alpha1.ServerConfigSpec
		override *quakekubeiov1alpha1.ServerConfigSpec
		check    func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec)
	}{
		{
			name:     "nil base returns override copy",
			base:     nil,
			override: &quakekubeiov1alpha1.ServerConfigSpec{FragLimit: intPtr(30)},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if result.FragLimit == nil || *result.FragLimit != 30 {
					t.Errorf("expected FragLimit 30, got %v", result.FragLimit)
				}
			},
		},
		{
			name:     "nil override returns base copy",
			base:     &quakekubeiov1alpha1.ServerConfigSpec{FragLimit: intPtr(25)},
			override: nil,
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if result.FragLimit == nil || *result.FragLimit != 25 {
					t.Errorf("expected FragLimit 25, got %v", result.FragLimit)
				}
			},
		},
		{
			name:     "override takes precedence for FragLimit",
			base:     &quakekubeiov1alpha1.ServerConfigSpec{FragLimit: intPtr(25)},
			override: &quakekubeiov1alpha1.ServerConfigSpec{FragLimit: intPtr(50)},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if result.FragLimit == nil || *result.FragLimit != 50 {
					t.Errorf("expected FragLimit 50 (override), got %v", result.FragLimit)
				}
			},
		},
		{
			name: "base value preserved when override not set",
			base: &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(25),
				TimeLimit: &metav1.Duration{Duration: 15 * time.Minute},
			},
			override: &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(50),
				// TimeLimit not set
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if result.FragLimit == nil || *result.FragLimit != 50 {
					t.Errorf("expected FragLimit 50, got %v", result.FragLimit)
				}
				if result.TimeLimit == nil || result.TimeLimit.Duration != 15*time.Minute {
					t.Errorf("expected TimeLimit 15m from base, got %v", result.TimeLimit)
				}
			},
		},
		{
			name: "maps from override replace base maps",
			base: &quakekubeiov1alpha1.ServerConfigSpec{
				Maps: []quakekubeiov1alpha1.MapSpec{
					{Name: "q3dm1"},
					{Name: "q3dm2"},
				},
			},
			override: &quakekubeiov1alpha1.ServerConfigSpec{
				Maps: []quakekubeiov1alpha1.MapSpec{
					{Name: "q3dm7"},
				},
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if len(result.Maps) != 1 {
					t.Errorf("expected 1 map, got %d", len(result.Maps))
				}
				if result.Maps[0].Name != "q3dm7" {
					t.Errorf("expected map q3dm7, got %s", result.Maps[0].Name)
				}
			},
		},
		{
			name: "empty override maps preserves base maps",
			base: &quakekubeiov1alpha1.ServerConfigSpec{
				Maps: []quakekubeiov1alpha1.MapSpec{
					{Name: "q3dm1"},
				},
			},
			override: &quakekubeiov1alpha1.ServerConfigSpec{
				Maps: []quakekubeiov1alpha1.MapSpec{},
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if len(result.Maps) != 1 {
					t.Errorf("expected 1 map from base, got %d", len(result.Maps))
				}
			},
		},
		{
			name: "game config merge - override type",
			base: &quakekubeiov1alpha1.ServerConfigSpec{
				Game: &quakekubeiov1alpha1.GameConfigSpec{
					Type: quakekubeiov1alpha1.FreeForAll,
					MOTD: "Welcome!",
				},
			},
			override: &quakekubeiov1alpha1.ServerConfigSpec{
				Game: &quakekubeiov1alpha1.GameConfigSpec{
					Type: quakekubeiov1alpha1.CaptureTheFlag,
				},
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if result.Game == nil {
					t.Fatal("expected Game config")
				}
				if result.Game.Type != quakekubeiov1alpha1.CaptureTheFlag {
					t.Errorf("expected CTF game type, got %s", result.Game.Type)
				}
				if result.Game.MOTD != "Welcome!" {
					t.Errorf("expected MOTD from base, got %s", result.Game.MOTD)
				}
			},
		},
		{
			name: "server settings merge",
			base: &quakekubeiov1alpha1.ServerConfigSpec{
				Server: &quakekubeiov1alpha1.ServerSettings{
					Hostname:   "Base Server",
					MaxClients: intPtr(12),
				},
			},
			override: &quakekubeiov1alpha1.ServerConfigSpec{
				Server: &quakekubeiov1alpha1.ServerSettings{
					Hostname: "Override Server",
				},
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if result.Server.Hostname != "Override Server" {
					t.Errorf("expected hostname from override, got %s", result.Server.Hostname)
				}
				if result.Server.MaxClients == nil || *result.Server.MaxClients != 12 {
					t.Errorf("expected MaxClients 12 from base, got %v", result.Server.MaxClients)
				}
			},
		},
		{
			name: "bot config merge",
			base: &quakekubeiov1alpha1.ServerConfigSpec{
				Bot: &quakekubeiov1alpha1.BotConfigSpec{
					MinPlayers: intPtr(4),
					Skill:      intPtr(3),
				},
			},
			override: &quakekubeiov1alpha1.ServerConfigSpec{
				Bot: &quakekubeiov1alpha1.BotConfigSpec{
					Skill: intPtr(5),
				},
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if result.Bot.MinPlayers == nil || *result.Bot.MinPlayers != 4 {
					t.Errorf("expected MinPlayers 4 from base, got %v", result.Bot.MinPlayers)
				}
				if result.Bot.Skill == nil || *result.Bot.Skill != 5 {
					t.Errorf("expected Skill 5 from override, got %v", result.Bot.Skill)
				}
			},
		},
		{
			name: "commands from override replace base",
			base: &quakekubeiov1alpha1.ServerConfigSpec{
				Commands: []string{"cmd1", "cmd2"},
			},
			override: &quakekubeiov1alpha1.ServerConfigSpec{
				Commands: []string{"cmd3"},
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerConfigSpec) {
				if len(result.Commands) != 1 || result.Commands[0] != "cmd3" {
					t.Errorf("expected commands from override, got %v", result.Commands)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeServerConfig(tt.base, tt.override)
			tt.check(t, result)
		})
	}
}

func TestGenerateConfigYAML(t *testing.T) {
	tests := []struct {
		name     string
		config   *quakekubeiov1alpha1.ServerConfigSpec
		contains []string
		excludes []string
	}{
		{
			name:     "nil config returns defaults",
			config:   nil,
			contains: []string{"fragLimit: 25", "timeLimit: 15m"},
		},
		{
			name: "custom frag limit",
			config: &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(50),
			},
			contains: []string{"fragLimit: 50"},
		},
		{
			name: "bot configuration",
			config: &quakekubeiov1alpha1.ServerConfigSpec{
				Bot: &quakekubeiov1alpha1.BotConfigSpec{
					MinPlayers: intPtr(4),
					Skill:      intPtr(3),
					NoChat:     true,
				},
			},
			contains: []string{"bot:", "minPlayers: 4", "skill: 3", "noChat: true"},
		},
		{
			name: "game configuration",
			config: &quakekubeiov1alpha1.ServerConfigSpec{
				Game: &quakekubeiov1alpha1.GameConfigSpec{
					Type:         quakekubeiov1alpha1.CaptureTheFlag,
					MOTD:         "Welcome to CTF!",
					ForceRespawn: true,
					QuadFactor:   intPtr(4),
				},
			},
			contains: []string{
				"game:",
				"type: CaptureTheFlag",
				`motd: "Welcome to CTF!"`,
				"forceRespawn: true",
				"quadFactor: 4",
			},
		},
		{
			name: "server settings",
			config: &quakekubeiov1alpha1.ServerConfigSpec{
				Server: &quakekubeiov1alpha1.ServerSettings{
					Hostname:     "My Quake Server",
					MaxClients:   intPtr(16),
					RconPassword: "secret",
				},
			},
			contains: []string{
				"server:",
				`hostname: "My Quake Server"`,
				"maxClients: 16",
				`password: "secret"`,
			},
		},
		{
			name: "map configuration",
			config: &quakekubeiov1alpha1.ServerConfigSpec{
				Maps: []quakekubeiov1alpha1.MapSpec{
					{Name: "q3dm7", Type: quakekubeiov1alpha1.FreeForAll},
					{Name: "q3ctf1", Type: quakekubeiov1alpha1.CaptureTheFlag, CaptureLimit: intPtr(8)},
				},
			},
			contains: []string{
				"maps:",
				"- name: q3dm7",
				"type: FreeForAll",
				"- name: q3ctf1",
				"type: CaptureTheFlag",
				"captureLimit: 8",
			},
		},
		{
			name: "commands configuration",
			config: &quakekubeiov1alpha1.ServerConfigSpec{
				Commands: []string{"sv_pure 1", "g_allowvote 1"},
			},
			contains: []string{
				"commands:",
				`- "sv_pure 1"`,
				`- "g_allowvote 1"`,
			},
		},
		{
			name: "empty bot config produces no bot section",
			config: &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(25),
			},
			excludes: []string{"bot:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateConfigYAML(tt.config)

			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("expected YAML to contain %q, got:\n%s", substr, result)
				}
			}

			for _, substr := range tt.excludes {
				if strings.Contains(result, substr) {
					t.Errorf("expected YAML to NOT contain %q, got:\n%s", substr, result)
				}
			}
		})
	}
}

func TestBuildContainerSecurityContext(t *testing.T) {
	tests := []struct {
		name   string
		custom *corev1.SecurityContext
		check  func(t *testing.T, ctx *corev1.SecurityContext)
	}{
		{
			name:   "nil custom returns secure defaults",
			custom: nil,
			check: func(t *testing.T, ctx *corev1.SecurityContext) {
				if ctx == nil {
					t.Fatal("expected non-nil security context")
				}
				if ctx.RunAsNonRoot == nil || !*ctx.RunAsNonRoot {
					t.Error("expected RunAsNonRoot to be true")
				}
				if ctx.AllowPrivilegeEscalation == nil || *ctx.AllowPrivilegeEscalation {
					t.Error("expected AllowPrivilegeEscalation to be false")
				}
				if ctx.ReadOnlyRootFilesystem == nil || !*ctx.ReadOnlyRootFilesystem {
					t.Error("expected ReadOnlyRootFilesystem to be true")
				}
				if ctx.Capabilities == nil || len(ctx.Capabilities.Drop) == 0 {
					t.Error("expected capabilities to drop ALL")
				}
				if ctx.Capabilities.Drop[0] != "ALL" {
					t.Errorf("expected to drop ALL capabilities, got %v", ctx.Capabilities.Drop)
				}
			},
		},
		{
			name: "custom context is returned as-is",
			custom: &corev1.SecurityContext{
				RunAsNonRoot: boolPtr(false),
			},
			check: func(t *testing.T, ctx *corev1.SecurityContext) {
				if ctx.RunAsNonRoot == nil || *ctx.RunAsNonRoot {
					t.Error("expected custom RunAsNonRoot=false to be preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildContainerSecurityContext(tt.custom)
			tt.check(t, result)
		})
	}
}

func TestBuildPodSecurityContext(t *testing.T) {
	tests := []struct {
		name   string
		custom *corev1.PodSecurityContext
		check  func(t *testing.T, ctx *corev1.PodSecurityContext)
	}{
		{
			name:   "nil custom returns secure defaults",
			custom: nil,
			check: func(t *testing.T, ctx *corev1.PodSecurityContext) {
				if ctx == nil {
					t.Fatal("expected non-nil pod security context")
				}
				if ctx.RunAsNonRoot == nil || !*ctx.RunAsNonRoot {
					t.Error("expected RunAsNonRoot to be true")
				}
				if ctx.RunAsUser == nil || *ctx.RunAsUser != 1000 {
					t.Errorf("expected RunAsUser 1000, got %v", ctx.RunAsUser)
				}
				if ctx.RunAsGroup == nil || *ctx.RunAsGroup != 1000 {
					t.Errorf("expected RunAsGroup 1000, got %v", ctx.RunAsGroup)
				}
				if ctx.FSGroup == nil || *ctx.FSGroup != 1000 {
					t.Errorf("expected FSGroup 1000, got %v", ctx.FSGroup)
				}
			},
		},
		{
			name: "custom context is returned as-is",
			custom: &corev1.PodSecurityContext{
				RunAsUser: int64Ptr(2000),
				FSGroup:   int64Ptr(3000),
			},
			check: func(t *testing.T, ctx *corev1.PodSecurityContext) {
				if ctx.RunAsUser == nil || *ctx.RunAsUser != 2000 {
					t.Errorf("expected custom RunAsUser 2000, got %v", ctx.RunAsUser)
				}
				if ctx.FSGroup == nil || *ctx.FSGroup != 3000 {
					t.Errorf("expected custom FSGroup 3000, got %v", ctx.FSGroup)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPodSecurityContext(tt.custom)
			tt.check(t, result)
		})
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}

func TestResourceNamingHelpers(t *testing.T) {
	server := &quakekubeiov1alpha1.QuakeServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-quake-server",
		},
	}

	t.Run("configMapName", func(t *testing.T) {
		expected := "my-quake-server-config"
		if got := configMapName(server); got != expected {
			t.Errorf("configMapName() = %s, want %s", got, expected)
		}
	})

	t.Run("serviceName", func(t *testing.T) {
		expected := "my-quake-server"
		if got := serviceName(server); got != expected {
			t.Errorf("serviceName() = %s, want %s", got, expected)
		}
	})

	t.Run("deploymentName", func(t *testing.T) {
		expected := "my-quake-server"
		if got := deploymentName(server); got != expected {
			t.Errorf("deploymentName() = %s, want %s", got, expected)
		}
	})

	t.Run("httpRouteName", func(t *testing.T) {
		expected := "my-quake-server-http"
		if got := httpRouteName(server); got != expected {
			t.Errorf("httpRouteName() = %s, want %s", got, expected)
		}
	})

	t.Run("udpRouteName", func(t *testing.T) {
		expected := "my-quake-server-udp"
		if got := udpRouteName(server); got != expected {
			t.Errorf("udpRouteName() = %s, want %s", got, expected)
		}
	})
}

func TestLabelsForServer(t *testing.T) {
	server := &quakekubeiov1alpha1.QuakeServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-server",
		},
	}

	labels := labelsForServer(server)

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "quake-server",
		"app.kubernetes.io/instance":   "test-server",
		"app.kubernetes.io/managed-by": "quake-operator",
	}

	for k, v := range expectedLabels {
		if labels[k] != v {
			t.Errorf("label %s = %s, want %s", k, labels[k], v)
		}
	}
}

func TestSelectorLabelsForServer(t *testing.T) {
	server := &quakekubeiov1alpha1.QuakeServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-server",
		},
	}

	labels := selectorLabelsForServer(server)

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":     "quake-server",
		"app.kubernetes.io/instance": "test-server",
	}

	if len(labels) != len(expectedLabels) {
		t.Errorf("selector labels count = %d, want %d", len(labels), len(expectedLabels))
	}

	for k, v := range expectedLabels {
		if labels[k] != v {
			t.Errorf("selector label %s = %s, want %s", k, labels[k], v)
		}
	}
}

func TestMergeGameConfig(t *testing.T) {
	tests := []struct {
		name     string
		base     *quakekubeiov1alpha1.GameConfigSpec
		override *quakekubeiov1alpha1.GameConfigSpec
		check    func(t *testing.T, result *quakekubeiov1alpha1.GameConfigSpec)
	}{
		{
			name: "override type",
			base: &quakekubeiov1alpha1.GameConfigSpec{
				Type: quakekubeiov1alpha1.FreeForAll,
			},
			override: &quakekubeiov1alpha1.GameConfigSpec{
				Type: quakekubeiov1alpha1.TeamDeathmatch,
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.GameConfigSpec) {
				if result.Type != quakekubeiov1alpha1.TeamDeathmatch {
					t.Errorf("expected TeamDeathmatch, got %s", result.Type)
				}
			},
		},
		{
			name: "override MOTD",
			base: &quakekubeiov1alpha1.GameConfigSpec{
				MOTD: "Old message",
			},
			override: &quakekubeiov1alpha1.GameConfigSpec{
				MOTD: "New message",
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.GameConfigSpec) {
				if result.MOTD != "New message" {
					t.Errorf("expected 'New message', got %s", result.MOTD)
				}
			},
		},
		{
			name: "forceRespawn override",
			base: &quakekubeiov1alpha1.GameConfigSpec{
				ForceRespawn: false,
			},
			override: &quakekubeiov1alpha1.GameConfigSpec{
				ForceRespawn: true,
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.GameConfigSpec) {
				if !result.ForceRespawn {
					t.Error("expected ForceRespawn true")
				}
			},
		},
		{
			name: "inactivity override",
			base: &quakekubeiov1alpha1.GameConfigSpec{
				Inactivity: &metav1.Duration{Duration: 5 * time.Minute},
			},
			override: &quakekubeiov1alpha1.GameConfigSpec{
				Inactivity: &metav1.Duration{Duration: 10 * time.Minute},
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.GameConfigSpec) {
				if result.Inactivity.Duration != 10*time.Minute {
					t.Errorf("expected 10m, got %v", result.Inactivity.Duration)
				}
			},
		},
		{
			name: "preserve base when override empty",
			base: &quakekubeiov1alpha1.GameConfigSpec{
				Type:       quakekubeiov1alpha1.FreeForAll,
				MOTD:       "Welcome",
				QuadFactor: intPtr(3),
			},
			override: &quakekubeiov1alpha1.GameConfigSpec{},
			check: func(t *testing.T, result *quakekubeiov1alpha1.GameConfigSpec) {
				if result.Type != quakekubeiov1alpha1.FreeForAll {
					t.Errorf("expected base Type preserved, got %s", result.Type)
				}
				if result.MOTD != "Welcome" {
					t.Errorf("expected base MOTD preserved, got %s", result.MOTD)
				}
				if result.QuadFactor == nil || *result.QuadFactor != 3 {
					t.Errorf("expected base QuadFactor preserved, got %v", result.QuadFactor)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of base to mutate
			result := &quakekubeiov1alpha1.GameConfigSpec{}
			if tt.base != nil {
				result.Type = tt.base.Type
				result.MOTD = tt.base.MOTD
				result.ForceRespawn = tt.base.ForceRespawn
				result.Inactivity = tt.base.Inactivity
				result.QuadFactor = tt.base.QuadFactor
				result.Password = tt.base.Password
				result.WeaponRespawn = tt.base.WeaponRespawn
			}
			mergeGameConfig(result, tt.override)
			tt.check(t, result)
		})
	}
}

func TestMergeServerSettings(t *testing.T) {
	tests := []struct {
		name     string
		base     *quakekubeiov1alpha1.ServerSettings
		override *quakekubeiov1alpha1.ServerSettings
		check    func(t *testing.T, result *quakekubeiov1alpha1.ServerSettings)
	}{
		{
			name: "override hostname",
			base: &quakekubeiov1alpha1.ServerSettings{
				Hostname: "Old Server",
			},
			override: &quakekubeiov1alpha1.ServerSettings{
				Hostname: "New Server",
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerSettings) {
				if result.Hostname != "New Server" {
					t.Errorf("expected 'New Server', got %s", result.Hostname)
				}
			},
		},
		{
			name: "override maxClients",
			base: &quakekubeiov1alpha1.ServerSettings{
				MaxClients: intPtr(12),
			},
			override: &quakekubeiov1alpha1.ServerSettings{
				MaxClients: intPtr(24),
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerSettings) {
				if *result.MaxClients != 24 {
					t.Errorf("expected 24, got %d", *result.MaxClients)
				}
			},
		},
		{
			name: "preserve base when override nil",
			base: &quakekubeiov1alpha1.ServerSettings{
				Hostname:   "My Server",
				MaxClients: intPtr(16),
			},
			override: &quakekubeiov1alpha1.ServerSettings{},
			check: func(t *testing.T, result *quakekubeiov1alpha1.ServerSettings) {
				if result.Hostname != "My Server" {
					t.Errorf("expected hostname preserved, got %s", result.Hostname)
				}
				if *result.MaxClients != 16 {
					t.Errorf("expected maxClients preserved, got %d", *result.MaxClients)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &quakekubeiov1alpha1.ServerSettings{}
			if tt.base != nil {
				result.Hostname = tt.base.Hostname
				result.MaxClients = tt.base.MaxClients
				result.RconPassword = tt.base.RconPassword
			}
			mergeServerSettings(result, tt.override)
			tt.check(t, result)
		})
	}
}

func TestMergeBotConfig(t *testing.T) {
	tests := []struct {
		name     string
		base     *quakekubeiov1alpha1.BotConfigSpec
		override *quakekubeiov1alpha1.BotConfigSpec
		check    func(t *testing.T, result *quakekubeiov1alpha1.BotConfigSpec)
	}{
		{
			name: "override minPlayers",
			base: &quakekubeiov1alpha1.BotConfigSpec{
				MinPlayers: intPtr(4),
			},
			override: &quakekubeiov1alpha1.BotConfigSpec{
				MinPlayers: intPtr(8),
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.BotConfigSpec) {
				if *result.MinPlayers != 8 {
					t.Errorf("expected 8, got %d", *result.MinPlayers)
				}
			},
		},
		{
			name: "override skill",
			base: &quakekubeiov1alpha1.BotConfigSpec{
				Skill: intPtr(2),
			},
			override: &quakekubeiov1alpha1.BotConfigSpec{
				Skill: intPtr(5),
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.BotConfigSpec) {
				if *result.Skill != 5 {
					t.Errorf("expected 5, got %d", *result.Skill)
				}
			},
		},
		{
			name: "noChat override",
			base: &quakekubeiov1alpha1.BotConfigSpec{
				NoChat: false,
			},
			override: &quakekubeiov1alpha1.BotConfigSpec{
				NoChat: true,
			},
			check: func(t *testing.T, result *quakekubeiov1alpha1.BotConfigSpec) {
				if !result.NoChat {
					t.Error("expected NoChat true")
				}
			},
		},
		{
			name: "preserve base when override empty",
			base: &quakekubeiov1alpha1.BotConfigSpec{
				MinPlayers: intPtr(4),
				Skill:      intPtr(3),
			},
			override: &quakekubeiov1alpha1.BotConfigSpec{},
			check: func(t *testing.T, result *quakekubeiov1alpha1.BotConfigSpec) {
				if *result.MinPlayers != 4 {
					t.Errorf("expected minPlayers preserved, got %d", *result.MinPlayers)
				}
				if *result.Skill != 3 {
					t.Errorf("expected skill preserved, got %d", *result.Skill)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &quakekubeiov1alpha1.BotConfigSpec{}
			if tt.base != nil {
				result.MinPlayers = tt.base.MinPlayers
				result.Skill = tt.base.Skill
				result.NoChat = tt.base.NoChat
			}
			mergeBotConfig(result, tt.override)
			tt.check(t, result)
		})
	}
}
