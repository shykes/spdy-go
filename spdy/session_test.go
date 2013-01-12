package spdy

import (
	"testing"
	"reflect"
	"errors"
	"fmt"
)



// [...] If the server is initiating the stream, the Stream-ID must be even. [...]

func TestServerStreamIdMustBeEven(t *testing.T) {
    s := NewSession(new(DummyHandler), true)
	for i:=0; i<42; i+=1 {
	    stream, err := s.OpenStream()
	 if err != nil {
	    t.Error(err)
	}
	if stream.Id % 2 != 0 {
	    t.Errorf("If the server is initiating the stream, the Stream-ID must be even.")
	}
    }
}

// [...] If the client is initiating the stream, the Stream-ID must be odd. [...]

func TestClientStreamIdMustBeOdd(t *testing.T) {
    s := NewSession(new(DummyHandler), false)
	for i:=0; i<42; i+=1 {
	    stream, err := s.OpenStream()
	 if err != nil {
	    t.Error(err)
	}
	if stream.Id % 2 != 1 {
	    t.Errorf("If the client is initiating the stream, the Stream-ID must be odd.")
	}
    }
}

// [...] 0 is not a valid Stream-ID. [...]

func TestStreamIDZeroNotValid(t *testing.T) {
    for _, isServer := range []bool{true, false} {
	s := NewSession(new (DummyHandler), isServer)
	    stream, err := s.OpenStream()
	    if err != nil {
		t.Error(err)
	    }
	    if stream.Id == 0 {
		t.Errorf("0 is not a valid Stream-ID.")
	    }
	    if err := s.WriteFrame(&SynStreamFrame{StreamId: 0}); err != nil {
		t.Error(err)
	    }
	    // Write a ping frame so there's always at least one frame
	    // to read
	    if err := s.WriteFrame(&PingFrame{}); err != nil {
		t.Error(err)
	    }
	    frame, err := s.ReadFrame()
	    if rstFrame, isRst := frame.(*RstStreamFrame); !isRst {
		t.Errorf("0 is not a valid Stream-ID.")
	    } else {
		if rstFrame.Status != ProtocolError {
		    t.Errorf("0 is not a valid Stream-ID.")
		}
		if rstFrame.StreamId != 0 {
		    t.Errorf("0 is not a valid Stream-ID.")
		}
	    }
	}
}

// [...] Stream-IDs from each side of the connection must increase monotonically as new
// streams are created [...]

func TestStreamIDIncrementLocal(t *testing.T) {
	for _, isServer := range []bool{true, false} {
		s := NewSession(new(DummyHandler), isServer)
		var previousId uint32
		for i:=0; i<42; i+=1 {
			stream, err := s.OpenStream()
			if err != nil {
				t.Error(err)
			}
			if previousId != 0 && stream.Id != previousId + 2 {
				t.Error("Stream-IDs from each side of the connection must increase monotically as new streams are created")
			}
		}
	}
}

// [...] E.g. Stream 2 may be created after stream 3, [...]

func TestStream2AfterStream3(t *testing.T) {
	s := NewSession(new(DummyHandler), false)
	s.OpenStream()
	stream3, err := s.OpenStream(); if err != nil {
		t.Error(err)
	} else {
		if stream3.Id != 3 {
			t.Error("Second client-initiated stream should have ID=3")
		}
	}
	if _, err := SendExpect(s, &SynStreamFrame{StreamId: 2}, nil); err != nil {
		t.Error(err)
	}
}	

//    but stream 7 must not be created after stream 9.

func TestStream7AfterStream9(t *testing.T) {
	s := NewSession(new(DummyHandler), true)
	if _, err := SendExpect(s, &SynStreamFrame{StreamId:9}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SendExpect(s, &SynStreamFrame{StreamId:7}, reflect.TypeOf(&RstStreamFrame{})); err != nil {
		t.Fatal(err)
	}
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

func SendExpect(s *Session, frameIn Frame, frameTypeOut reflect.Type) (Frame, error) {
	if err := s.WriteFrame(&PingFrame{Id:42}); err != nil {
		return nil, err
	}
	if err := s.WriteFrame(frameIn); err != nil {
		return nil, err
	}
	frame, err := s.ReadFrame()
	if err != nil {
		return nil, err
	}
	var receivedNothing bool
	switch ff := frame.(type) {
		case *PingFrame: {
			if ff.Id == 42 {
				receivedNothing = true
			}
		}
	}
	if receivedNothing {
		if frameTypeOut != nil {
			// Expected something but received nothing", frameTypeOut)
		}
	} else {
		if frameTypeOut == nil {
			// Expected nothing but received something
			return nil, errors.New(fmt.Sprintf("Expected nothing but received %#v", frame))
		} else if frameTypeOut != reflect.TypeOf(frame) {
			return nil, errors.New("Received wrong frame type")
		}
	}
	if frameTypeOut != nil {
		if _, err := s.ReadFrame(); err != nil {
			return nil, err
		}
	}
	return frame, nil
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
