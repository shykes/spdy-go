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
