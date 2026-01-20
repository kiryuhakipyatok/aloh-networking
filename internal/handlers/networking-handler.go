package handlers

import (
	"bufio"
	"fmt"
	"networking/internal/domain/services"
	"os"
	"strings"
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
	return &networkingHandler{
		NetworkingServ: ns,
	}
}

func (nh *networkingHandler) Start() {
	reader := bufio.NewReader(os.Stdin)
	for {
		m, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		m = strings.TrimSpace(m)
		// strs := strings.Split(m, " ")
		switch m {
		case "connect":
			m, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			m = strings.TrimSpace(m)
			strs := strings.Split(m, " ")
			nh.Connect(strs)
		case "sendMsg":
			m, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			nh.SendMessage(m)
		case "sendDatagram":
			m, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			nh.SendMessage(m)
		default:
			fmt.Println("sosi")
		}
	}
}

func (nh *networkingHandler) Connect(receiversIds []string) {
	if err := nh.NetworkingServ.Сonnect(receiversIds); err != nil {
		panic(err)
	}
}

func (nh *networkingHandler) SendMessage(msg string) {
	if err := nh.NetworkingServ.SendMessage(msg); err != nil {
		panic(err)
	}
}

func (nh *networkingHandler) SendDatagram(msg string) {
	if err := nh.NetworkingServ.SendDatagram([]byte(msg)); err != nil {
		panic(err)
	}
}
