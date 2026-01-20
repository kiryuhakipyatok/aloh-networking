package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"networking/internal/client"
	"networking/internal/domain/models"
	"networking/internal/domain/repository"
	"networking/internal/protocol"
	"networking/internal/utils"
	"networking/pkg/errs"
	"strings"
	"time"

	"github.com/pion/ice/v2"
	"github.com/pion/logging"
	"github.com/pion/stun"
	"github.com/quic-go/quic-go"
	"golang.org/x/sync/errgroup"
)

const (
	CREDS = iota
	CANDIDATE
	CONNECTED = "Connected"
)

type NetworkingServ interface {
	Сonnect(rids []string) error
	SendMessage(msg string) error
	SendDatagram(data []byte) error
}

type networkingServ struct {
	SignalingClient client.SignalingClient
	AgentRepo       repository.SessionRepository
	receiveSDPs     chan protocol.ReplyMessage
}

func NewNetworkingServ(sc client.SignalingClient, ar repository.SessionRepository, receiveSDPs chan protocol.ReplyMessage) NetworkingServ {
	ns := &networkingServ{
		SignalingClient: sc,
		AgentRepo:       ar,
		receiveSDPs:     receiveSDPs,
	}
	go func() {
		if err := ns.receiveConnect(context.Background()); err != nil {
			panic(err)
		}
	}()
	return ns
}

