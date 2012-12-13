package wire

import (
	"os"
	"log"
	"io"
)

/*
** Run `f` in a new goroutine and return a channel which will receive
** its return value
 */

func Promise(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

/*
** Output a message only if the DEBUG env variable is set
 */

var DEBUG bool = false

func debug(msg string, args ...interface{}) {
	if DEBUG || (os.Getenv("DEBUG") != "") {
		log.Printf(msg, args...)
	}
}


type HandlerFunc func(*Stream)

func (f *HandlerFunc) ServeSPDY(s *Stream) {
	(*f)(s)
}



type DummyHandler struct {}

func (f *DummyHandler) ServeSPDY(s *Stream) {
	for {
		_, err := s.Input.ReadFrame()
		if err != nil {
			return
		}
	}
}


func Copy(w FrameWriter, r FrameReader) error {
	for {
		frame, err := r.ReadFrame()
		if err != nil {
			return err
		}
		err = w.WriteFrame(frame)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}
