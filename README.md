# aloh Networking

A real-time **peer-to-peer (P2P) networking library** built in Go, providing secure, low-latency communication channels for chat, voice, video (webcam/screen share), and real-time event streaming between users. The library establishes direct connections between peers using **QUIC over ICE** (WebRTC-compatible NAT traversal) and end-to-end encryption (E2EE) via **AES-GCM** with **X25519** key exchange.

---

## Table of Contents

- [Overview](#overview)
  - [How It Works](#how-it-works)
- [Architecture](#architecture)
  - [Component Breakdown](#component-breaksemble)
- [Key Features](#key-features)
- [Technology Stack](#technology-stack)
  - [Core Dependencies](#core-dependencies)
  - [Signaling Dependency](#signaling-dependency)
  - [Build Tools](#build-tools)
- [Project Structure](#project-structure)
- [Installation](#installation)
  - [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
  - [config.App](#configapp)
  - [config.Signaling](#configsignaling)
  - [config.Networking](#confignetworking)
  - [config.Handler](#confighandler)
  - [Environment Variables](#environment-variables)
- [API Reference](#api-reference)
  - [Connection Management](#connection-management)
  - [Sending Data](#sending-data)
  - [Event Streaming](#event-streaming)
  - [Callbacks](#callbacks)
  - [Fetching Data](#fetching-data)
- [Message Types](#message-types)
- [Signaling Protocol](#signaling-protocol)
- [Error Codes](#error-codes)
- [Event Types](#event-types)
  - [Helper Functions](#helper-functions)
- [Error Handling](#error-handling)
- [End-to-End Encryption](#end-to-end-encryption)
  - [Encryption Flow](#encryption-flow)
  - [E2EE Components](#e2ee-components)
- [C API](#c-api)
  - [Building the Shared Library](#building-the-shared-library)
  - [Exposed C Functions](#exposed-c-functions)
- [Docker](#docker)
- [Build from Source](#build-from-source)
- [Testing](#testing)
- [License](#license)
- [Related Projects](#related-projects)
- [Acknowledgments](#acknowledgments)

---

## Overview

aloh Networking is a networking library designed to power real-time communication (RTC) applications. It enables direct peer-to-peer communication between users, bypassing the need for relay servers for media/data transfer after the initial handshake.

### How It Works

1. **Signaling**: Each client connects to a central **signaling server** using **QUIC** (over TLS) to register, discover peers, and exchange connection metadata (credentials, ICE candidates).
2. **ICE Agent**: Using the **Pion ICE** library, peers perform NAT traversal to establish a direct UDP/TCP connection.
3. **QUIC Transport**: Once the ICE connection is established, peers negotiate a **QUIC** session for multiplexed, encrypted transport.
4. **E2EE Key Exchange**: Peers exchange ECDH public keys over QUIC and derive a shared AES-GCM key, ensuring all media and data streams are encrypted.
5. **Media Streams**:
   - **Streams**: Chat messages, voice, webcam, and screen data are sent via QUIC **unidirectional streams**.
   - **Datagrams**: Voice data can also be sent via **QUIC datagrams** for lower latency.
6. **Event Stream**: A dedicated QUIC stream carries control events (mute toggles, denoising, webcam/screen state changes, key frames).

---

## Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                       aloh Networking                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐      ┌──────────────────┐   ┌────────────┐ │
│  │  Signaling  │      │    ICE Agent     │   │ QUIC Layer │ │
│  │   Client    │─────►│    (Pion ICE)    │──►│ (quic-go)  │ │
│  │   (QUIC)    │      │  (STUN + TURN)   │   │   + E2EE   │ │
│  └─────────────┘      └──────────────────┘   └────────────┘ │
│         │                      │                   │        │
│         ▼                      ▼                   ▼        │
│  ┌─────────────┐      ┌──────────────────┐   ┌────────────┐ │
│  │ Repository  │      │   Event Stream   │   │Data Streams│ │
│  │ (in-memory) │      │   (JSON over     │   │Chat/Voice/ │ │
│  │Session Store│      │      QUIC)       │   │Webcam/Scrn │ │
│  └─────────────┘      └──────────────────┘   └────────────┘ │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │               Handler (External API)                 │   │
│  │  - Connect / Disconnect                              │   │
│  │  - Send Message / Voice / Webcam / Screen            │   │
│  │  - Register Callbacks                                │   │
│  │  - Fetch Online / Sessions / Friends                 │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Component Breakdown

| Layer | Package | Description |
| :--- | :--- | :--- |
| **Public API** | `alohnetwork` | Root package — exports `Netwoking`, config types, events, and public methods |
| **Handler** | `internal/handlers` | `NetworkingHandler` — bridges public API calls to internal domain services |
| **App Init** | `internal/app` | Initializes loggers, repositories, signaling clients, and network services |
| **Networking Service** | `internal/domain/services/networking` | Core business logic: session lifecycle, ICE/QUIC orchestration, E2EE setup |
| **Signaling Client** | `internal/client` | QUIC client for communicating with the central signaling server |
| **Session Model** | `internal/domain/models` | In-memory session data structures |
| **Repository** | `internal/domain/repository` | Thread-safe session store implemented with `sync.Map` |
| **E2EE** | `internal/domain/e2ee` | End-to-end encryption using X25519 + AES-GCM |
| **Errors** | `pkg/errs` | Structured error types and unified error code mappings |
| **Logger** | `pkg/logger` | Structured logging with `slog` (supports `local`, `dev`, `prod` modes) |

---

## Key Features

- **🔒 End-to-End Encryption**: All peer-to-peer data is encrypted with AES-GCM using an ECDH-derived shared secret. No plaintext media or data is ever visible to the signaling server.
- **🌐 NAT Traversal**: Uses Pion ICE with STUN and TURN servers for reliable peer connection across strict NAT boundaries.
- **⚡ QUIC Transport**: Built on `quic-go` for multiplexed streams, built-in TLS, connection migration, and datagram support.
- **📡 Signaling via QUIC**: The signaling channel uses QUIC instead of WebSockets for lower connection latency and multiplexed messaging.
- **🔄 Auto-Reconnect**: Exponential backoff reconnection for dropped signaling connections; ICE-level reconnection for lost peer links.
- **📊 Multiple Media Types**: Independent stream pipelines for chat, voice, webcam, and screen sharing.
- **🎯 Event Streaming**: A dedicated control channel for real-time signaling events (mute, denoise, keyframe requests).
- **🧵 Thread-Safe**: Employs `atomic.Value` for callback handlers and `sync.Map` for internal storage.
- **🔧 Highly Configurable**: Fine-grained timeout control across all connection phases, configurable via `.env`.
- **📦 C-Compatible**: Compiles to native shared libraries (`libnetworking.so` / `libnetworking.dll`) and static archives with a clean C header interface.

---

## Technology Stack

### Core Dependencies

| Dependency | Version | Purpose |
| :--- | :--- | :--- |
| `github.com/pion/ice/v4` | `v4.4.4` | ICE agent for NAT traversal |
| `github.com/pion/stun/v4` | `v4.0.1` | STUN protocol implementation |
| `github.com/quic-go/quic-go` | `v0.59.0` | QUIC transport layer |
| `github.com/cenkalti/backoff/v5` | `v5.0.3` | Exponential backoff for retries |
| `github.com/google/uuid` | `v1.6.0` | User and session UUID management |
| `github.com/joho/godotenv` | `v1.5.1` | `.env` configuration loader |
| `golang.org/x/crypto` | `v0.48.0` | Cryptographic primitives (HKDF, etc.) |
| `golang.org/x/sync` | `v0.19.0` | `errgroup` and `singleflight` concurrency primitives |

### Signaling Dependency

| Dependency | Purpose |
| :--- | :--- |
| `github.com/kiryuhakipyatok/aloh-signalling` | Shared signaling protocol definitions |

### Build Tools

| Tool | Purpose |
| :--- | :--- |
| **CMake** | Cross-platform build system for C archive compilation |
| **Clang / MinGW** | C cross-compilation for shared library targets |
| **Docker** | Containerized deployment and multi-node test environment |

---

## Project Structure

```text
aloh-networking/
├── cmd/
│   ├── app/
│   │   └── main.go                    # Application entry point (standalone)
│   └── c-api/
│       └── main.go                    # C-compatible shared library entry point
├── config/
│   └── config.go                      # Configuration structures
├── internal/
│   ├── app/
│   │   └── app.go                     # Application initialization and lifecycle
│   ├── client/
│   │   ├── signaling-client.go        # QUIC signaling client
│   │   ├── protocol.go                # Protocol constants and types
│   │   └── helper.go                  # Connection management helpers
│   ├── domain/
│   │   ├── e2ee/
│   │   │   ├── e2ee.go                # Key generation and AES-GCM
│   │   │   ├── secure-datagram.go     # E2EE for QUIC datagrams
│   │   │   └── secure-stream.go       # Generic E2EE stream wrapper
│   │   ├── models/
│   │   │   └── session.go             # Session data model
│   │   ├── repository/
│   │   │   └── session-repo.go        # Thread-safe session store
│   │   └── services/
│   │       └── networking/
│   │           ├── network-serv.go    # Core networking service
│   │           ├── handlers.go        # Handler storage
│   │           ├── helpers.go         # Connection helpers
│   │           ├── protocol.go        # Networking protocol constants
│   │           ├── utils.go           # E2EE setup utilities
│   │           └── wrapper.go         # PacketConn adapter for ICE
│   ├── handlers/
│   │   └── networking-handler.go      # Public API handler
│   └── utils/
│       └── utils.go                   # TLS generation, error checking
├── pkg/
│   ├── errs/
│   │   ├── app/
│   │   │   └── app-errs.go            # Application error types
│   │   └── handlers/
│   │       └── handlers-errs.go       # Error code mapping
│   └── logger/
│       ├── logger.go                  # Structured logger (slog)
│       └── sparce-logger.go           # Sparse logging utility
├── CMakeLists.txt                     # CMake build configuration
├── Dockerfile                         # Docker image build
├── docker-compose.yaml                # Multi-user test deployment
├── Makefile                           # Build/run/test targets
├── go.mod                             # Go module definition
├── networking.go                      # Root package — Networking struct
├── callbacks.go                       # Callback registration methods
├── event.go                           # Event types and factory functions
├── fetchers.go                        # Data fetching methods
├── senders.go                         # Data sending methods
├── connetions.go                      # Connection management methods
├── config.go                          # Config type aliases
├── .env                               # Environment variables
└── .gitlab-ci.yml                     # CI/CD pipeline
```

---

## Installation

### As a Go Library

```bash
go get github.com/kiryuhakipyatok/aloh-networking
```

### Prerequisites

- **Go 1.26+** (uses modern generics and `errors.Join`)
- **C compiler** (`gcc` or `clang` for CGO / C-API builds)
- Access to an active signaling server (`aloh-signalling`)

---

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/kiryuhakipyatok/aloh-networking"
	"github.com/kiryuhakipyatok/aloh-networking/config"
)

func main() {
	cfg := config.Config{
		App: config.App{
			Name:           "my-app",
			Version:        "1.0.0",
			Env:            "dev",
			LogPath:        "",
			SendSDPSize:    100,
			ReceiveSDPSize: 100,
		},
		Signaling: config.Signaling{
			Address:                "signaling.example.com",
			Port:                   "443",
			MaxIdleTimeout:         30 * time.Second,
			HandshakeTimeout:       10 * time.Second,
			KeepAlivePeriodTimeout: 15 * time.Second,
			CloseTimeout:           5 * time.Second,
			StartTimeout:           5 * time.Second,
			NextProtos:             []string{"aloh-signaling"},
			MaxIncomingStreams:     100,
			MaxIncomingUniStreams:  100,
			RegTimeout:             10 * time.Second,
		},
		Networking: config.Networking{
			MaxIdleTimeout:         30 * time.Second,
			HandshakeTimeout:       10 * time.Second,
			KeepAlivePeriodTimeout: 15 * time.Second,
			NextProtos:             []string{"aloh-networking"},
			STUNHost:               "stun.l.google.com",
			STUNPort:               19302,
			TURNHost:               "turn.example.com",
			TURNPort:               3478,
			NewSDPTimeout:          5 * time.Second,
			SendInStreamTimeout:    10 * time.Second,
			EstablishConnTimeout:   30 * time.Second,
			DatagramLogTargetCount: 100,
			FetchLogTargetCount:    100,
			DisconnectedTimeout:    10 * time.Second,
		},
		Handler: config.Handler{
			SendVideoTimeout:   5 * time.Second,
			SendChatTimeout:    5 * time.Second,
			SendVoiceTimeout:   5 * time.Second,
			ConnectTimeout:     30 * time.Second,
			DisonnectTimeout:   5 * time.Second,
			FetchOnlineTimeout: 10 * time.Second,
		},
	}

	userID := uuid.New()
	networking, err := alohnetwork.NewNetworking(userID, cfg)
	if err != nil {
		panic(err)
	}
	defer networking.Delete()

	// Register callbacks
	networking.RegisterOnChat(func(id uuid.UUID, data []byte) {
		fmt.Printf("Message from %s: %s\n", id, string(data))
	})

	networking.RegisterOnPeerConnected(func(id uuid.UUID) {
		fmt.Printf("Peer connected: %s\n", id)
	})

	networking.RegisterOnPeerDisconnected(func(id uuid.UUID) {
		fmt.Printf("Peer disconnected: %s\n", id)
	})

	networking.RegisterOnVoice(func(id uuid.UUID, data []byte) {
		// Handle incoming voice frame
	})

	networking.RegisterOnWebcam(func(id uuid.UUID, data []byte) {
		// Render webcam frame
	})

	networking.RegisterOnScreen(func(id uuid.UUID, data []byte) {
		// Render desktop frame
	})

	networking.RegisterOnEvent(func(id uuid.UUID, e alohnetwork.Event) {
		fmt.Printf("Event from %s: type=%d, timestamp=%d\n", id, e.Typee, e.Timestamp)
	})

	// Discover online peers
	online, _ := networking.FetchOnline()
	fmt.Println("Online users:", online)

	if len(online) > 0 {
		peerID := online[0]
		if err := networking.Connect(peerID); err != nil {
			panic(err)
		}

		// Send a chat message
		_ = networking.SendMessage([]byte("Hello, World!"))

		// Send a microphone mute event
		ev, _ := alohnetwork.MuteMicEvent(true)
		_ = networking.SendEvent(ev)
	}

	// Wait for termination signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("Shutting down...")
}
```

---

## Configuration

The library is configured via `config.Config` consisting of four primary sections:

### config.App

| Field | Type | Description |
| :--- | :--- | :--- |
| `Name` | `string` | Application identifier (used in logs) |
| `Version` | `string` | Application version tag |
| `Env` | `string` | Logging mode: `"local"`, `"dev"`, or `"prod"` |
| `ReceiveSDPSize` | `int` | Buffer capacity for receiving SDP channels |
| `SendSDPSize` | `int` | Buffer capacity for outgoing SDP channels |
| `LogPath` | `string` | Path to log file (empty = stdout) |

### config.Signaling

| Field | Type | Description |
| :--- | :--- | :--- |
| `Address` | `string` | Signaling server hostname or IP |
| `Port` | `string` | Signaling server port |
| `MaxIdleTimeout` | `time.Duration` | QUIC maximum idle timeout |
| `HandshakeTimeout` | `time.Duration` | QUIC handshake timeout |
| `KeepAlivePeriodTimeout` | `time.Duration` | Heartbeat keep-alive interval |
| `CloseTimeout` | `time.Duration` | Maximum wait duration on connection teardown |
| `StartTimeout` | `time.Duration` | Dial timeout |
| `NextProtos` | `[]string` | ALPN protocol negotiation slice |
| `MaxIncomingStreams` | `int64` | Max permitted incoming bidirectional streams |
| `MaxIncomingUniStreams` | `int64` | Max permitted incoming unidirectional streams |
| `RegTimeout` | `time.Duration` | Client registration timeout |

### config.Networking

| Field | Type | Description |
| :--- | :--- | :--- |
| `MaxIdleTimeout` | `time.Duration` | Peer QUIC maximum idle timeout |
| `HandshakeTimeout` | `time.Duration` | Peer QUIC handshake timeout |
| `KeepAlivePeriodTimeout` | `time.Duration` | Peer keep-alive heartbeat interval |
| `NextProtos` | `[]string` | ALPN protocols for peer QUIC session |
| `STUNHost` | `string` | Hostname of the STUN server |
| `STUNPort` | `int` | Port of the STUN server |
| `TURNHost` | `string` | Hostname of the TURN relay |
| `TURNPort` | `int` | Port of the TURN relay |
| `NewSDPTimeout` | `time.Duration` | Deadline for exchanging SDP offers/answers |
| `SendInStreamTimeout` | `time.Duration` | Timeout for writing data to streams |
| `EstablishConnTimeout` | `time.Duration` | End-to-end peer connection establishment deadline |
| `DatagramLogTargetCount` | `uint32` | Sparse log counter for datagram transmission |
| `FetchLogTargetCount` | `uint32` | Sparse log counter for query operations |
| `DisconnectedTimeout` | `time.Duration` | ICE disconnection timeout |

### config.Handler

| Field | Type | Description |
| :--- | :--- | :--- |
| `SendVideoTimeout` | `time.Duration` | Timeout for streaming video chunks |
| `SendChatTimeout` | `time.Duration` | Timeout for delivering chat text |
| `SendVoiceTimeout` | `time.Duration` | Timeout for dispatching audio frames |
| `ConnectTimeout` | `time.Duration` | Timeout on initiate connection call |
| `DisonnectTimeout` | `time.Duration` | Timeout on graceful session disconnect |
| `FetchOnlineTimeout` | `time.Duration` | Timeout when querying online roster |

### Environment Variables

Configuration parameters can be loaded from a `.env` file located in the application root or executable directory:

```dotenv
CONFIG_PATH=../../configs/app
CONFIG_NAME=config.yaml
TURN_USERNAME=testuser
TURN_PASSWORD=testpass
```

---

## API Reference

All primary methods are exposed on the `*alohnetwork.Netwoking` struct.

### Connection Management

- `NewNetworking(userId uuid.UUID, cfg config.Config) (*Netwoking, error)`  
  Initializes a new client instance for the specified user UUID.
- `Connect(id uuid.UUID) error`  
  Initiates a connection with a peer. Reconciles multiple sessions automatically if present.
- `ConnectById(id uuid.UUID) error`  
  Directly dials a peer connection by UUID without resolving remote session pools.
- `Disconnect() error`  
  Terminates all active peer sessions.
- `DisconnectById(id uuid.UUID) error`  
  Disconnects a specific peer identified by UUID.
- `Delete()`  
  Frees resources, halts contexts, disconnects peers, and closes the signaling connection.

### Sending Data

- `SendMessage(msg []byte) error`  
  Sends a chat message payload to all connected peers.
- `SendVoice(data []byte) error`  
  Dispatches voice audio data to connected peers via unidirectional QUIC streams.
- `SendWebcam(data []byte) error`  
  Sends webcam video frames via unidirectional QUIC streams.
- `SendScreen(data []byte) error`  
  Transmits screen sharing video frames to all peers.
- `SendEvent(e Event) error`  
  Dispatches control events over the dedicated event stream.

### Event Streaming

Refer to the [Event Types](#event-types) section for factory constructors.

### Callbacks

All callbacks are thread-safe and can be registered either before or after establishing peer connections:

- `RegisterOnChat(cb func(id uuid.UUID, data []byte))`
- `RegisterOnVoice(cb func(id uuid.UUID, data []byte))`
- `RegisterOnWebcam(cb func(id uuid.UUID, data []byte))`
- `RegisterOnScreen(cb func(id uuid.UUID, data []byte))`
- `RegisterOnPeerConnected(cb func(id uuid.UUID))`
- `RegisterOnPeerDisconnected(cb func(id uuid.UUID))`
- `RegisterOnEvent(cb func(id uuid.UUID, e Event))`

### Fetching Data

- `FetchOnline() ([]uuid.UUID, error)`  
  Retrieves a list of all currently registered online user IDs.
- `FetchSessions(id uuid.UUID) ([]uuid.UUID, error)`  
  Returns active peer session IDs for a specific user ID.
- `FetchFriends(ids []uuid.UUID) (map[uuid.UUID][]string, error)`  
  Resolves an array of friend IDs into a map of active session addresses.

---

## Message Types

Raw payloads prepend a single type byte before the body:

| Constant | Value | Description |
| :--- | :---: | :--- |
| `CHAT` | `0` | Text/image chat message |
| `VOICE` | `1` | Real-time audio stream |
| `WEBCAM` | `2` | Webcam video frame |
| `SCREEN` | `3` | Screen capture video frame |

---

## Signaling Protocol

The signaling client communicates over a JSON protocol defined in `aloh-signalling`:

| Constant | Description |
| :--- | :--- |
| `REG_TYPE` | User session registration |
| `STREAM_TYPE` | Payload/stream initialization message |
| `DATAGRAM_TYPE` | Datagram transmission packet |
| `DISCONN_TYPE` | Graceful peer disconnect alert |
| `GET_ONLINE_TYPE` | Query online peer roster |
| `ADD_IN_SESSION` | Attach user to existing session |
| `GET_SESSIONS_BY_ID` | Fetch active session list for a user |
| `DELETE_FROM_SESSION` | Remove user from active session |
| `GET_ONLINE_FRIENDS` | Filter friend list by online status |

---

## Error Codes

| Code | Name | Description |
| :---: | :--- | :--- |
| `0` | `SUCCESS` | Operation executed successfully |
| `1` | `NOT_FOUND` | Target resource does not exist |
| `2` | `ALREADY_EXISTS` | Target resource already present |
| `3` | `REQUEST_TIMEOUT` | Request context exceeded deadline |
| `4` | `VALIDATION_ERROR` | Schema validation failed |
| `5` | `CHAT_ERROR` | Error originating from chat pipeline |
| `6` | `VOICE_ERROR` | Error originating from audio pipeline |
| `7` | `VIDEO_ERROR` | Error originating from video pipeline |
| `8` | `OFFLINE` | Remote endpoint or signaling is unreachable |
| `9` | `INTERNAL_ERROR` | Unhandled internal runtime error |

---

## Event Types

| Constant | Description | Data Format |
| :--- | :--- | :--- |
| `FULL_MUTE` | Toggle full audio mute (deafen) | `bool` (JSON) |
| `MIC_MUTE` | Toggle microphone mute | `bool` (JSON) |
| `HARD_DENOISE` | Toggle hard denoise filter | `bool` (JSON) |
| `SOFT_DENOISE` | Toggle soft denoise filter | `bool` (JSON) |
| `WEBCAM_STATE` | Webcam active/inactive status | `bool` (JSON) |
| `SCREEN_STATE` | Desktop capture active/inactive status | `bool` (JSON) |
| `WEBCAM_KEY_FRAME` | Request keyframe from webcam stream | *Empty* |
| `SCREEN_KEY_FRAME` | Request keyframe from screen share | *Empty* |
| `GENERAL` | Bundled composite state update | `GeneralData` (JSON) |

### Helper Functions

```go
MuteMicEvent(state bool) (Event, error)
MuteFullEvent(state bool) (Event, error)
HardDenoiseEvent(state bool) (Event, error)
SoftDenoiseEvent(state bool) (Event, error)
WebcamEvent(state bool) (Event, error)
ScreenEvent(state bool) (Event, error)
WebcamKeyFrameEvent() (Event, error)
ScreenKeyFrameEvent() (Event, error)
GeneralEvent(fm, mm, hd, sd bool) (Event, error)
DataToState(data []byte) (bool, error)
DataToGeneral(data []byte) (GeneralData, error)
```

```go
type GeneralData struct {
    FullMute    bool `json:"full-mute"`
    MicMute     bool `json:"mic-mute"`
    HardDenoise bool `json:"hard-denoise"`
    SoftDenoise bool `json:"soft-denoise"`
}
```

---

## Error Handling

Errors carry operation tags (`Op`) for distributed call-chain tracing:

```go
type AppError struct {
    Op  string
    Err error
}
```

Key sentinel values:
- `ErrNotFoundBase`
- `ErrAlreadyExistsBase`
- `ErrRequestTimeoutBase`
- `ErrInvalidJsonBase`
- `ErrOfflineBase`
- `ErrConnToHimselfBase`
- `AppClosingBase`

---

## End-to-End Encryption

- **X25519 ECDH**: Ephemeral keypair generated per session (`crypto/ecdh`).
- **HKDF-SHA256**: Key derivation transforms ECDH raw shared secret into a 32-byte AES key.
- **AES-256-GCM**: Cryptographic authenticated cipher with per-message nonce and `"aloh-hell-yeah"` AAD.

### Encryption Flow

```text
1. Handshake:
   Peer A ───[ X25519 Public Key via QUIC Stream ]───► Peer B
   Peer B ───[ X25519 Public Key via QUIC Stream ]───► Peer A

2. Key Derivation:
   Shared Secret = ECDH(LocalPrivate, RemotePublic)
   AES-256 Key   = HKDF-SHA256(Shared Secret)

3. Transport:
   Payload ──► AES-GCM (Random Nonce + AAD) ──► Encrypted QUIC Stream
```

### E2EE Components

| Component | Purpose |
| :--- | :--- |
| `e2ee.NewKeys()` | Generates ephemeral X25519 keypair |
| `e2ee.NewAESCM(key)` | Instantiates AES-GCM cipher |
| `e2ee.NewMasterKey(priv, pub)` | Derives 32-byte master key via HKDF |
| `e2ee.CipherPayload(aead, data)` | Encrypts generic payload |
| `e2ee.DecipherPayload(aead, data)` | Decrypts generic payload |
| `e2ee.CipherDatagram(datagram, key)` | Encrypts raw datagram bytes |
| `e2ee.DecipherDatagram(datagram, key)` | Decrypts raw datagram bytes |
| `e2ee.NewSecureStream(stream, key)` | Wraps a QUIC stream in an E2EE abstraction |

---

## C API

### Building the Shared Library

```bash
# Linux (.so)
make build-lib

# Windows (.dll cross-compiled)
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -o libnetworking.dll -buildmode=c-shared \
  -ldflags "-s -w -extldflags '-Wl,--output-def,libnetworking.def'" \
  cmd/c-api/main.go

# CMake
mkdir build && cd build
cmake ..
make
```

### Exposed C Functions

```c
// Create and destroy handler
handler NewHandler(const char* userID, const char* logPath);
void    DeleteHandler(handler h);

// Connection management
uint    Connect(handler h, const char* receiverID);
uint    Disconnect(handler h);

// Sending
uint    SendMessage(handler h, void* msg, int length);
uint    SendVoice(handler h, void* data, int length);
uint    SendVideo(handler h, void* data, int length);

// Receiving
void    RegisterOnChat(handler h, DataCallback cb);
void    RegisterOnVoice(handler h, DataCallback cb);
void    RegisterOnVideo(handler h, DataCallback cb);

// Fetching
char**  FetchOnline(handler h, size_t* count);
char**  FetchSessions(handler h, const char* id, size_t* count);

typedef void (*DataCallback)(const char* id, void* data, size_t len);
```

---

## Docker

### Build and Run

```bash
# Build the Docker image
make docker-build

# Run a single instance (user-000)
make docker-run-app

# Run a designated instance
make docker-run-app-456

# Start a 10-node test mesh
docker compose -f docker-compose.yaml up -d --build

# Stop all containers
make docker-down
```

---

## Build from Source

```bash
# Build standalone binary
go build -o main.exe cmd/app/main.go

# Run locally
go run cmd/app/main.go

# Run tests
go test ./... -v

# Build C-Archive via CMake
cmake -B build .
cmake --build build
```

---

## Testing

CI Pipeline stages (`.gitlab-ci.yml`):

| Stage | Description |
| :--- | :--- |
| `build` | Compiles Docker test images |
| `test` | Automated unit and integration testing suite |
| `deploy` | Multi-node deployment via Docker Compose |

```bash
# Local tests with coverage
go test ./... -v -cover

# Run tests with race condition detector
go test -race ./...
```

---

## License

This project is licensed under the [MIT License](LICENSE).

---

## Related Projects

- **[aloh-signalling](https://github.com/kiryuhakipyatok/aloh-signalling)** — QUIC signaling server for peer discovery and session negotiation.

---

## Acknowledgments

- [Pion](https://github.com/pion) — WebRTC & ICE implementations in pure Go
- [quic-go](https://github.com/quic-go/quic-go) — QUIC implementation in Go
- [cenkalti/backoff](https://github.com/cenkalti/backoff) — Exponential backoff algorithm
- [Go Cryptography](https://golang.org/x/crypto) — HKDF & cryptographic primitives