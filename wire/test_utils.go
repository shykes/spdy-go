package wire

import (
	"time"
	"code.google.com/p/go.net/spdy"
)

type TestSession struct {
	*Session
	pipe *Pipe
}


type Pipe struct {
	Input	*ChanFramer
	Output	*ChanFramer
}

func (pipe *Pipe) ReadFrame() (spdy.Frame, error) {
	return pipe.Input.ReadFrame()
}

func (pipe *Pipe) WriteFrame(frame spdy.Frame) error {
	return pipe.Output.WriteFrame(frame)
}


func NewPipe() *Pipe {
	return &Pipe{NewChanFramer(), NewChanFramer()}
}


func NewTestSession(h func(*Stream), server bool) *TestSession {
	var handler Handler
	if h == nil {
		handler = &DummyHandler{}
	} else {
		h := HandlerFunc(h)
		handler = &h
	}
	pipe := NewPipe()
	return &TestSession{
		Session:	NewSession(pipe, handler, server),
		pipe:		pipe,
	}
}


func Timeout(d time.Duration) chan bool {
	timeout := make(chan bool)
	go func() {
		timeout <- true
		debug("Sleeping %v", d)
		time.Sleep(d)
		debug("Done sleeping")
		timeout <- true
	}()
	return timeout
}
