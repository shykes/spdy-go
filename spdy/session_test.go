package spdy

import (
	"testing"
	"io"
	"time"
)

func TestSynCallsHandler(t *testing.T) {
	ch := make(chan uint32)
	s := NewTestSession(func(stream *Stream) {
		debug("Handler!")
		ch <- stream.Id
	}, true)
	s.processFrame(&SynStreamFrame{StreamId: 1})
	if <-ch != 1 {
		t.Error("handler was not called")
	}
}

func TestSynStreamTooHigh(t *testing.T) {
	s := NewTestSession(nil, true)
	s.processFrame(&SynStreamFrame{StreamId: 3})
	s.pipe.Output.Close()
	frame, err := s.pipe.Output.ReadFrame()
	if err != nil {
		t.Error("Session didn't send protocol error on invalid stream id")
		return
	}
	if f, isRst := frame.(*RstStreamFrame); isRst {
		if f.Status != ProtocolError {
			t.Error("Session didn't send protocol error on invalid stream id")
		}
	} else {
		t.Error("Session didn't send protocol error on invalid stream id")
	}

}

func TestSynStreamInvalidId(t *testing.T) {
	s := NewTestSession(nil, true)
	s.processFrame(&SynStreamFrame{StreamId: 2})
	s.pipe.Output.Close()
	frame, err := s.pipe.Output.ReadFrame()
	if err != nil {
		t.Error("Session didn't send protocol error on invalid stream id")
		return
	}
	if f, isRst := frame.(*RstStreamFrame); isRst {
		if f.Status != ProtocolError {
			t.Error("Session didn't send protocol error on invalid stream id")
		}
	} else {
		t.Error("Session didn't send protocol error on invalid stream id")
	}

}

/*
** ChanFramer
*/

func TestChanFramerSendOneFrame(t *testing.T) {
	framer := NewChanFramer()
	f_in := NoopFrame{}
	framer.WriteFrame(&f_in)
	f_out, _ := framer.ReadFrame()
	if f_in != *f_out.(*NoopFrame) {
		t.Errorf("Sent %v, received %v\n", f_in, f_out)
	}
}


/*
** 
*/

func _TestSessionSynStreamCallsHandlerTemp(t *testing.T) {
	framer := NewChanFramer()
	ch := make(chan bool)
	handler := HandlerFunc(func(stream *Stream) {
		if stream.Id != 1 {
			t.Errorf("Wrong stream ID: %d\n", stream.Id)
		}
		ch <- true
	})
	NewSession(framer, &handler, true)
	framer.WriteFrame(&SynStreamFrame{StreamId: 1})
	debug("Wrote frame")
	timeout := Timeout(1 * time.Second)
	debug("Waiting for timeout to start")
	<-timeout
	debug("timeout has started")
	select {
		case _, ok := <-ch: {
			if !ok {
				t.Error("Stream was not created\n")
			}
		}
		case <-timeout: {
			debug("Timeout ended")
			t.Error("Stream was not created\n")
		}
	}
	debug("???")
}


func TestSessionOpenStreamServer(t *testing.T) {
	session := NewTestSession(nil, true)
	stream, err := session.OpenStream()
	if err != nil {
		t.Error(err)
	}
	if stream.Id != 2 {
		t.Errorf("First server-created stream should be 2 (not %d)", stream.Id)
	}
}

func TestSessionOpenStreamClient(t *testing.T) {
	session := NewTestSession(nil, false)
	stream, err := session.OpenStream()
	if err != nil {
		t.Error(err)
	}
	if stream.Id != 1 {
		t.Errorf("First client-created stream should be 1 (not %d)", stream.Id)
	}
}


func TestNStreams(t *testing.T) {
	session := NewTestSession(nil, true)
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
