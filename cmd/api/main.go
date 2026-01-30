package main

/*
#include <stdlib.h>
#include <stdint.h>

// Определение типа callback
typedef void (*DataCallback)(void* data, int len);

// Хелпер для вызова callback из Go
static void call_callback(DataCallback f, void* data, int len) {
    if (f) {
        f(data, len);
    }
}
*/
import "C"

import (
	"context"
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
func NewHandler(userID *C.char, configPath *C.char) C.uintptr_t {
	goUserID := C.GoString(userID)

	goConfigPath := ""
	if configPath != nil {
		goConfigPath = C.GoString(configPath)
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
func DeleteHandler(h C.uintptr_t) {
	handle := cgo.Handle(h)
	wr := handle.Value().(*Wrapper)

	if wr.Cancel != nil {
		wr.Cancel()
	}

	handle.Delete()
}

func getWrapper(h C.uintptr_t) *Wrapper {
	return cgo.Handle(h).Value().(*Wrapper)
}

//export Connect
func Connect(h C.uintptr_t, idsStr *C.char) *C.char {
	wr := getWrapper(h)
	goIdsStr := C.GoString(idsStr)
	receivers := strings.Split(goIdsStr, ",")

	if err := wr.Handler.Connect(receivers); err != nil {
		return C.CString(err.Error())
	}
	return nil
}

//export Disconnect
func Disconnect(h C.uintptr_t) *C.char {
	wr := getWrapper(h)
	if err := wr.Handler.Disconnect(); err != nil {
		return C.CString(err.Error())
	}
	return nil
}

//export SendMessage
func SendMessage(h C.uintptr_t, msg *C.char) *C.char {
	wr := getWrapper(h)
	goMsg := C.GoString(msg)

	if err := wr.Handler.SendMessage(goMsg); err != nil {
		return C.CString(err.Error())
	}
	return nil
}

//export SendVoice
func SendVoice(h C.uintptr_t, data unsafe.Pointer, length C.int) *C.char {
	wr := getWrapper(h)
	goData := C.GoBytes(data, length)
	if err := wr.Handler.SendVoice(goData); err != nil {
		return C.CString(err.Error())
	}
	return nil
}

//export SendVideo
func SendVideo(h C.uintptr_t, data unsafe.Pointer, length C.int) *C.char {
	wr := getWrapper(h)
	goData := C.GoBytes(data, length)
	if err := wr.Handler.SendVideo(goData); err != nil {
		return C.CString(err.Error())
	}
	return nil
}

//export RegisterOnChat
func RegisterOnChat(h C.uintptr_t, cb C.DataCallback) {
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
func RegisterOnVoice(h C.uintptr_t, cb C.DataCallback) {
	wr := getWrapper(h)
	wr.Handler.OnVoice(func(data []byte) {
		if len(data) == 0 {
			return
		}
		C.call_callback(cb, unsafe.Pointer(&data[0]), C.int(len(data)))
	})
}

//export RegisterOnVideo
func RegisterOnVideo(h C.uintptr_t, cb C.DataCallback) {
	wr := getWrapper(h)
	wr.Handler.OnVideo(func(data []byte) {
		if len(data) == 0 {
			return
		}
		C.call_callback(cb, unsafe.Pointer(&data[0]), C.int(len(data)))
	})
}

func main() {}
