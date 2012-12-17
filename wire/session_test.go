package wire

import (
	"testing"
	"code.google.com/p/go.net/spdy"
)

func TestSynCallsHandler(t *testing.T) {
	ch := make(chan uint32)
	s := NewTestSession(func(stream *Stream) {
		debug("Handler!")
		ch <- stream.Id
	}, true)
	s.processFrame(&spdy.SynStreamFrame{StreamId: 1})
	if <-ch != 1 {
		t.Error("handler was not called")
	}
}

func TestSynStreamTooHigh(t *testing.T) {
	s := NewTestSession(nil, true)
	s.processFrame(&spdy.SynStreamFrame{StreamId: 3})
	s.pipe.Output.Close()
	frame, err := s.pipe.Output.ReadFrame()
	if err != nil {
		t.Error("Session didn't send protocol error on invalid stream id")
		return
	}
	if f, isRst := frame.(*spdy.RstStreamFrame); isRst {
		if f.Status != spdy.ProtocolError {
			t.Error("Session didn't send protocol error on invalid stream id")
		}
	} else {
		t.Error("Session didn't send protocol error on invalid stream id")
	}

}

func TestSynStreamInvalidId(t *testing.T) {
	s := NewTestSession(nil, true)
	s.processFrame(&spdy.SynStreamFrame{StreamId: 2})
	s.pipe.Output.Close()
	frame, err := s.pipe.Output.ReadFrame()
	if err != nil {
		t.Error("Session didn't send protocol error on invalid stream id")
		return
	}
	if f, isRst := frame.(*spdy.RstStreamFrame); isRst {
		if f.Status != spdy.ProtocolError {
			t.Error("Session didn't send protocol error on invalid stream id")
		}
	} else {
		t.Error("Session didn't send protocol error on invalid stream id")
	}

}
