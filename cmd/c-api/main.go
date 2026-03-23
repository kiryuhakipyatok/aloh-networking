package main

/*
#include <stdlib.h>
#include <stdint.h>

typedef const char cchar_t;

typedef uintptr_t handler;

typedef void (*DataCallback)(cchar_t* id, void* data, size_t len);

static void call_callback(DataCallback f, cchar_t* id, void* data, size_t len) {
    if (f) {
        f(id, data, len);
    }
}
*/
import "C"

import (
	"context"
	"errors"
	"runtime/cgo"
	"strings"
	"unsafe"

	"github.com/kiryuhakipyatok/aloh-networking/internal/app"
	"github.com/kiryuhakipyatok/aloh-networking/internal/handlers"
)

type Wrapper struct {
	Handler *handlers.NetworkingHandler
	Cancel  context.CancelFunc
}

//export NewHandler
func NewHandler(userID *C.cchar_t, logPath *C.cchar_t) C.handler {
	goUserID := C.GoString((*C.char)(userID))
	goLogPath := C.GoString((*C.char)(logPath))

	service, cancel, cfg := app.Init(goUserID, goLogPath)

	nh := handlers.NewNetworkingHandler(service, cfg)

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
	receivers := strings.Split(goIdsStr, ",")
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
	if cb == nil {
		return
	}
	wr := getWrapper(h)
	wr.Handler.OnChat(func(id string, data []byte) {
		if len(data) == 0 {
			return
		}
		cData := unsafe.Pointer(&data[0])
		cLen := C.size_t(len(data))
		cId := C.CString(id)
		defer C.free(unsafe.Pointer(cId))
		C.call_callback(cb, cId, cData, cLen)
	})
}

//export RegisterOnVoice
func RegisterOnVoice(h C.handler, cb C.DataCallback) {
	if cb == nil {
		return
	}
	wr := getWrapper(h)
	wr.Handler.OnVoice(func(id string, data []byte) {
		if len(data) == 0 {
			return
		}
		cData := unsafe.Pointer(&data[0])
		cLen := C.size_t(len(data))
		cId := C.CString(id)
		defer C.free(unsafe.Pointer(cId))
		C.call_callback(cb, cId, cData, cLen)
	})
}

//export RegisterOnVideo
func RegisterOnVideo(h C.handler, cb C.DataCallback) {
	if cb == nil {
		return
	}
	wr := getWrapper(h)
	wr.Handler.OnVideo(func(id string, data []byte) {
		if len(data) == 0 {
			return
		}
		cData := unsafe.Pointer(&data[0])
		cLen := C.size_t(len(data))
		cId := C.CString(id)
		defer C.free(unsafe.Pointer(cId))
		C.call_callback(cb, cId, cData, cLen)
	})
}

//export FetchOnline
func FetchOnline(h C.handler) (**C.char, C.size_t, C.uint) {
	wr := getWrapper(h)
	online, err := wr.Handler.FetchOnline()
	if err != nil {
		return nil, C.size_t(0), proccessError(err)
	}

	l := len(online)
	ptrSize := unsafe.Sizeof((*C.char)(nil))
	cArrayPtr := C.malloc(C.size_t(l) * C.size_t(ptrSize))
	cArraySlice := unsafe.Slice((**C.char)(cArrayPtr), l)
	for i, s := range online {
		cArraySlice[i] = C.CString(s)
	}
	return (**C.char)(cArrayPtr), C.size_t(l), C.uint(handlers.SUCCESS)
}

//export FetchSessions
func FetchSessions(h C.handler, id *C.cchar_t) (**C.char, C.size_t, C.uint) {
	wr := getWrapper(h)
	goID := C.GoString((*C.char)(id))
	sessions, err := wr.Handler.FetchSessionById(goID)
	if err != nil {
		return nil, C.size_t(0), proccessError(err)
	}

	l := len(sessions)
	ptrSize := unsafe.Sizeof((*C.char)(nil))
	cArrayPtr := C.malloc(C.size_t(l) * C.size_t(ptrSize))
	cArraySlice := unsafe.Slice((**C.char)(cArrayPtr), l)
	for i, s := range sessions {
		cArraySlice[i] = C.CString(s)
	}
	return (**C.char)(cArrayPtr), C.size_t(l), C.uint(handlers.SUCCESS)
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
