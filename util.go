// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package spdy implements SPDY protocol which is described in
// draft-mbelshe-httpbis-spdy-00.
//
// http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00

package spdy

import (
	"os"
	"log"
	"io"
	"net/http"
)

func (frame *DataFrame)		GetStreamId() (uint32, bool)	{ return frame.StreamId, true }
func (frame *SynStreamFrame)	GetStreamId() (uint32, bool)	{ return frame.StreamId, true }
func (frame *HeadersFrame)	GetStreamId() (uint32, bool)	{ return frame.StreamId, true }
func (frame *SynReplyFrame)	GetStreamId() (uint32, bool)	{ return frame.StreamId, true }
func (frame *RstStreamFrame)	GetStreamId() (uint32, bool)	{ return frame.StreamId, true }
func (frame *NoopFrame)		GetStreamId() (uint32, bool)	{ return 0, false }
func (frame *SettingsFrame)	GetStreamId() (uint32, bool)	{ return 0, false }
func (frame *PingFrame)		GetStreamId() (uint32, bool)	{ return 0, false }
func (frame *GoAwayFrame)	GetStreamId() (uint32, bool)	{ return 0, false }

func (frame *DataFrame)		GetHeaders() *http.Header	{ return nil }
func (frame *SynStreamFrame)	GetHeaders() *http.Header	{ return &frame.Headers}
func (frame *HeadersFrame)	GetHeaders() *http.Header	{ return &frame.Headers}
func (frame *SynReplyFrame)	GetHeaders() *http.Header	{ return &frame.Headers}
func (frame *RstStreamFrame)	GetHeaders() *http.Header	{ return nil }
func (frame *NoopFrame)		GetHeaders() *http.Header	{ return nil }
func (frame *SettingsFrame)	GetHeaders() *http.Header	{ return nil }
func (frame *PingFrame)		GetHeaders() *http.Header	{ return nil }
func (frame *GoAwayFrame)	GetHeaders() *http.Header	{ return nil }

func (frame *DataFrame)		GetFinFlag() bool	{ return frame.Flags&DataFlagFin != 0 }
func (frame *SynStreamFrame)	GetFinFlag() bool	{ return frame.CFHeader.Flags&ControlFlagFin != 0 }
func (frame *HeadersFrame)	GetFinFlag() bool	{ return frame.CFHeader.Flags&ControlFlagFin != 0 }
func (frame *SynReplyFrame)	GetFinFlag() bool	{ return frame.CFHeader.Flags&ControlFlagFin != 0 }
func (frame *RstStreamFrame)	GetFinFlag() bool	{ return frame.CFHeader.Flags&ControlFlagFin != 0 }
func (frame *NoopFrame)		GetFinFlag() bool	{ return frame.CFHeader.Flags&ControlFlagFin != 0 }
func (frame *SettingsFrame)	GetFinFlag() bool	{ return frame.CFHeader.Flags&ControlFlagFin != 0 }
func (frame *PingFrame)		GetFinFlag() bool	{ return frame.CFHeader.Flags&ControlFlagFin != 0 }
func (frame *GoAwayFrame)	GetFinFlag() bool	{ return frame.CFHeader.Flags&ControlFlagFin != 0 }



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


// DummyHandler is an http.Handler which does nothing.
type DummyHandler struct {}

func (f *DummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}


// Copy copies from src to dst until either EOF is reached on src or an
// error occurs. It returns the first error encountered while copying, if any.
//
// A successful Copy returns err == nil, not err == EOF. Because Copy is
// defined to read from src until EOF, it does not treat an EOF from Read
// as an error to be reported.
//
// As a special case, if w is nil, all frames will be discarded.
func Copy(w Writer, r Reader) error {
	for {
		frame, err := r.ReadFrame()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		// If the destination is nil, discard all frames
		if w == nil {
			continue
		}
		err = w.WriteFrame(frame)
		if err != nil {
			return err
		}
	}
	return nil
}

// CopyBytes reads frames from src, extracts payload data
// when it exists, and writes it to dst. It does so until either
// EOF is reached on src or an error occurs. It returns the first error encountered
// while copying, if any.
//
// Frames without a payload (eg. every frame not of type DATA) are discarded.
func CopyBytes(dst io.Writer, src Reader) error {
	for {
		frame, err := src.ReadFrame()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		switch f := frame.(type) {
			case *DataFrame: {
				if _, err := dst.Write(f.Data); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Splice runs Copy(a, b) and Copy(b, a) in 2 distinct goroutines then waits for
// either one or both copies to complete, depending on the value of wait.
//
// - If wait=true, Splice waits for both copies to complete and returns the first error
// encountered, if any. If both copies end with an error, it is undetermined which error
// is returned.
//
// - If wait=false, Splice waits for one copy to complete and returns the first error
// encountered during that copy, if any. The other copy continues in the background.
func Splice(a ReadWriter, b ReadWriter, wait bool) error {
	Ab, Ba := func() error {return Copy(a, b)}, func() error {return Copy(b, a)}
	promiseAb, promiseBa := Promise(Ab), Promise(Ba)
	if wait {
		debug("[SPLICE] Waiting for both copies to complete...\n")
		errAb, errBa := <-promiseAb, <-promiseBa
		if errAb != nil {
			return errAb
		}
		return errBa
	} else {
		for i:=0; i<2; i+= 1 {
			select {
				case err := <-promiseAb: if err == io.EOF { return nil } else { return err }
				case err := <-promiseBa: if err == io.EOF { return nil } else { return err }
			}
		}
	}
	return nil
}


func Extract(src Reader, data io.Writer, headers chan http.Header, drain Writer) error {
	for {
		if frame, err := src.ReadFrame(); err == io.EOF {
			break
		} else if err != nil {
			return err
		} else {
			var err error
			switch f := frame.(type) {
				case *DataFrame:	if (data != nil) { _, err = data.Write(f.Data) }
				case *HeadersFrame:	if (headers != nil) { headers<-f.Headers }
				default:		if (drain != nil) { err = drain.WriteFrame(frame) }
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ExtractData(src Reader, data io.Writer) error {
	return Extract(src, data, nil, nil)
}


// UpdateHeaders appends the contents of newHeaders to headers.
func UpdateHeaders(headers *http.Header, newHeaders *http.Header) {
	for key, values := range *newHeaders {
		for _, value := range values {
			headers.Add(key, value)
		}
	}
}


