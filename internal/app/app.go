package app

import (
	"context"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/kiryuhakipyatok/aloh-networking/config"
	"github.com/kiryuhakipyatok/aloh-networking/internal/client"

	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/repository"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/services/networking"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/logger"
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

func Init(userID string, friends []string, cfg config.Config) (networking.NetworkingServ, context.CancelFunc, error) {
	if err := loadEnv(); err != nil {
		panic(err)
	}
	//cfg := config.NewConfig()
	log := logger.NewLogger(cfg.App, cfg.App.LogPath)
	log.Info("initializing library...")

	sessionRepo := repository.NewSessionRepository()

	sendSDP := make(chan client.Message, cfg.App.SendSDPSize)
	receiveSDP := make(chan client.ReplyMessage, cfg.App.ReceiveSDPSize)

	ctx, cancel := context.WithCancel(context.Background())

	signalingClient, err := client.NewSignalingClient(ctx, log, userID, sendSDP, receiveSDP, cfg.Signaling)
	if err != nil {
		log.Error("failed to create signaling client", logger.Err(err))
		cancel()
		return nil, nil, err
	}

	networkingSetup := networking.NewNetworkingSetup{
		Id:          userID,
		L:           log,
		SC:          signalingClient,
		Cfg:         cfg.Networking,
		SR:          sessionRepo,
		ReceiveSDPs: receiveSDP,
	}

	networkingService := networking.NewNetworkingServ(ctx, networkingSetup)

	log.Info("library initialized for user: " + userID)

	return networkingService, func() {
		log.Info("stopping library...")
		cancel()
		if err := networkingService.Disconnect(); err != nil {
			log.Error("failed to disconnect sessoions", logger.Err(err))
		}
		if err := signalingClient.CloseConnection(0, "close"); err != nil {
			log.Error("failed to close signaling client", logger.Err(err))
		}
		close(sendSDP)
		close(receiveSDP)
		log.Info("library stopped")
	}, nil
}

// func Run() {
// 	id := flag.String("id", "123", "user id")
// 	flag.Parse()

// 	networkingServ, close, handlerCfg, err := Init(*id, "")
// 	if err != nil {
// 		panic(fmt.Errorf("failed to init networking service: %w", err))
// 	}

// 	networkingHandler := handlers.NewNetworkingHandler(networkingServ, handlerCfg)

// 	go networkingHandler.Start()

// 	networkingHandler.OnChat(func(id string, data []byte) {
// 		fmt.Println(id, ":", string(data))
// 	})
// 	networkingHandler.OnVideo(func(id string, data []byte) {
// 		fmt.Println(id, ":", string(data))
// 	})
// 	networkingHandler.OnVoice(func(id string, data []byte) {
// 		fmt.Println(id, ":", string(data))
// 	})

// 	quit := make(chan os.Signal, 1)
// 	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
// 	<-quit

// 	close()
// }
