package app

import (
	"context"
	"flag"
	"fmt"
	"networking/internal/client"
	"networking/internal/config"
	"networking/internal/domain/repository"
	"networking/internal/domain/services"
	"networking/internal/handlers"
	"networking/internal/protocol"
	"networking/pkg/logger"
	"os"
	"os/signal"
	"syscall"
)

func Run() {
	path := os.Getenv("CONFIG_PATH")
	cfg := config.NewConfig(path)
	fmt.Println(cfg)
	id := flag.String("id", "123", "user id")
	flag.Parse()
	log := logger.NewLogger(cfg.App)
	log.Info("app starting...")
	log.Info("config loaded")
	sessionRepo := repository.NewSessionRepository()
	log.Info("session repo created")
	sendSDP := make(chan protocol.Message, cfg.App.SendSDPSize)
	receiveSDP := make(chan protocol.ReplyMessage, cfg.App.ReceiveSDPSize)
	closeCtx, cancel := context.WithCancel(context.Background())
	signalingClient := client.NewSignalingClient(closeCtx, log, *id, sendSDP, receiveSDP, cfg.Signaling)
	log.Info("signaling client created")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	networkingService := services.NewNetworkingServ(closeCtx, signalingClient, cfg.Networking, log, sessionRepo, receiveSDP)
	go func() {
		if err := networkingService.ReceiveConnects(); err != nil {
			log.Error("failed to receive connects", logger.Err(err))
			quit <- syscall.SIGINT
		}
	}()
	log.Info("networking client created")
	_ = handlers.NewNetworkingHandler(networkingService)
	log.Info("networking handler created")
	log.Info("app started")
	<-quit
	log.Info("app closing...")
	cancel()
	networkingService.Disconnect(context.Background())
	signalingClient.Close(0, "close")
	close(quit)
	log.Info("app closed")
}
