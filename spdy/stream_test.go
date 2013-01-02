package spdy

import (
	"net/http"
	"testing"
)

func TestHeadersCrash(t *testing.T) {
	stream := NewStream(1, false)
	headers := http.Header{}
	headers.Add("foo", "bar")
	err := stream.Input.WriteFrame(&SynStreamFrame{
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
