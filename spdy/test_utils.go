package spdy

import (
	"time"
	"net/http"
)

type TestSession struct {
	*Session
	pipe *Socket
}


func NewTestSession(h http.HandlerFunc, server bool) *TestSession {
	var handler Handler
	if h == nil {
		handler = &DummyHandler{}
	} else {
		h := http.HandlerFunc(h)
		handler = &h
	}
	session := NewSession(handler, server)
	inputR, inputW := Pipe(0)
	outputR, outputW := Pipe(0)
	go Splice(session, &Socket{inputR, outputW}, true)
	return &TestSession{
		Session:	NewSession(handler, server),
		pipe:		&Socket{outputR, inputW},
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
