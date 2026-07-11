// package app

// import (
// 	"context"
// 	"flag"
// 	"fmt"
// 	"os"
// 	"path/filepath"

// 	"github.com/google/uuid"

// 	"github.com/kiryuhakipyatok/aloh-networking/config"
// 	"github.com/kiryuhakipyatok/aloh-networking/internal/client"

// 	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/repository"
// 	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/services/networking"
// 	"github.com/kiryuhakipyatok/aloh-networking/pkg/logger"

// 	"github.com/joho/godotenv"
// )

// func loadEnv() error {
// 	path, err := os.Executable()
// 	if err == nil {
// 		dir := filepath.Dir(path)
// 		envPath := filepath.Join(dir, ".env")
// 		if err := godotenv.Load(envPath); err == nil {
// 			return nil
// 		}
// 	}

// 	if err := godotenv.Load(".env"); err == nil {
// 		return nil
// 	}
// 	return err
// }

// func Init(userID uuid.UUID, cfg config.Config) (networking.NetworkingServ, context.CancelFunc, error) {
// 	if err := loadEnv(); err != nil {
// 		panic(err)
// 	}
// 	//cfg := config.NewConfig()
// 	log := logger.NewLogger(cfg.App, cfg.App.LogPath)
// 	log.Info("initializing library...")

// 	sessionRepo := repository.NewSessionRepository()

// 	sendSDP := make(chan client.Message, cfg.App.SendSDPSize)
// 	receiveSDP := make(chan client.ReplyMessage, cfg.App.ReceiveSDPSize)

// 	ctx, cancel := context.WithCancel(context.Background())

// 	signalingClient, err := client.NewSignalingClient(ctx, log, userID, sendSDP, receiveSDP, cfg.Signaling)
// 	if err != nil {
// 		log.Error("failed to create signaling client", logger.Err(err))
// 		cancel()
// 		return nil, nil, err
// 	}

// 	networkingSetup := networking.NewNetworkingSetup{
// 		Id:          userID,
// 		L:           log,
// 		SC:          signalingClient,
// 		Cfg:         cfg.Networking,
// 		SR:          sessionRepo,
// 		ReceiveSDPs: receiveSDP,
// 	}

// 	networkingService := networking.NewNetworkingServ(ctx, networkingSetup)

// 	log.Info("library initialized for user: " + userID.String())

// 	return networkingService, func() {
// 		log.Info("stopping library...")
// 		cancel()
// 		if err := networkingService.Disconnect(); err != nil {
// 			log.Error("failed to disconnect sessoions", logger.Err(err))
// 		}
// 		if err := signalingClient.CloseConnection(0, "close"); err != nil {
// 			log.Error("failed to close signaling client", logger.Err(err))
// 		}
// 		close(sendSDP)
// 		close(receiveSDP)
// 		log.Info("library stopped")
// 	}, nil
// }

// func Run() {
// 	id := flag.String("id", "123", "user id")
// 	flag.Parse()

// 	// 	networkingServ, close, handlerCfg, err := Init(*id, "")
// 	// 	if err != nil {
// 	// 		panic(fmt.Errorf("failed to init networking service: %w", err))
// 	// 	}

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

// 	// 	quit := make(chan os.Signal, 1)
// 	// 	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
// 	// 	<-quit

//		//		close()
//		//	}
//	}
package app

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/google/uuid"
	"github.com/kiryuhakipyatok/aloh-networking/config"
	"github.com/kiryuhakipyatok/aloh-networking/internal/client"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/repository"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/services/networking"
	"github.com/kiryuhakipyatok/aloh-networking/internal/handlers"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/logger"

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
	return err
}

func Init(userID uuid.UUID, logPath string) (networking.NetworkingServ, context.CancelFunc, config.Handler, error) {
	if err := loadEnv(); err != nil {
		panic(err)
	}
	cfg := config.NewConfig()
	log := logger.NewLogger(cfg.App, logPath)
	log.Info("initializing library...")

	sessionRepo := repository.NewSessionRepository()

	sendSDP := make(chan client.Message, cfg.App.SendSDPSize)
	receiveSDP := make(chan client.ReplyMessage, cfg.App.ReceiveSDPSize)

	ctx, cancel := context.WithCancel(context.Background())

	signalingClient, err := client.NewSignalingClient(ctx, log, userID, sendSDP, receiveSDP, cfg.Signaling)
	if err != nil {
		log.Error("failed to create signaling client", logger.Err(err))
		cancel()
		return nil, nil, config.Handler{}, err
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

	log.Info("library initialized for user: " + userID.String())

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
	}, cfg.Handler, nil
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
	id := flag.String("id", "00000000-0000-0000-0000-000000000000", "user id")
	flag.Parse()

	idUUID, _ := uuid.Parse(*id)

	networkingServ, close, handlerCfg, err := Init(idUUID, "")
	if err != nil {
		panic(fmt.Errorf("failed to init networking service: %w", err))
	}

	networkingHandler := handlers.NewNetworkingHandler(networkingServ, handlerCfg)

	go networkingHandler.Start()

	ctx := context.Background()

	clientCfg := &genai.ClientConfig{
		APIKey: "AIzaSyD8U3EX4C4ijWsjNfILBeea_PGr88O3C2k",
	}
	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		panic(err)
	}

	networkingHandler.OnChat(func(id uuid.UUID, data []byte) {
		var res string
		result, err := client.Models.GenerateContent(
			ctx,
			"gemini-2.5-flash",
			genai.Text(string(data)),
			nil,
		)
		if err != nil {
			res = err.Error()
			fmt.Println(err)
		} else {
			res = result.Text()
		}
		if err := networkingHandler.SendMessage([]byte(res)); err != nil {
			fmt.Println(err)
		}

	})
	// networkingHandler.OnVideo(func(id string, data []byte) {
	// 	if err := networkingHandler.SendVideo(data); err != nil {
	// 		fmt.Println(err)
	// 	}
	// })
	networkingHandler.OnVoice(func(id uuid.UUID, data []byte) {
		if err := networkingHandler.SendVoice(data); err != nil {
			fmt.Println(err)
		}
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	close()
}
