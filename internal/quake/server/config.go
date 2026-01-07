package server

import (
	"bytes"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config is the configuration for the quake server.
type Config struct {
	// FragLimit is the number of frags required to win.
	FragLimit int `json:"fragLimit"`
	// TimeLimit is the duration of time before the game ends.
	TimeLimit metav1.Duration `json:"timeLimit"`
	// GameConfig is the game configuration.
	GameConfig GameConfig `json:"game"`
	// NetworkConfig is the network configuration.
	NetworkConfig NetworkConfig `json:"network"`
	// BotConfig is the bot configuration.
	BotConfig BotConfig `json:"bot"`
	// FileServerConfig is the file/assets server configuration.
	FileServerConfig FileServerConfig `json:"fs"`
	// ServerConfig is the server configuration.
	ServerConfig ServerConfig `json:"server"`
	// Overrides are extra commands to run.
	Overrides []string `json:"commands"`
	// Maps is the list of maps to play.
	Maps []Map `json:"maps"`
}

// GameConfig is the game configuration.
type GameConfig struct {
	// ForceRespawn enables forced respawn.
	ForceRespawn bool `json:"forceRespawn"`
	// GameType is the type of game.
	GameType GameType `json:"type"`
	// Inactivity is the inactivity timeout in minutes.
	Inactivity metav1.Duration `json:"inactivity"`
	// Log is the log file.
	Log string `json:"log"`
	// MOTD is "the message of the day".
	MOTD string `json:"motd"`
	// JoinPassword is the server password for players joining.
	JoinPassword string `json:"password"`
	// QuadFactor is the multiplier for quad damage.
	QuadFactor int `json:"quadFactor"`
	// WeaponRespawn is the weapon respawn time.
	WeaponRespawn int `json:"weaponRespawn"`
}

// BotConfig is the bot configuration.
type BotConfig struct {
	// MinPlayers is the minimum number of players required to start the game (bots).
	MinPlayers int `json:"minPlayers"`
	// NoChat disables bot chat.
	NoChat bool `json:"noChat"`
	// Skill is the single player skill.
	Skill int `json:"skill"`
}

// FileServerConfig is the file server configuration.
type FileServerConfig struct {
	// BaseGame allows people to base mods upon mods syntax to follow
	BaseGame string `json:"baseGame"`
	// BasePath for files to be downloaded from this path may change for TC's and MOD's
	BasePath string `json:"basePath"`
	// CopyFiles toggle if files can be copied from servers or if client will download
	CopyFiles bool `json:"copyFiles"`
	// Debug enables file server debug mode for download/uploads or something
	Debug bool `json:"debug"`
	// Game set the game folder/dir default is baseq3
	Game string `json:"game"`
	// HomePath possibly for TC's and MODS the default is the path to quake3.exe
	HomePath string `json:"homePath"`
}

// NetworkConfig is the network configuration.
type NetworkConfig struct {
	// AllowDownload enables file downloads.
	AllowDownload bool `json:"allowDownload"`
	// DownloadURL is the download URL.
	DownloadURL string `json:"downloadURL"`
}

// ServerConfig is the server configuration.
type ServerConfig struct {
	// Hostname is the server hostname.
	Hostname string `json:"hostname"`
	// MaxClients is the maximum number of clients.
	MaxClients int `json:"maxClients"`
	// RconPassword is the server rcon password.
	RconPassword string `json:"password"`
}

// Marshal returns the configuration as a string.
func (c *Config) Marshal() ([]byte, error) {
	var b bytes.Buffer

	// GameConfig
	fmt.Fprintf(&b, "seta fraglimit %s\n", strconv.Quote(strconv.Itoa(c.FragLimit)))
	fmt.Fprintf(&b, "seta timelimit %s\n", strconv.Quote(strconv.Itoa(int(c.TimeLimit.Minutes()))))
	fmt.Fprintf(&b, "seta bot_minplayers %s\n", strconv.Quote(strconv.Itoa(c.BotConfig.MinPlayers)))
	fmt.Fprintf(&b, "seta bot_nochat %s\n", strconv.Quote(boolToString(c.BotConfig.NoChat)))
	fmt.Fprintf(&b, "seta g_forcerespawn %s\n", strconv.Quote(boolToString(c.GameConfig.ForceRespawn)))
	fmt.Fprintf(&b, "seta g_gametype %s\n", strconv.Quote(strconv.Itoa(int(c.GameConfig.GameType))))
	fmt.Fprintf(&b, "seta g_inactivity %s\n", strconv.Quote(strconv.Itoa(int(c.GameConfig.Inactivity.Seconds()))))
	fmt.Fprintf(&b, "seta g_log %s\n", strconv.Quote(c.GameConfig.Log))
	fmt.Fprintf(&b, "seta g_motd %s\n", strconv.Quote(c.GameConfig.MOTD))
	fmt.Fprintf(&b, "seta g_password %s\n", strconv.Quote(c.GameConfig.JoinPassword))
	fmt.Fprintf(&b, "seta g_quadfactor %s\n", strconv.Quote(strconv.Itoa(c.GameConfig.QuadFactor)))
	fmt.Fprintf(&b, "seta g_spSkill %s\n", strconv.Quote(strconv.Itoa(c.BotConfig.Skill)))
	fmt.Fprintf(&b, "seta g_weaponrespawn %s\n", strconv.Quote(strconv.Itoa(c.GameConfig.WeaponRespawn)))

	// NetworkConfig
	fmt.Fprintf(&b, "seta sv_allowDownload %s\n", strconv.Quote(boolToString(c.NetworkConfig.AllowDownload)))
	if c.NetworkConfig.DownloadURL != "" {
		fmt.Fprintf(&b, "sets sv_dlURL %s\n", c.NetworkConfig.DownloadURL)
	}

	// FileServerConfig
	fmt.Fprintf(&b, "seta fs_basegame %s\n", strconv.Quote(c.FileServerConfig.BaseGame))
	fmt.Fprintf(&b, "seta fs_basepath %s\n", strconv.Quote(c.FileServerConfig.BasePath))
	fmt.Fprintf(&b, "seta fs_copyfiles %s\n", strconv.Quote(boolToString(c.FileServerConfig.CopyFiles)))
	fmt.Fprintf(&b, "seta fs_debug %s\n", strconv.Quote(boolToString(c.FileServerConfig.Debug)))
	fmt.Fprintf(&b, "seta fs_game %s\n", strconv.Quote(c.FileServerConfig.Game))
	fmt.Fprintf(&b, "seta fs_homepath %s\n", strconv.Quote(c.FileServerConfig.HomePath))

	// ServerConfig
	fmt.Fprintf(&b, "seta sv_hostname %s\n", strconv.Quote(c.ServerConfig.Hostname))
	fmt.Fprintf(&b, "seta sv_maxclients %s\n", strconv.Quote(strconv.Itoa(c.ServerConfig.MaxClients)))
	fmt.Fprintf(&b, "seta rconpassword %s\n", strconv.Quote(c.ServerConfig.RconPassword))

	// Maps
	if len(c.Maps) > 0 {
		data, err := MarshalMaps(c.Maps)
		if err != nil {
			return nil, err
		}
		b.Write(data)
	}

	// Overrides
	for _, cmd := range c.Overrides {
		b.WriteString(cmd)
		b.WriteString("\n")
	}

	return b.Bytes(), nil
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
