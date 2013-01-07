package spdy

import (
	"net/http"
	"testing"
	"bytes"
	"sync"
	"io"
)

func TestHeadersCrash(t *testing.T) {
	_, peer := NewStream(1, false)
	headers := http.Header{}
	headers.Add("foo", "bar")
	err := peer.WriteFrame(&SynStreamFrame{
		StreamId:	1,
		Headers:	headers,
	})
	if err != nil {
		t.Error(err)
	}
	if peer.output.Headers.Get("foo") != "bar" {
		t.Errorf("Stream input didn't store headers (%v != %v)", peer.output.Headers, headers)
	}
}

func TestBodyWrite(t *testing.T) {
	data := []byte("hello world\n")
	session := NewSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}), true)
	session.WriteFrame(&SynStreamFrame{StreamId: 1})
	session.ReadFrame()
	frame, err := session.ReadFrame()
	if err != nil {
		t.Error(err)
	}
	if dataframe, isData := frame.(*DataFrame); !isData {
		t.Error("Writing to ResponseWriter body failed")
	} else {
		if !bytes.Equal(dataframe.Data, data) {
			t.Error("ResponseWriter sent |%#v| instead of |%#v|", dataframe.Data, data)
		}
	}
}

func TestBodyRead(t *testing.T) {
	data := []byte("hello world\n")
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := make([]byte, len(data))
		n, err := r.Body.Read(result)
		if err != nil {
			t.Error(err)
		}
		if n != len(data) {
			t.Error("Request body received %d bytes instead of %d", n, len(data))
		}
		locker.Unlock()
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	if err := session.WriteFrame(&DataFrame{StreamId: 1, Data: data}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestBodyReadClose(t *testing.T) {
	data := []byte("hello world\n")
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := make([]byte, len(data))
		n, err := r.Body.Read(result)
		if err != nil {
			t.Error(err)
		}
		if n != len(data) {
			t.Error("Request body received %d bytes instead of %d", n, len(data))
		}
		n, err = r.Body.Read(result)
		if err != io.EOF {
			t.Error("Request body was not closed")
		}
		if n != 0 {
			t.Error("Body.Read() should have returned 0, returned %d instead", n)
		}
		locker.Unlock()
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	if err := session.WriteFrame(&DataFrame{StreamId: 1, Data: data, Flags: DataFlagFin}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestHeadersRead(t *testing.T) {
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("foo") != "bar" {
			t.Errorf("Request header 'foo' shouldbe 'bar', but it's '%s'", r.Header.Get("foo"))
		}
		locker.Unlock()
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1, Headers: http.Header{"foo": {"bar"}}}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestHeadersWrite(t *testing.T) {
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		w.Header().Set("foo", "bar")
		w.WriteHeader(200)
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	if frame, err := session.ReadFrame(); err != nil {
		t.Error(err)
	} else {
		if replyFrame, isReply := frame.(*SynReplyFrame); !isReply {
			t.Errorf("HTTPResponse.WriteHeader() did not send a SYN_REPLY frame (%#v)", frame)
		} else {
			if replyFrame.Headers.Get("foo") != "bar" {
				t.Errorf("Header foo should be bar, but it's %s", replyFrame.Headers.Get("foo"))
			}
		}
	}
}

func TestHeadersWriteDefault(t *testing.T) {
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	if frame, err := session.ReadFrame(); err != nil {
		t.Error(err)
	} else {
		if replyFrame, isReply := frame.(*SynReplyFrame); !isReply {
			t.Errorf("HTTPResponse.WriteHeader() did not send a SYN_REPLY frame (%#v)", frame)
		} else {
			if replyFrame.Headers.Get("status") != "200" {
				t.Errorf("Header status should be 200, but it's %s", replyFrame.Headers.Get("status"))
			}
		}
	}
}

func TestURL(t *testing.T) {
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/foo/bar" {
			t.Errorf("Request URL should be '/foo/bar', but it is '%s'", r.URL.Path)
		}
		locker.Unlock()
	}), true)
	headers := make(http.Header)
	headers.Set("url", "/foo/bar")
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1, Headers: headers}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestMethod(t *testing.T) {
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Request method should be 'POST', but it is '%s'", r.Method)
		}
		locker.Unlock()
	}), true)
	headers := make(http.Header)
	headers.Set("method", "POST")
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1, Headers: headers}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestDefaultMethod(t *testing.T) {
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Request method should be 'GET', but it is '%s'", r.Method)
		}
		locker.Unlock()
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}
