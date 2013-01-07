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

func (frame *DataFrame)		GetStreamId() uint32	{ return frame.StreamId }
func (frame *SynStreamFrame)	GetStreamId() uint32	{ return frame.StreamId }
func (frame *HeadersFrame)	GetStreamId() uint32	{ return frame.StreamId }
func (frame *SynReplyFrame)	GetStreamId() uint32	{ return frame.StreamId }
func (frame *RstStreamFrame)	GetStreamId() uint32	{ return frame.StreamId }
func (frame *NoopFrame)		GetStreamId() uint32	{ return 0 }
func (frame *SettingsFrame)	GetStreamId() uint32	{ return 0 }
func (frame *PingFrame)		GetStreamId() uint32	{ return 0 }
func (frame *GoAwayFrame)	GetStreamId() uint32	{ return 0 }

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


type DummyHandler struct {}

func (f *DummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}


func Copy(w FrameWriter, r FrameReader) error {
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

func CopyBytes(dst io.Writer, src FrameReader) error {
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

func Splice(a FrameReadWriter, b FrameReadWriter, wait bool) error {
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


func Split(src FrameReader, data FrameWriter, headers FrameWriter, control FrameWriter) error {
	for {
		if frame, err := src.ReadFrame(); err == io.EOF {
			break
		} else if err != nil {
			return err
		} else {
			var err error
			switch frame.(type) {
				case *DataFrame:	err = data.WriteFrame(frame)
				case *HeadersFrame:	if (headers != nil) { err = headers.WriteFrame(frame) }
				default:		if (control != nil) { err = control.WriteFrame(frame) }
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}



/*
** Add the contents of `newHeaders` to `headers`
 */

func UpdateHeaders(headers *http.Header, newHeaders *http.Header) {
	for key, values := range *newHeaders {
		for _, value := range values {
			headers.Add(key, value)
		}
	}
}


