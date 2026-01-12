package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GameType represents the Quake 3 game mode
// +kubebuilder:validation:Enum=FreeForAll;Tournament;SinglePlayer;TeamDeathmatch;CaptureTheFlag
type GameType string

const (
	FreeForAll     GameType = "FreeForAll"
	Tournament     GameType = "Tournament"
	SinglePlayer   GameType = "SinglePlayer"
	TeamDeathmatch GameType = "TeamDeathmatch"
	CaptureTheFlag GameType = "CaptureTheFlag"
)

// QuakeServerSpec defines the desired state of QuakeServer
type QuakeServerSpec struct {
	// AgreeEula must be set to true to accept the Quake 3 EULA.
	// +kubebuilder:validation:Required
	AgreeEula bool `json:"agreeEula"`

	// TemplateRef is an optional reference to a QuakeServerTemplate for base configuration.
	// +optional
	TemplateRef *TemplateReference `json:"templateRef,omitempty"`

	// ServerConfig contains the server configuration. Values here override template values.
	// +optional
	ServerConfig *ServerConfigSpec `json:"serverConfig,omitempty"`

	// Gateway configures the Gateway API routes for this server.
	// +optional
	Gateway *GatewaySpec `json:"gateway,omitempty"`

	// Resources specifies the compute resources for the server container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// PodSecurityContext specifies the security context for the server pod.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// SecurityContext specifies the security context for the server container.
	// If not specified, secure defaults are applied (runAsNonRoot, drop all capabilities, read-only root filesystem).
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// TemplateReference references a QuakeServerTemplate
type TemplateReference struct {
	// Name is the name of the QuakeServerTemplate.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the QuakeServerTemplate.
	// If not specified, the QuakeServer's namespace is used.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ServerConfigSpec contains all server configuration options
type ServerConfigSpec struct {
	// FragLimit is the number of frags required to win.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=25
	// +optional
	FragLimit *int `json:"fragLimit,omitempty"`

	// TimeLimit is the duration of time before the game ends (e.g., "15m").
	// +optional
	TimeLimit *metav1.Duration `json:"timeLimit,omitempty"`

	// Game contains game-specific configuration.
	// +optional
	Game *GameConfigSpec `json:"game,omitempty"`

	// Server contains server-specific configuration.
	// +optional
	Server *ServerSettings `json:"server,omitempty"`

	// Bot contains bot configuration.
	// +optional
	Bot *BotConfigSpec `json:"bot,omitempty"`

	// Maps is the list of maps to play.
	// +optional
	Maps []MapSpec `json:"maps,omitempty"`

	// Commands are extra commands to run on the server.
	// +optional
	Commands []string `json:"commands,omitempty"`
}

// GameConfigSpec contains game configuration
type GameConfigSpec struct {
	// Type is the game type (FreeForAll, Tournament, SinglePlayer, TeamDeathmatch, CaptureTheFlag).
	// +kubebuilder:default=FreeForAll
	// +optional
	Type GameType `json:"type,omitempty"`

	// MOTD is the message of the day shown to players.
	// +optional
	MOTD string `json:"motd,omitempty"`

	// ForceRespawn enables forced respawn.
	// +optional
	ForceRespawn bool `json:"forceRespawn,omitempty"`

	// Inactivity is the inactivity timeout (e.g., "10m").
	// +optional
	Inactivity *metav1.Duration `json:"inactivity,omitempty"`

	// QuadFactor is the damage multiplier for quad damage.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	// +optional
	QuadFactor *int `json:"quadFactor,omitempty"`

	// Password is the server password for players joining.
	// +optional
	Password string `json:"password,omitempty"`

	// WeaponRespawn is the weapon respawn time in seconds.
	// +optional
	WeaponRespawn *int `json:"weaponRespawn,omitempty"`
}

// ServerSettings contains server-specific settings
type ServerSettings struct {
	// Hostname is the server hostname displayed to players.
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// MaxClients is the maximum number of clients.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	// +kubebuilder:default=12
	// +optional
	MaxClients *int `json:"maxClients,omitempty"`

	// RconPassword is the RCON password for remote administration.
	// +optional
	RconPassword string `json:"rconPassword,omitempty"`
}

// BotConfigSpec contains bot configuration
type BotConfigSpec struct {
	// MinPlayers is the minimum number of players (bots fill remaining slots).
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinPlayers *int `json:"minPlayers,omitempty"`

	// Skill is the bot skill level (1-5).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=3
	// +optional
	Skill *int `json:"skill,omitempty"`

	// NoChat disables bot chat.
	// +optional
	NoChat bool `json:"noChat,omitempty"`
}

// MapSpec defines a map in the rotation
type MapSpec struct {
	// Name is the map name (e.g., "q3dm7").
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type is the game type for this specific map.
	// +optional
	Type GameType `json:"type,omitempty"`

	// CaptureLimit is the capture limit for CTF maps.
	// +optional
	CaptureLimit *int `json:"captureLimit,omitempty"`

	// FragLimit overrides the global frag limit for this map.
	// +optional
	FragLimit *int `json:"fragLimit,omitempty"`

	// TimeLimit overrides the global time limit for this map.
	// +optional
	TimeLimit *metav1.Duration `json:"timeLimit,omitempty"`
}

// GatewaySpec configures Gateway API integration
type GatewaySpec struct {
	// Enabled enables Gateway API route creation.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// GatewayRef references the Gateway to attach routes to.
	// +optional
	GatewayRef *GatewayReference `json:"gatewayRef,omitempty"`

	// Hostname is the hostname for HTTPRoute (browser WebSocket access).
	// +optional
	Hostname string `json:"hostname,omitempty"`
}

// GatewayReference references a Gateway resource
type GatewayReference struct {
	// Name is the name of the Gateway.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the Gateway.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// QuakeServerStatus defines the observed state of QuakeServer
type QuakeServerStatus struct {
	// Ready indicates if the server is ready to accept connections.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Endpoint is the browser WebSocket endpoint URL.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// GamePort is the UDP port for native Quake clients.
	// +optional
	GamePort int32 `json:"gamePort,omitempty"`

	// Players is the current number of connected players.
	// +optional
	Players int `json:"players,omitempty"`

	// TemplateRef shows the resolved template reference.
	// +optional
	TemplateRef string `json:"templateRef,omitempty"`

	// Conditions represent the latest available observations of the QuakeServer's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Server ready state"
//+kubebuilder:printcolumn:name="Players",type="integer",JSONPath=".status.players",description="Current player count"
//+kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".status.endpoint",description="Browser endpoint"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// QuakeServer is the Schema for the quakeservers API
type QuakeServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuakeServerSpec   `json:"spec,omitempty"`
	Status QuakeServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QuakeServerList contains a list of QuakeServer
type QuakeServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuakeServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuakeServer{}, &QuakeServerList{})
}
