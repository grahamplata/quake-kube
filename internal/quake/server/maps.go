package server

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Map configuration.
type Map struct {
	Name string   `json:"name"`
	Type GameType `json:"type"`

	CaptureLimit int             `json:"captureLimit"`
	FragLimit    int             `json:"fragLimit"`
	TimeLimit    metav1.Duration `json:"timeLimit"`
}

func (m Map) command() string {
	cmds := []string{fmt.Sprintf("g_gametype %d", m.Type)}
	if m.Type == CaptureTheFlag && m.CaptureLimit != 0 {
		cmds = append(cmds, fmt.Sprintf("capturelimit %d", m.CaptureLimit))
	}
	if m.FragLimit != 0 {
		cmds = append(cmds, fmt.Sprintf("fraglimit %d", m.FragLimit))
	}
	if m.TimeLimit.Duration != 0 {
		cmds = append(cmds, fmt.Sprintf("timelimit %d", int(m.TimeLimit.Minutes())))
	}
	cmds = append(cmds, fmt.Sprintf("map %s", m.Name))
	return strings.Join(cmds, " ; ")
}

// MarshalMaps returns the maps as a string.
func MarshalMaps(maps []Map) ([]byte, error) {
	var b strings.Builder
	for i, m := range maps {
		nextmap := fmt.Sprintf("d%d", (i+1)%len(maps))
		fmt.Fprintf(&b, "set d%d \"seta %s ; set nextmap vstr %s\"\n", i, m.command(), nextmap)
	}
	if len(maps) > 0 {
		b.WriteString("vstr d0\n")
	}
	return []byte(b.String()), nil
}
