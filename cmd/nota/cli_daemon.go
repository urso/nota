package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urso/nota/pkg/rpc"
	"github.com/urso/nota/pkg/service"
)

type DaemonCmd struct{}

func (c *DaemonCmd) Run() error {
	svc, err := service.New("")
	if err != nil {
		return fmt.Errorf("initializing service: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	svc.StartPoller(ctx, service.DefaultPollInterval)
	defer svc.StopPoller()

	server := rpc.NewServer(os.Stdin, os.Stdout)

	handlers := rpc.NewThreadHandlers(svc)
	handlers.Register(server)

	go rpc.SubscribeAndNotify(ctx, svc, server)

	return server.Serve(ctx)
}