func (ns *networkingServ) Сonnect(rids []string) error {
	sessions, err := ns.AgentRepo.Fetch(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println("num:", len(sessions))
	var errg errgroup.Group
	loggerFactory := logging.NewDefaultLoggerFactory()

	loggerFactory.DefaultLogLevel = logging.LogLevelTrace
	op := "networkingServ.Сonnect"
	for _, rid := range rids {
		errg.Go(func() error {
			agent, err := ice.NewAgent(&ice.AgentConfig{
				Urls: []*stun.URI{
					{Scheme: stun.SchemeTypeSTUN, Host: "164.90.163.153", Port: 3478, Proto: stun.ProtoTypeUDP},
					{Scheme: stun.SchemeTypeTURN, Host: "164.90.163.153", Port: 3478, Username: "testuser", Password: "testpass", Proto: stun.ProtoTypeTCP},
				},
				NetworkTypes: []ice.NetworkType{
					ice.NetworkTypeUDP4,
					ice.NetworkTypeUDP6,
					ice.NetworkTypeTCP4,
					ice.NetworkTypeTCP6,
				},
				// LoggerFactory: loggerFactory,
			})
			if err != nil {
				return errs.NewAppError(op, err)
			}
			session := &models.Session{
				UserID:      rid,
				Agent:       agent,
				IsInitiator: true,
				CredsChan:   make(chan struct{}, 1),
			}
			if err := ns.AgentRepo.Add(context.Background(), rid, session); err != nil {
				return errs.NewAppError(op, err)
			}
			localFrag, localPwd, err := agent.GetLocalUserCredentials()
			if err != nil {
				return errs.NewAppError(op, err)
			}
			creds := []byte(fmt.Sprintf("%s %s", localFrag, localPwd))
			byteSDP := make([]byte, 0, len(creds)+1)
			byteSDP = append(byteSDP, CREDS)
			byteSDP = append(byteSDP, creds...)
			fmt.Println(byteSDP[0])
			if err := ns.SignalingClient.NewSDP(byteSDP, []string{session.UserID}); err != nil {
				return errs.NewAppError(op, err)
			}
			agent.OnCandidate(func(c ice.Candidate) {
				if c == nil {
					return
				}
				candidate := []byte(c.Marshal())
				sdp := make([]byte, 0, len(candidate)+1)
				sdp = append(sdp, CANDIDATE)
				sdp = append(sdp, candidate...)
				fmt.Println(sdp[0])
				if err := ns.SignalingClient.NewSDP(sdp, []string{rid}); err != nil {
					panic(err)
				}
			})
			if err = agent.OnConnectionStateChange(func(c ice.ConnectionState) {
				fmt.Printf("ICE Connection State has changed: %s\n", c.String())
				session.State = c.String()
			}); err != nil {
				panic(err)
			}
			if err := agent.GatherCandidates(); err != nil {
				return errs.NewAppError(op, err)
			}
			fmt.Println("ccc")
			sessions, err := ns.AgentRepo.Fetch(context.Background())
			if err != nil {
				panic(err)
			}
			fmt.Println("num:", len(sessions))
			go func() {
				if err := ns.establishConnection(context.Background(), session); err != nil {
					panic(err)
				}
			}()
			return nil
		})

	}
	if err := errg.Wait(); err != nil {
		return errs.NewAppError(op, err)
	}
	return nil
}

func (ns *networkingServ) receiveConnect(ctx context.Context) error {
	op := "networkingServ.receiveConnect"
	loggerFactory := logging.NewDefaultLoggerFactory()

	loggerFactory.DefaultLogLevel = logging.LogLevelTrace

	for sdp := range ns.receiveSDPs {

		fmt.Println(string(sdp.Payload))
		var (
			remoteUrfrag string
			remotePwd    string
		)
		session, err := ns.AgentRepo.Get(ctx, sdp.Sender)
		if err != nil {
			if errors.Is(err, errs.ErrNotFoundBase) {
				sessions, err := ns.AgentRepo.Fetch(context.Background())
				if err != nil {
					panic(err)
				}
				fmt.Println("num:", len(sessions))
				agent, err := ice.NewAgent(&ice.AgentConfig{
					Urls: []*stun.URI{
						{Scheme: stun.SchemeTypeSTUN, Host: "164.90.163.153", Port: 3478, Proto: stun.ProtoTypeUDP},
						{Scheme: stun.SchemeTypeTURN, Host: "164.90.163.153", Port: 3478, Username: "testuser", Password: "testpass", Proto: stun.ProtoTypeTCP},
					},
					NetworkTypes: []ice.NetworkType{
						ice.NetworkTypeUDP4,
						ice.NetworkTypeUDP6,
						ice.NetworkTypeTCP4,
						ice.NetworkTypeTCP6,
					},
					// LoggerFactory: loggerFactory,
				})
				if err != nil {
					return errs.NewAppError(op, err)
				}
				newSession := &models.Session{
					UserID:      sdp.Sender,
					Agent:       agent,
					IsInitiator: false,
					CredsChan:   make(chan struct{}, 1),
				}
				if err := ns.AgentRepo.Add(context.Background(), sdp.Sender, newSession); err != nil {
					return errs.NewAppError(op, err)
				}
				session = newSession
				localFrag, localPwd, err := agent.GetLocalUserCredentials()
				if err != nil {
					return errs.NewAppError(op, err)
				}
				creds := []byte(fmt.Sprintf("%s %s", localFrag, localPwd))
				byteSDP := make([]byte, 0, len(creds)+1)
				byteSDP = append(byteSDP, CREDS)
				byteSDP = append(byteSDP, creds...)
				fmt.Println(byteSDP[0])
				if err := ns.SignalingClient.NewSDP(byteSDP, []string{sdp.Sender}); err != nil {
					return errs.NewAppError(op, err)
				}
				agent.OnCandidate(func(c ice.Candidate) {
					if c == nil {
						return
					}
					candidate := []byte(c.Marshal())
					byteSDP := make([]byte, 0, len(candidate)+1)
					byteSDP = append(byteSDP, CANDIDATE)
					byteSDP = append(byteSDP, candidate...)
					fmt.Println(byteSDP[0])
					if err := ns.SignalingClient.NewSDP(byteSDP, []string{sdp.Sender}); err != nil {
						panic(err)
					}
				})
				if err = agent.OnConnectionStateChange(func(c ice.ConnectionState) {
					fmt.Printf("ICE Connection State has changed: %s\n", c.String())
					session.State = c.String()
				}); err != nil {
					panic(err)
				}
				if err := agent.GatherCandidates(); err != nil {
					return errs.NewAppError(op, err)
				}
				fmt.Println("bbb")
				sessions, err = ns.AgentRepo.Fetch(context.Background())
				if err != nil {
					panic(err)
				}
				fmt.Println("num:", len(sessions))
				go func() {
					if err := ns.establishConnection(context.Background(), session); err != nil {
						panic(err)
					}
				}()

			} else {
				return errs.NewAppError(op, err)
			}

		}
		fmt.Println("mmm")
		switch sdp.Payload[0] {
		case CREDS:
			fmt.Println("creds")
			creds := strings.Split(string(sdp.Payload[1:]), " ")
			remoteUrfrag = creds[0]
			remotePwd = creds[1]
			fmt.Println(remoteUrfrag)
			fmt.Println(remotePwd)
			session.Agent.SetRemoteCredentials(remoteUrfrag, remotePwd)
			select {
			case session.CredsChan <- struct{}{}:
			default:
			}

		case CANDIDATE:
			fmt.Println("candidate")
			fmt.Println("sdp: ", string(sdp.Payload[1:]))
			c, err := ice.UnmarshalCandidate(string(sdp.Payload[1:]))
			if err != nil {
				return errs.NewAppError(op, err)
			}
			if err := session.Agent.AddRemoteCandidate(c); err != nil {
				return errs.NewAppError(op, err)
			}
		}
	}

	return nil
}

func (ns *networkingServ) establishConnection(ctx context.Context, session *models.Session) error {
	op := "networkingServ.establishConnection"
	select {
	case <-session.CredsChan:
		fmt.Println("connecting")
	case <-ctx.Done():
		panic(ctx.Err())
	}
	var (
		conn     *ice.Conn
		quicConn *quic.Conn
	)
	remoteUfrag, remotePwd, err := session.Agent.GetRemoteUserCredentials()
	if err != nil {
		return errs.NewAppError(op, err)
	}
	quicConf := &quic.Config{
		KeepAlivePeriod: time.Second * 10,
		EnableDatagrams: true,
	}
	switch session.IsInitiator {
	case true:
		conn, err = session.Agent.Dial(ctx, remoteUfrag, remotePwd)
		if err != nil {
			return errs.NewAppError(op, err)
		}
		packetConn := &PacketConnWrapper{
			Conn: conn,
		}

		t := quic.Transport{
			Conn: packetConn,
		}
		fmt.Println("addr", conn.RemoteAddr())
		tlsConf := &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"networking-p2p"},
		}
		quicConn, err = t.Dial(ctx, conn.RemoteAddr(), tlsConf, quicConf)
		if err != nil {
			return errs.NewAppError(op, err)
		}

	case false:
		conn, err = session.Agent.Accept(ctx, remoteUfrag, remotePwd)
		if err != nil {
			return errs.NewAppError(op, err)
		}
		packetConn := &PacketConnWrapper{
			Conn: conn,
		}
		t := quic.Transport{
			Conn: packetConn,
		}
		tlsConf := utils.GenerateTLSConfig()
		listener, err := t.Listen(tlsConf, quicConf)
		if err != nil {
			return errs.NewAppError(op, err)
		}
		quicConn, err = listener.Accept(ctx)
		if err != nil {
			return errs.NewAppError(op, err)
		}
	}
	session.Conn = quicConn
	go ns.handleConnection(ctx, quicConn)
	return nil
}

