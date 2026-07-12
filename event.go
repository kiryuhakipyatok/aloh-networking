package alohnetwork

import "github.com/kiryuhakipyatok/aloh-networking/internal/domain/services/networking"

const (
	FULL_MUTE    = networking.FULL_MUTE
	MIC_MUTE     = networking.MIC_MUTE
	HARD_DENOISE = networking.HARD_DENOISE
	SOFT_DENOISE = networking.SOFT_DENOISE
)

type Event = networking.Event
