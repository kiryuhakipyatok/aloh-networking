package mediatransfer

import (
	"gocv.io/x/gocv"
)

type MediatransferService interface {
}

type mediatransferService struct {
	webcam *gocv.VideoCapture
}

func NewMediatransferService() MediatransferService {
	webcam, err := gocv.VideoCaptureDevice(0)
	if err != nil {
		panic(err)
	}
	return &mediatransferService{
		webcam: webcam,
	}
}

func (ms *mediatransferService) SendWebcamData() {
	window := gocv.NewWindow("Webcam & Audio")
	defer window.Close()
	img := gocv.NewMat()
	for {
		ms.webcam.Read(&img)
		if img.Empty() {
			continue
		}
		window.IMShow(img)
		if window.WaitKey(1) >= 0 {
			break
		}
	}
}

func (ms *mediatransferService) ReceiveWebcamData() {

}
