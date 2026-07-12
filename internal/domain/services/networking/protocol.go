package networking

const (
	FULL_MUTE = iota
	MIC_MUTE
	HARD_DENOISE
	SOFT_DENOISE
	GENERAL
)

type Event struct {
	Typee     uint
	Data      []byte
	Timestamp int64
}

type GeneralData struct {
	FullMute    bool `json:"full-mute"`
	MicMute     bool `json:"mic-mute"`
	HardDenoise bool `json:"hard-denoise"`
	SoftDenoise bool `json:"soft-denoise"`
}