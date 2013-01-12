package spdy

import (
	"testing"
)


func TestSynStreamTooHigh(t *testing.T) {
	s := NewTestSession(nil, true)
	if err := s.pipe.WriteFrame(&SynStreamFrame{StreamId: 3}); err != nil {
		t.Error(err)
	}
	frame, err := s.pipe.ReadFrame()
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

//    Stream IDs do not
//    wrap: when a client or server cannot create a new stream id without
//    exceeding a 31 bit value, it MUST NOT create a new stream.

func TestIdWrap(t *testing.T) {
	for _, isServer := range []bool{true, false} {
		s := NewSession(new(DummyHandler), isServer)
		if isServer {
			s.lastStreamIdOut = 0xffffffff
		} else {
			s.lastStreamIdOut = 0xffffffff - 1
		}
		_, err := s.OpenStream()
		if err == nil {
			t.Error("[...] when a client or server cannot create a new stream id without exceeding a 31 bit value, it MUST NOT create a new stream")
		}
		if s.NStreams() != 0 {
			t.Error("[...] when a client or server cannot create a new stream id without exceeding a 31 bit value, it MUST NOT create a new stream")

		}
	}
}

func TestSynStreamInvalidId(t *testing.T) {
	s := NewTestSession(nil, true)
	if err := s.pipe.WriteFrame(&SynStreamFrame{StreamId: 2}); err != nil {
		t.Error(err)
	}
	frame, err := s.pipe.ReadFrame()
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
	session := NewTestSession(nil, false)
	session.OpenStream()
	session.CloseStream(1)
	if session.NStreams() != 0 {
		t.Errorf("CloseStream() did not delete the stream")
	}
}
