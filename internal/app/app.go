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
	"networking/pkg/logger"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
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

func Init(configPath, configName, userID string) (networking.NetworkingServ, context.CancelFunc, config.Handler) {
	if err := loadEnv(); err != nil {
		panic(err)
	}
	cfg := config.NewConfig(configPath, configName)
	log := logger.NewLogger(cfg.App)
	log.Info("initializing library...")

	sessionRepo := repository.NewSessionRepository()

	sendSDP := make(chan client.Message, cfg.App.SendSDPSize)
	receiveSDP := make(chan client.ReplyMessage, cfg.App.ReceiveSDPSize)

	ctx, cancel := context.WithCancel(context.Background())

	signalingClient := client.NewSignalingClient(ctx, log, userID, sendSDP, receiveSDP, cfg.Signaling)
	networkingService := networking.NewNetworkingServ(ctx, userID, signalingClient, cfg.Networking, log, sessionRepo, receiveSDP)

	log.Info("library initialized for user: " + userID)

	return networkingService, func() {
		log.Info("stopping library...")
		cancel()
		networkingService.Disconnect()
		if err := signalingClient.Close(0, "close"); err != nil {
			log.Error("failed to close signaling client", logger.Err(err))
		}
		close(sendSDP)
		close(receiveSDP)
		log.Info("library stopped")
	}, cfg.Handler
}

func Run() {
	// if err := godotenv.Load("../../.env"); err != nil {
	// 	panic(err)
	// }
	// // if err := godotenv.Load(".env"); err != nil {
	// // 	panic(err)
	// // }
	if err := loadEnv(); err != nil {
		panic(err)
	}
	id := flag.String("id", "123", "user id")
	flag.Parse()

	configPath := os.Getenv("CONFIG_PATH")
	configName := os.Getenv("CONFIG_NAME")

	networkingServ, close, handlerCfg := Init(configPath, configName, *id)

	networkingHandler := handlers.NewNetworkingHandler(networkingServ, handlerCfg)

	go networkingHandler.Start()

	ctx := context.Background()
	clientCfg := &genai.ClientConfig{
		APIKey: "AIzaSyDhPDckohaUJZj727j-Ttp_CYOlRE60pxU",
	}
	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		panic(err)
	}

	networkingHandler.OnChat(func(data []byte) {
		var res string
		result, err := client.Models.GenerateContent(
			ctx,
			"gemini-2.0-flash",
			genai.Text(string(data)),
			nil,
		)
		if err != nil {
			res = err.Error()
			fmt.Println(err)
		}else{
			res = result.Text()
		}
		if err := networkingHandler.SendMessage(res); err != nil {
			fmt.Println(err)
		}

	})
	networkingHandler.OnVideo(func(data []byte) {
		if err := networkingHandler.SendVideo(data); err != nil {
			fmt.Println(err)
		}
	})
	networkingHandler.OnVoice(func(data []byte) {
		if err := networkingHandler.SendVoice(data); err != nil {
			fmt.Println(err)
		}
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	close()
}
