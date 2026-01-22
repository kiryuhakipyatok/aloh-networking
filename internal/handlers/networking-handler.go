package handlers

import (
	"bufio"
	"context"
	"fmt"
	"networking/internal/domain/services"
	"os"
	"strings"
	"time"
)

type NetworkingHandler interface {
	Connect(receiversIds []string)
	SendMessage(msg string)
	Start()
}

type networkingHandler struct {
	NetworkingServ services.NetworkingServ
}

func NewNetworkingHandler(ns services.NetworkingServ) NetworkingHandler {
	nh := &networkingHandler{
		NetworkingServ: ns,
	}
	go nh.Start()
	return nh
}

func (nh *networkingHandler) Start() {
	reader := bufio.NewReader(os.Stdin)
	for {
		m, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err.Error())
		}
		m = strings.TrimSpace(m)
		switch m {
		case "connect":
			m, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println(err.Error())
			}
			m = strings.TrimSpace(m)
			strs := strings.Split(m, " ")
			nh.Connect(strs)
		case "sendMsg":
			m, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println(err.Error())
			}
			nh.SendMessage(m)
		case "sendDatagram":
			m, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println(err.Error())
			}
			nh.SendDatagram(m)
		case "disconnect":
			disconCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			if err := nh.NetworkingServ.Disconnect(disconCtx); err != nil {
				fmt.Println(err.Error())
			}
			cancel()
		default:
			fmt.Println("sosi")
		}
	}
}

func (nh *networkingHandler) Connect(receiversIds []string) {
	if err := nh.NetworkingServ.Сonnect(receiversIds); err != nil {
		fmt.Println(err.Error())
	}
}

func (nh *networkingHandler) SendMessage(msg string) {
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	// defer cancel()
	if err := nh.NetworkingServ.SendMessage(context.Background(), msg); err != nil {
		fmt.Println(err.Error())
	}
}

func (nh *networkingHandler) SendDatagram(msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := nh.NetworkingServ.SendDatagram(ctx, []byte(msg)); err != nil {
		fmt.Println(err.Error())
	}
}