func (ns *networkingServ) handleConnection(ctx context.Context, quicConn *quic.Conn) {
	buf := make([]byte, 128)
	go func() {
		for {
			data, err := quicConn.ReceiveDatagram(ctx)
			if err != nil {
				panic(err)
			}
			fmt.Println(string(data))
		}
	}()
	for {
		stream, err := quicConn.AcceptUniStream(ctx)
		if err != nil {
			panic(err)
		}
		n, err := stream.Read(buf)
		fmt.Println(string(buf[:n]))
		stream.CancelRead(0)
	}
}

func (ns *networkingServ) SendMessage(msg string) error {
	op := "networkingServ.SensdMessage"
	sessions, err := ns.AgentRepo.Fetch(context.Background())
	if err != nil {
		return errs.NewAppError(op, err)
	}
	fmt.Println(len(sessions))
	for _, s := range sessions {
		if s.State == CONNECTED {
			stream, err := s.Conn.OpenUniStreamSync(context.Background())
			if err != nil {
				return errs.NewAppError(op, err)
			}
			if _, err := stream.Write([]byte(msg)); err != nil {
				return errs.NewAppError(op, err)
			}
			stream.Close()
		} else {
			fmt.Println("disconnected")
		}

	}
	return nil
}

func (ns *networkingServ) SendDatagram(data []byte) error {
	op := "networkingServ.SendDatagram"
	sessions, err := ns.AgentRepo.Fetch(context.Background())
	if err != nil {
		return errs.NewAppError(op, err)
	}
	fmt.Println(len(sessions))
	for _, s := range sessions {
		if s.State == CONNECTED {
			if err := s.Conn.SendDatagram(data); err != nil {
				return errs.NewAppError(op, err)
			}
		} else {
			fmt.Println("disconnected")
		}
	}
	return nil
}
