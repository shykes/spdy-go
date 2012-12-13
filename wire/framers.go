package wire

import (
	"code.google.com/p/go.net/spdy"
	"io"
)


/*
** A ChanFramer allows 2 goroutines to send SPDY frames to each other
** using the Framer interface.
**
** Frames are sent through a buffered channel of hardcoded size (currently 4096).
*/

type ChanFramer struct {
	ch	chan spdy.Frame
	err	error
}

func NewChanFramer() *ChanFramer {
	return &ChanFramer{
		ch:	make(chan spdy.Frame, 4096),
	}
}

func (framer *ChanFramer) WriteFrame(frame spdy.Frame) error {
	if framer.err != nil {
		return framer.err
	}
	framer.ch <- frame
	return nil
}

func (framer *ChanFramer) ReadFrame() (spdy.Frame, error) {
	/* This will not block if the channel is closed and empty */
	frame, ok := <-framer.ch
	if !ok {
		return nil, framer.err
	}
	return frame, nil
}

func (framer *ChanFramer) Error(err error) {
	if framer.err != nil {
		return
	}
	framer.err = err
	close(framer.ch)
}

func (framer *ChanFramer) Close() {
	framer.Error(io.EOF)
}

func (framer *ChanFramer) Closed() bool {
	return framer.err != nil
}
