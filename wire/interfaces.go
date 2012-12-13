package wire

import (
	"code.google.com/p/go.net/spdy"
)

type Handler interface {
	ServeSPDY(*Stream)
}

type FrameReader interface {
	ReadFrame() (spdy.Frame, error)
}

type FrameWriter interface {
	WriteFrame(spdy.Frame) error
}

type Framer interface {
	ReadFrame() (spdy.Frame, error)
	WriteFrame(spdy.Frame) error
}

