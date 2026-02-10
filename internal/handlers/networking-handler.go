package handlers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"networking/config"
	"networking/internal/domain/services/networking"
	"networking/internal/utils"
	"os"
	"strings"
)

type NetworkingHandler struct {
	NetworkingServ networking.NetworkingServ
	Cfg            config.Handler
}

func NewNetworkingHandler(ns networking.NetworkingServ, cfg config.Handler) *NetworkingHandler {
	nh := &NetworkingHandler{
		NetworkingServ: ns,
		Cfg:            cfg,
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
				fmt.Println(err.Error())
			}
		case "sendMsg":
			m, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Println(err.Error())
				break
			}
			m = strings.TrimSpace(m)
			if err := nh.SendMessage(m); err != nil {
				fmt.Println(err.Error())
			}
		case "sendVoice":
			m, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Println(err.Error())
				break
			}
			m = strings.TrimSpace(m)
			if err := nh.SendVoice([]byte(m)); err != nil {
				fmt.Println(err)
			}
		case "sendVideo":
			m, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Println(err.Error())
				break
			}
			m = strings.TrimSpace(m)
			if err := nh.SendVideo([]byte(m)); err != nil {
				fmt.Println(err.Error())
			}
		case "disconnect":
			if err := nh.Disconnect(); err != nil {
				fmt.Println(err.Error())
			}
		case "online":
			online, err := nh.FetchOnline()
			if err != nil {
				fmt.Println(err.Error())
			} else {
				fmt.Println(online)
			}
		case "sessionsId":
			m, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Println(err.Error())
				break
			}
			m = strings.TrimSpace(m)
			sessions, err := nh.FetchSessionById(m)
			if err != nil {
				fmt.Println(err.Error())
			} else {
				fmt.Println(sessions)
			}
		default:
			fmt.Println("sosi")
		}
	}
}

func (nh *NetworkingHandler) Connect(receiversIds []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), nh.Cfg.ConnectTimeout)
	defer cancel()
	if err := nh.NetworkingServ.Connect(ctx, receiversIds); err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), nh.Cfg.SendChatTimeout)
	defer cancel()
	data := utils.SetFirstByte(networking.CHAT, []byte(msg))
	if err := nh.NetworkingServ.SendInStream(ctx, data); err != nil {
		return processError(err)
	}
	return nil
}

func (nh *NetworkingHandler) SendVoice(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), nh.Cfg.SendVoiceTimeout)
	defer cancel()
	data = utils.SetFirstByte(networking.VOICE, data)
	if err := nh.NetworkingServ.SendDatagram(ctx, data); err != nil {
		return processError(err)
	}
	return nil
}

func (nh *NetworkingHandler) SendVideo(data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), nh.Cfg.SendVideoTimeout)
	defer cancel()
	data = utils.SetFirstByte(networking.VIDEO, data)
	if err := nh.NetworkingServ.SendDatagram(ctx, data); err != nil {
		return processError(err)
	}
	return nil
}

func (nh *NetworkingHandler) OnChat(f func(id string, data []byte)) {
	nh.NetworkingServ.SaveChatHandler(f)
}

func (nh *NetworkingHandler) OnVideo(f func(id string, data []byte)) {
	nh.NetworkingServ.SaveVideoHandler(f)
}

func (nh *NetworkingHandler) OnVoice(f func(id string, data []byte)) {
	nh.NetworkingServ.SaveVoiceHandler(f)
}

func (nh *NetworkingHandler) FetchOnline() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nh.Cfg.FetchOnlineTimeout)
	defer cancel()
	online, err := nh.NetworkingServ.FetchOnline(ctx)
	if err != nil {
		return nil, processError(err)
	}
	return online, nil
}

func (nh *NetworkingHandler) FetchSessionById(id string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nh.Cfg.FetchOnlineTimeout)
	defer cancel()
	sessions, err := nh.NetworkingServ.FetchSessionsById(ctx, id)
	if err != nil {
		return nil, processError(err)
	}
	return sessions, nil
}
