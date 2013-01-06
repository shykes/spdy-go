package spdy

import (
	"time"
	"net/http"
)

type TestSession struct {
	*Session
	pipe *Pipe
}


type Pipe struct {
	Input	*ChanFramer
	Output	*ChanFramer
}

func (pipe *Pipe) ReadFrame() (Frame, error) {
	return pipe.Input.ReadFrame()
}

func (pipe *Pipe) WriteFrame(frame Frame) error {
	return pipe.Output.WriteFrame(frame)
}


func NewPipe() *Pipe {
	return &Pipe{NewChanFramer(), NewChanFramer()}
}


func NewTestSession(h http.HandlerFunc, server bool) *TestSession {
	var handler Handler
	if h == nil {
		handler = &DummyHandler{}
	} else {
		h := http.HandlerFunc(h)
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
