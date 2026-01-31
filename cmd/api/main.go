package main

/*
#include <stdlib.h>
#include <stdint.h>

typedef const char cchar_t;

typedef uintptr_t handler;

typedef void (*DataCallback)(void* data, size_t len);

static void call_callback(DataCallback f, void* data, int len) {
    if (f) {
        f(data, len);
    }
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"runtime/cgo"
	"strings"
	"unsafe"

	"networking/internal/app"
	"networking/internal/handlers"
)

type Wrapper struct {
	Handler *handlers.NetworkingHandler
	Cancel  context.CancelFunc
}

//export NewHandler
func NewHandler(userID *C.cchar_t, configPath *C.cchar_t) C.handler {
	goUserID := C.GoString((*C.char)(userID))

	goConfigPath := ""
	if configPath != nil {
		goConfigPath = C.GoString((*C.char)(configPath))
	}

	service, cancel := app.Init(goConfigPath, goUserID)

	nh := handlers.NewNetworkingHandler(service)

	wr := &Wrapper{
		Handler: nh,
		Cancel:  cancel,
	}

	return C.uintptr_t(cgo.NewHandle(wr))
}

//export DeleteHandler
func DeleteHandler(h C.handler) {
	handle := cgo.Handle(h)
	wr := handle.Value().(*Wrapper)

	if wr.Cancel != nil {
		wr.Cancel()
	}

	handle.Delete()
}

func getWrapper(h C.handler) *Wrapper {
	return cgo.Handle(h).Value().(*Wrapper)
}

//export Connect
func Connect(h C.handler, idsStr *C.cchar_t) C.uint {
	wr := getWrapper(h)

	goIdsStr := C.GoString((*C.char)(idsStr))
	fmt.Println("goIdsStr:", goIdsStr)
	receivers := strings.Split(goIdsStr, ",")
	fmt.Println("receivers:", receivers)
	if err := wr.Handler.Connect(receivers); err != nil {
		return proccessError(err)
	}
	return C.uint(handlers.SUCCESS)
}

//export Disconnect
func Disconnect(h C.handler) C.uint {
	wr := getWrapper(h)
	if err := wr.Handler.Disconnect(); err != nil {
		return proccessError(err)
	}
	return C.uint(handlers.SUCCESS)
}

//export SendMessage
func SendMessage(h C.handler, msg *C.cchar_t) C.uint {
	wr := getWrapper(h)
	goMsg := C.GoString((*C.char)(msg))

	if err := wr.Handler.SendMessage(goMsg); err != nil {
		return proccessError(err)
	}
	return C.uint(handlers.SUCCESS)
}

//export SendVoice
func SendVoice(h C.handler, data unsafe.Pointer, length C.int) C.uint {
	wr := getWrapper(h)
	goData := C.GoBytes(data, length)
	if err := wr.Handler.SendVoice(goData); err != nil {
		return proccessError(err)
	}
	return C.uint(handlers.SUCCESS)
}

//export SendVideo
func SendVideo(h C.handler, data unsafe.Pointer, length C.int) C.uint {
	wr := getWrapper(h)
	goData := C.GoBytes(data, length)
	if err := wr.Handler.SendVideo(goData); err != nil {
		return proccessError(err)
	}
	return C.uint(handlers.SUCCESS)
}

//export RegisterOnChat
func RegisterOnChat(h C.handler, cb C.DataCallback) {
	wr := getWrapper(h)
	wr.Handler.OnChat(func(data []byte) {
		if len(data) == 0 {
			return
		}
		cData := unsafe.Pointer(&data[0])
		cLen := C.int(len(data))
		C.call_callback(cb, cData, cLen)
	})
}

//export RegisterOnVoice
func RegisterOnVoice(h C.handler, cb C.DataCallback) {
	wr := getWrapper(h)
	wr.Handler.OnVoice(func(data []byte) {
		if len(data) == 0 {
			return
		}
		C.call_callback(cb, unsafe.Pointer(&data[0]), C.int(len(data)))
	})
}

//export RegisterOnVideo
func RegisterOnVideo(h C.handler, cb C.DataCallback) {
	wr := getWrapper(h)
	wr.Handler.OnVideo(func(data []byte) {
		if len(data) == 0 {
			return
		}
		C.call_callback(cb, unsafe.Pointer(&data[0]), C.int(len(data)))
	})
}

func proccessError(err error) C.uint {
	var (
		ae   handlers.ErrorCode
		code uint = handlers.INTERNAL_ERROR
	)
	if errors.As(err, &ae) {
		code = ae.Code
	}
	return C.uint(code)
}

func main() {}
