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
	State     bool
	Timestamp int64
}
