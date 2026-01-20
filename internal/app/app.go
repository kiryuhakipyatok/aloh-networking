package app

import (
	"context"
	"flag"
	"fmt"
	"networking/internal/client"
	"networking/internal/domain/repository"
	"networking/internal/domain/services"
	"networking/internal/handlers"
	"networking/internal/protocol"
	"os"
	"os/signal"
	"syscall"
)

func Run() {
	// path := os.Getenv("CONFIG_PATH")
	// cfg := config.NewConfig(path)
	id := flag.String("id", "123", "user id")
	flag.Parse()
	fmt.Println("start")
	sessionRepo := repository.NewSessionRepository()
	sendSDP := make(chan protocol.Message, 100)
	receiveSDP := make(chan protocol.ReplyMessage, 100)
	signalingClient := client.NewSignalingClient(sessionRepo, "164.90.163.153:1234", *id, sendSDP, receiveSDP)
	networkingServie := services.NewNetworkingServ(signalingClient, sessionRepo, receiveSDP)
	networkingHandler := handlers.NewNetworkingHandler(networkingServie)
	go networkingHandler.Start()
	ctx := context.Background()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	signalingClient.Close(ctx, 0, "close")
	fmt.Println("close")
	close(quit)
}
