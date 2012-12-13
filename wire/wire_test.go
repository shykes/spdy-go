package wire

import (
	"io"
	"time"
	"testing"
	"code.google.com/p/go.net/spdy"
)

func TestDummy(t *testing.T) {
	// Do nothing
}


/*
** ChanFramer
*/

func TestChanFramerSendOneFrame(t *testing.T) {
	framer := NewChanFramer()
	f_in := spdy.NoopFrame{}
	framer.WriteFrame(&f_in)
	f_out, _ := framer.ReadFrame()
	if f_in != *f_out.(*spdy.NoopFrame) {
		t.Errorf("Sent %v, received %v\n", f_in, f_out)
	}
}


/*
** 
*/


func TestSessionSynStreamCallsHandler(t *testing.T) {
	framer := NewChanFramer()
	ch := make(chan bool)
	handler := HandlerFunc(func(stream *Stream) {
		if stream.Id != 1 {
			t.Errorf("Wrong stream ID: %d\n", stream.Id)
		}
		ch <- true
	})
	NewSession(framer, &handler, true)
	framer.WriteFrame(&spdy.SynStreamFrame{StreamId: 1})
	_, ok := <-ch // FIXME: in case of a bug, the test will hang instead of failing
	if !ok {
		t.Error("Stream was not created\n")
	}
}

func TestSessionOpenStreamCallsHandler(t *testing.T) {
	framer := NewChanFramer()
	ch := make(chan bool)
	handler := HandlerFunc(func(stream *Stream) {
		if stream.Id != 2 {
			t.Errorf("Wrong stream ID: %d\n", stream.Id)
		}
		ch <- true
	})
	session := NewSession(framer, &handler, true)
	session.OpenStream()
	framer.WriteFrame(&spdy.NoopFrame{})
	timeout := make(chan bool)
	go func() {
		timeout <- true
		time.Sleep(3 * time.Second)
		timeout <- true
	}()
	<-timeout
	select {
		case _, ok := <-ch: {
			if !ok {
				t.Error("Stream handler was not called after OpenStream")
			}
		}
		case <-timeout: {
			t.Error("Stream handler was not called after OpenStream")
		}
	}
}


func NewTestSession(server bool) *Session {
	return NewSession(NewChanFramer(), &DummyHandler{}, server)

}

func TestSessionOpenStreamServer(t *testing.T) {
	session := NewTestSession(true)
	stream, err := session.OpenStream()
	if err != nil {
		t.Error(err)
	}
	if stream.Id != 2 {
		t.Errorf("First server-created stream should be 2 (not %d)", stream.Id)
	}
}

func TestSessionOpenStreamClient(t *testing.T) {
	session := NewTestSession(false)
	stream, err := session.OpenStream()
	if err != nil {
		t.Error(err)
	}
	if stream.Id != 1 {
		t.Errorf("First client-created stream should be 1 (not %d)", stream.Id)
	}
}


func TestNStreams(t *testing.T) {
	session := NewTestSession(true)
	if session.NStreams() != 0 {
		t.Errorf("NStreams() for empty session should be 0 (not %d)", session.NStreams())
	}
	session.OpenStream()
	if session.NStreams() != 1 {
		t.Errorf("NStreams() should be 1 (not %d)", session.NStreams())
	}
}

func TestCloseStream(t *testing.T) {
	handler := HandlerFunc(func(s *Stream) {
			_, err := s.Input.ReadFrame()
			if err != io.EOF {
				t.Errorf("CloseStream() did not pass io.EOF")
			}
		})
	session := NewSession(
		NewChanFramer(),
		&handler,
		false)
	session.OpenStream()
	session.CloseStream(1)
	if session.NStreams() != 0 {
		t.Errorf("CloseStream() did not delete the stream")
	}
}
