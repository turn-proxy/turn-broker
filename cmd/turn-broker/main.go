package main

import (
	"cmp"
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/turn-proxy/turn-broker/internal/broker"
	"github.com/turn-proxy/turn-broker/internal/vkprovider"
)

func main() {
	cfgPath := flag.String("config", cmp.Or(os.Getenv("TURN_BROKER_CONFIG"), "turn-broker.json"), "config path")
	flag.Parse()

	level := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	cfg, err := broker.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("loading config", "path", *cfgPath, "err", err)
		os.Exit(1)
	}
	prov, err := vkprovider.New(cfg.VK.CookiesFile, cfg.VK.AppID, cfg.VK.APIVersion)
	if err != nil {
		slog.Error("building vk provider", "err", err)
		os.Exit(1)
	}
	var refresh time.Duration
	if cfg.VK.SessionRefreshSec != nil {
		refresh = time.Duration(*cfg.VK.SessionRefreshSec) * time.Second
	}

	slog.Info("starting turn-broker", "bind", cfg.Bind)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cache := broker.NewSessionCache()
	if err := broker.Serve(ctx, cfg.Bind, cache, prov, refresh); err != nil {
		slog.Error("broker exited", "err", err)
		os.Exit(1)
	}
}
