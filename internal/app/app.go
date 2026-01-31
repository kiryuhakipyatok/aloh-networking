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
	"path/filepath"
	"syscall"

	"github.com/joho/godotenv"
)

func loadEnv() error {
	path, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(path)
		envPath := filepath.Join(dir, ".env")
		if err := godotenv.Load(envPath); err == nil {
			return nil
		}
	}

	if err := godotenv.Load(".env"); err == nil {
		return nil
	}
	return nil
}

func Init(configPath string, userID string) (networking.NetworkingServ, context.CancelFunc) {
	if err := loadEnv(); err != nil {
		panic(err)
	}
	cfg := config.NewConfig(configPath)
	log := logger.NewLogger(cfg.App)
	log.Info("initializing library...")

	sessionRepo := repository.NewSessionRepository()

	sendSDP := make(chan protocol.Message, cfg.App.SendSDPSize)
	receiveSDP := make(chan protocol.ReplyMessage, cfg.App.ReceiveSDPSize)

	ctx, cancel := context.WithCancel(context.Background())

	signalingClient := client.NewSignalingClient(ctx, log, userID, sendSDP, receiveSDP, cfg.Signaling)
	networkingService := networking.NewNetworkingServ(ctx, userID, signalingClient, cfg.Networking, log, sessionRepo, receiveSDP)

	log.Info("library initialized for user: " + userID)

	return networkingService, func() {
		log.Info("stopping library...")
		cancel()
		networkingService.Disconnect()
		signalingClient.Close(0, "close")
		log.Info("library stopped")
	}
}

func Run() {
	// if err := godotenv.Load("../../.env"); err != nil {
	// 	panic(err)
	// }
	// // if err := godotenv.Load(".env"); err != nil {
	// // 	panic(err)
	// // }
	id := flag.String("id", "123", "user id")
	flag.Parse()

	path := os.Getenv("CONFIG_PATH")

	networkingServ, close := Init(path, *id)

	networkingHandler := handlers.NewNetworkingHandler(networkingServ)

	go networkingHandler.Start()

	networkingHandler.OnChat(func(data []byte) {
		fmt.Println(string(data))
	})
	networkingHandler.OnVideo(func(data []byte) {
		fmt.Println(string(data))
	})
	networkingHandler.OnVoice(func(data []byte) {
		fmt.Println(string(data))
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	close()
}
