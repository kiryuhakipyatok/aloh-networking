package app

import (
	"context"
	"flag"
	"fmt"
	"networking/internal/client"
	"networking/internal/config"
	"networking/internal/domain/repository"
	"networking/internal/domain/services/networking"
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
	networkingService := networking.NewNetworkingServ(closeCtx, *id, signalingClient, cfg.Networking, log, sessionRepo, receiveSDP)
	log.Info("networking client created")
	networkingHandler := handlers.NewNetworkingHandler(networkingService)
	networkingHandler.OnChat(func(data []byte) {
		fmt.Println(string(data))
	})
	networkingHandler.OnVideo(func(data []byte) {
		fmt.Println(string(data))
	})
	networkingHandler.OnVoice(func(data []byte) {
		fmt.Println(string(data))
	})
	log.Info("networking handler created")
	log.Info("app started")
	<-quit
	log.Info("app closing...")
	cancel()
	networkingService.Disconnect()
	signalingClient.Close(0, "close")
	close(quit)
	log.Info("app closed")
}
