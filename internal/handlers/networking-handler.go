package handlers

import (
	"bufio"
	"context"
	"fmt"
	"networking/internal/domain/services"
	"networking/internal/utils"
	"os"
	"strings"
	"time"
)

const (
	CHAT = iota
	VOICE
	VIDEO
)

type NetworkingHandler struct {
	NetworkingServ services.NetworkingServ
}

func NewNetworkingHandler(ns services.NetworkingServ) *NetworkingHandler {
	nh := &NetworkingHandler{
		NetworkingServ: ns,
	}
	go nh.Start()
	return nh
}

func (nh *NetworkingHandler) Start() {
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
		case "sendVoice":
			m, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println(err.Error())
			}
			nh.SendVoice([]byte(m))
		case "sendVideo":
			m, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println(err.Error())
			}
			nh.SendVideo([]byte(m))
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

func (nh *NetworkingHandler) Connect(receiversIds []string) {
	ridsLen := len(receiversIds)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(ridsLen))
	defer cancel()
	if err := nh.NetworkingServ.Сonnect(ctx, receiversIds); err != nil {
		fmt.Println(err.Error())
	}
}

func (nh *NetworkingHandler) SendMessage(msg string) {
	data := utils.SetFirstByte(CHAT, []byte(msg))
	if err := nh.NetworkingServ.SendInStream(context.Background(), data); err != nil {
		fmt.Println(err.Error())
	}
}

func (nh *NetworkingHandler) SendVoice(data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	data = utils.SetFirstByte(VOICE, data)
	if err := nh.NetworkingServ.SendDatagram(ctx, data); err != nil {
		fmt.Println(err.Error())
	}
}

func (nh *NetworkingHandler) SendVideo(data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	data = utils.SetFirstByte(VIDEO, data)
	if err := nh.NetworkingServ.SendDatagram(ctx, data); err != nil {
		fmt.Println(err.Error())
	}
}

func (nh *NetworkingHandler) OnChat(f func(data []byte)) {
	nh.NetworkingServ.SaveChatHandler(f)
}

func (nh *NetworkingHandler) OnVideo(f func(data []byte)) {
	nh.NetworkingServ.SaveVideoHandler(f)
}

func (nh *NetworkingHandler) OnVoice(f func(data []byte)) {
	nh.NetworkingServ.SaveVoiceHandler(f)
}
