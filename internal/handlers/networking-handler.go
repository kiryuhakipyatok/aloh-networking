package handlers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"networking/internal/domain/services/networking"
	"networking/internal/utils"
	"os"
	"strings"
	"time"
)

type NetworkingHandler struct {
	NetworkingServ networking.NetworkingServ
}

func NewNetworkingHandler(ns networking.NetworkingServ) *NetworkingHandler {
	nh := &NetworkingHandler{
		NetworkingServ: ns,
	}
	return nh
}

func (nh *NetworkingHandler) Start() {
	reader := bufio.NewReader(os.Stdin)
	for {
		m, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Println(err.Error())
			break
		}
		m = strings.TrimSpace(m)
		switch m {
		case "connect":
			m, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Println(err.Error())
				break
			}
			m = strings.TrimSpace(m)
			strs := strings.Split(m, " ")
			if err := nh.Connect(strs); err != nil {
				fmt.Println(err)
			}
		case "sendMsg":
			m, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Println(err.Error())
				break
			}
			if err := nh.SendMessage(m); err != nil {
				fmt.Println(err)
			}
		case "sendVoice":
			m, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Println(err.Error())
				break
			}
			if err := nh.SendVoice([]byte(m)); err != nil {
				fmt.Println(err)
			}
		case "sendVideo":
			m, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Println(err.Error())
				break
			}
			if err := nh.SendVideo([]byte(m)); err != nil {
				fmt.Println(err)
			}
		case "disconnect":
			if err := nh.NetworkingServ.Disconnect(); err != nil {
				fmt.Println(err.Error())
			}
		default:
			fmt.Println("sosi")
		}
	}
}

func (nh *NetworkingHandler) Connect(receiversIds []string) error {
	ridsLen := len(receiversIds)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(ridsLen*2))
	defer cancel()
	if err := nh.NetworkingServ.Connect(ctx, receiversIds); err != nil {
		fmt.Printf("DEBUG: Error Type: %T, Error Value: %[1]v\n", err)
		return processError(err)
	}
	return nil
}

func (nh *NetworkingHandler) Disconnect() error {
	if err := nh.NetworkingServ.Disconnect(); err != nil {
		return processError(err)
	}
	return nil
}

func (nh *NetworkingHandler) SendMessage(msg string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	data := utils.SetFirstByte(networking.CHAT, []byte(msg))
	if err := nh.NetworkingServ.SendInStream(ctx, data); err != nil {
		return processError(err)
	}
	return nil
}

func (nh *NetworkingHandler) SendVoice(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	data = utils.SetFirstByte(networking.VOICE, data)
	if err := nh.NetworkingServ.SendDatagram(ctx, data); err != nil {
		return processError(err)
	}
	return nil
}

func (nh *NetworkingHandler) SendVideo(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	data = utils.SetFirstByte(networking.VIDEO, data)
	if err := nh.NetworkingServ.SendDatagram(ctx, data); err != nil {
		return processError(err)
	}
	return nil
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
