package server

import (
	"context"
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	activePlayers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "quake_active_players",
		Help: "The current number of active players",
	})

	scores = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "quake_player_scores",
		Help: "Current scores by player, by map",
	}, []string{"player", "map"})

	pings = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "quake_player_pings",
		Help: "Current ping by player",
	}, []string{"player"})

	configReloads = promauto.NewCounter(prometheus.CounterOpts{
		Name: "quake_config_reloads",
		Help: "Config file reload count",
	})
)

// monitorMetrics monitors the server metrics.
func (s *Server) monitorMetrics(ctx context.Context, host, port string) {
	addr := s.Addr
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		addr = net.JoinHostPort("127.0.0.1", port)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status, err := s.netClient.GetStatus(addr)
			if err != nil {
				s.Logger.Error("metrics: get status failed", zap.Error(err))
				continue
			}
			activePlayers.Set(float64(len(status.Players)))
			for _, p := range status.Players {
				if mapname, ok := status.Configuration["mapname"]; ok {
					scores.WithLabelValues(p.Name, mapname).Set(float64(p.Score))
				}
				pings.WithLabelValues(p.Name).Set(float64(p.Ping))
			}
		case <-ctx.Done():
			return
		}
	}
}
