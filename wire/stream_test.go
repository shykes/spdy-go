package wire

import (
	"net/http"
	"testing"
	"code.google.com/p/go.net/spdy"
)

func TestHeadersCrash(t *testing.T) {
	stream := NewStream(1, false)
	headers := http.Header{}
	headers.Add("foo", "bar")
	err := stream.Input.WriteFrame(&spdy.SynStreamFrame{
		StreamId:	1,
		Headers:	headers,
	})
	if err != nil {
		t.Error(err)
	}
	if stream.Input.Headers.Get("foo") != "bar" {
		t.Errorf("Stream input didn't store headers (%v != %v)", stream.Input.Headers, headers)
	}
}

func TestLastFrameReceived(t *testing.T) {
	stream := NewStream(1, false)
	headers := http.Header{}
	headers.Add("foo", "bar")
	err := stream.Input.WriteFrame(&spdy.SynStreamFrame{
		StreamId:	1,
		Headers:	headers,
		CFHeader:	spdy.ControlFrameHeader{Flags: spdy.ControlFlagFin},
	})
	if err != nil {
		t.Error(err)
	}
	frame, err := stream.Input.ReadFrame()
	if err != nil {
		t.Errorf("Frame with FLAG_FIN=1 was dropped by stream input: %s", err)
	} else if headers := FrameHeaders(frame); headers != nil {
		if headers.Get("foo") != "bar" {
			t.Errorf("Frame with FLAG_FIN=1 was not passed intact by stream input")
		}
	} else {
		t.Errorf("Frame with FLAG_FIN=1 was not passed intact by stream input")
	}
}
