package alohnetwork

import (
	"encoding/json"
	"time"

	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/services/networking"
)

const (
	FULL_MUTE    = networking.FULL_MUTE
	MIC_MUTE     = networking.MIC_MUTE
	HARD_DENOISE = networking.HARD_DENOISE
	SOFT_DENOISE = networking.SOFT_DENOISE
	GENERAL      = networking.GENERAL
)

type (
	Event       = networking.Event
	GeneralData = networking.GeneralData
)

func MuteMicEvent(state bool) (Event, error) {
	t := time.Now().UTC().Unix()
	e := Event{
		Typee:     MIC_MUTE,
		Timestamp: t,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return e, err
	}
	e.Data = data
	return e, nil
}

func MuteFullEvent(state bool) (Event, error) {
	t := time.Now().UTC().Unix()
	e := Event{
		Typee:     FULL_MUTE,
		Timestamp: t,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return e, err
	}
	e.Data = data
	return e, nil
}

func HardDenoiseEvent(state bool) (Event, error) {
	t := time.Now().UTC().Unix()
	e := Event{
		Typee:     HARD_DENOISE,
		Timestamp: t,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return e, err
	}
	e.Data = data
	return e, nil
}

func SoftDenoiseEvent(state bool) (Event, error) {
	t := time.Now().UTC().Unix()
	e := Event{
		Typee:     SOFT_DENOISE,
		Timestamp: t,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return e, err
	}
	e.Data = data
	return e, nil
}

func GeneralEvent(fm, mm, hd, sd bool) (Event, error) {
	t := time.Now().UTC().Unix()
	e := Event{
		Typee:     GENERAL,
		Timestamp: t,
	}
	gd := GeneralData{
		FullMute:    fm,
		MicMute:     mm,
		HardDenoise: hd,
		SoftDenoise: sd,
	}
	data, err := json.Marshal(gd)
	if err != nil {
		return e, err
	}
	e.Data = data
	return e, nil
}

func DataToState(data []byte) (bool, error) {
	var state bool
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func DataToGeneral(data []byte) (GeneralData, error) {
	var general GeneralData
	if err := json.Unmarshal(data, &general); err != nil {
		return general, err
	}
	return general, nil
}