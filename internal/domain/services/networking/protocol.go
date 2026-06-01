package networking

const (
	FULL_MUTE = iota
	MIC_MUTE
)

type Event struct {
	Typee     uint
	State     bool
	Timestamp int64
}