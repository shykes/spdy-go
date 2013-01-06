// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package spdy implements SPDY protocol which is described in
// draft-mbelshe-httpbis-spdy-00.
//
// http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00
package spdy

import (
	"net/http"
	"errors"
	"io"
	"io/ioutil"
)


/*
** A stream is just a place holder for an id, frame reader and frame writer.
*/

type Stream struct {
	Id      uint32
	Input	StreamInput
	Output	StreamOutput
	local	bool	// Was this stream created locally?
	// FIXME: unidirectional
	// FIXME: priority
}

func NewStream(id uint32, local bool) *Stream {
	s := &Stream{
		Id:	id,
		local:	local,
	}
	s.Input = StreamInput{NewHalfStream(s)}
	s.Output = StreamOutput{NewHalfStream(s)}
	return s
}

func (s *Stream) ReadFrame() (Frame, error) {
	return s.Input.ReadFrame()
}

func (s *Stream) WriteFrame(frame Frame) error {
	return s.Output.WriteFrame(frame)
}


func (s *Stream) Reply(headers *http.Header, fin bool) error {
	if headers == nil {
		headers = new(http.Header)
	}
	var flags ControlFlags
	if fin {
		flags = ControlFlagFin
	}
	return s.Output.WriteFrame(&SynReplyFrame{
		StreamId:	s.Id,
		Headers:	*headers,
		CFHeader:	ControlFrameHeader{Flags:flags},
	})
}

func (s *Stream) Syn(headers *http.Header, fin bool) error {
	if headers == nil {
		headers = new(http.Header)
	}
	var flags ControlFlags
	if fin {
		flags = ControlFlagFin
	}
	return s.Output.WriteFrame(&SynStreamFrame{
		StreamId:	s.Id,
		Headers:	*headers,
		CFHeader:	ControlFrameHeader{Flags:flags},
	})
}

func (s *Stream) WriteHeadersFrame(headers *http.Header, fin bool) error {
	if headers == nil {
		headers = &http.Header{}
	}
	var flags ControlFlags
	if fin {
		flags = ControlFlagFin
	}
	return s.Output.WriteFrame(&HeadersFrame{
		StreamId:	s.Id,
		Headers:	*headers,
		CFHeader:	ControlFrameHeader{Flags:flags},
	})
}

func (s *Stream) WriteDataFrame(data []byte, fin bool) error {
	var flags DataFlags
	if fin {
		flags = DataFlagFin
	}
	return s.Output.WriteFrame(&DataFrame{
		StreamId:	s.Id,
		Data:		data,
		Flags:		flags,
	})
}

func (s *Stream) CopyFrom(src io.Reader) error {
	data := make([]byte, 4096)
	for {
		n, err := src.Read(data)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if err := s.WriteDataFrame(data[:n], false); err != nil {
			return err
		}
	}
	return nil
}

func (s *Stream) Rst(status StatusCode) error {
	return s.Output.WriteFrame(&RstStreamFrame{StreamId: s.Id, Status: status})
}

func (s *Stream) ProtocolError() error {
	return s.Rst(ProtocolError)
}

func (stream *Stream) Serve(handler http.Handler) {
	debug("Running handler")
	if handler == nil {
		stream.Rst(RefusedStream)
		return
	}
	w := &ResponseWriter{Stream: stream}
	r, err := stream.ParseHTTPRequest(nil);
	if err != nil {
		// FIXME: send error
		debug("Error parsing http request: %s\n", err)
		return
	}
	debug("[%d] Running handler\n", stream.Id)
	handler.ServeHTTP(w, r)
	debug("Handler returned for stream id %d. Cleaning up.", stream.Id)
	stream.WriteDataFrame(nil, true) // Close the stream in case the handler hasn't
	_, err = io.Copy(ioutil.Discard, r.Body) // Drain all remaining input
	if err != nil {
		debug("Error while draining stream id %d: %s", stream.Id, err)
	}
	debug("Done cleaning up for stream id %d", stream.Id)
}

func (s *Stream) ParseHTTPRequest(drain FrameWriter) (*http.Request, error) {
	if s.Input.nFramesOut > 0 {
		return nil, errors.New("Can't parse HTTP request: first SPDY frame already read")
	}
	frame, err := s.ReadFrame()
	if err != nil {
		return nil, err
	}
	headers := frame.GetHeaders()
	method := headers.Get("method")
	if method == "" {
		method = "GET"
	}
	path := headers.Get("url")
	if path == "" {
		path = "/"
	}
	bodyReader, bodyWriter := io.Pipe()
	go func() {
		Split(s.Input, &DataWriter{bodyWriter}, drain, drain)
		bodyWriter.Close()
	}()
	r, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		return nil, err
	}
	UpdateHeaders(&r.Header, headers)
	return r, nil
}

func (s *Stream) Close() {
	s.Input.HalfStream.Close()
	s.Output.HalfStream.Close()
}


type HalfStream struct {
	stream *Stream
	*ChanFramer
	Headers	http.Header
	nFramesIn	uint32
	nFramesOut	uint32
}

func NewHalfStream(s *Stream) *HalfStream {
	return &HalfStream{
		stream:		s,
		ChanFramer:	NewChanFramer(),
		Headers:	http.Header{},
	}
}


func (s *HalfStream) ReadFrame() (Frame, error) {
	frame, err := s.ChanFramer.ReadFrame()
	if err != nil {
		return nil, err
	}
	s.nFramesOut += 1
	return frame, nil
}

func (s *HalfStream) WriteFrame(frame Frame) error {
	if err := s.ChanFramer.WriteFrame(frame); err != nil {
		return err
	}
	s.nFramesIn += 1
	/* If we sent a frame with FLAG_FIN, mark the output as closed */
	if frame.GetFinFlag() {
		s.Close()
	}
	/* If we sent headers, store them */
	if headers := frame.GetHeaders(); headers != nil {
		UpdateHeaders(&s.Headers, headers)
	}
	/* If we sent a RST_STREAM frame, mark input and output as closed */
	if _, isRst := frame.(*RstStreamFrame); isRst {
		debug("Received RST_STREAM. Closing")
		s.stream.Close()
	}
	return nil
}


type StreamInput struct {
	*HalfStream
}


func (s *StreamInput) WriteFrame(frame Frame) error {
	debug("[StreamInput.WriteFrame]")
	if s.Closed() {
		debug("[StreamInput.WriteFrame] input is closed")
		/*
		 *                      "An endpoint MUST NOT send a RST_STREAM in
		 * response to an RST_STREAM, as doing so would lead to RST_STREAM
		 * loops."
		 *
		 * (http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00#section-2.4.2)
		 */
		if _, isRst := frame.(*RstStreamFrame); !isRst {
			s.stream.Rst(9) // STREAM_ALREADY_CLOSED, introduced in version 3
		}
		return nil
	}
	switch frame.(type) {
		case *SynStreamFrame: {
			if s.nFramesIn > 0 || s.stream.local {
				debug("[StreamInput.WriteFrame] synstream at the wrong time")
				s.stream.ProtocolError()
				return nil
				// ("Received invalid SYN_STREAM frame")
			}
		}
		case *SynReplyFrame: {
			if s.nFramesIn > 0 || !s.stream.local {
				s.stream.ProtocolError()
				return nil
				// ("Received invalid SYN_REPLY frame")
			}
		}
		case *HeadersFrame, *DataFrame: {
			if s.nFramesIn == 0 {
				s.stream.ProtocolError()
				return nil
				// ("Received invalid first frame")
			}
		}
		case *RstStreamFrame: {
			// RST_STREAM frames are always allowed
		}
		default: {
			debug("Received invalid frame")
			s.stream.ProtocolError()
			return nil
		}
	}
	return s.HalfStream.WriteFrame(frame)

}


type StreamOutput struct {
	*HalfStream
}


func (s *StreamOutput) WriteFrame(frame Frame) error {
	if s.Closed() {
		return errors.New("Output closed")
	}
	/* Is this frame type allowed at this point? */
	switch frame.(type) {
		case *SynStreamFrame: {
			if s.nFramesIn > 0 || !s.stream.local {
				return errors.New("Won't send invalid SYN_STREAM frame")
			}
		}
		case *SynReplyFrame: {
			if s.nFramesIn > 0 || s.stream.local {
				return errors.New("Won't send invalid SYN_REPLY frame")
			}
		}
		case *HeadersFrame, *DataFrame: {
			if s.nFramesIn == 0 {
				return errors.New("First frame sent must be SYN_STREAM or SYN_REPLY")
			}
		}
		default: {
			return errors.New("Won't send invalid frame type")
		}
	}
	return s.HalfStream.WriteFrame(frame)
}


/*
** A ChanFramer allows 2 goroutines to send SPDY frames to each other
** using the Framer interface.
**
** Frames are sent through a buffered channel of hardcoded size (currently 4096).
*/

type ChanFramer struct {
	ch	chan Frame
	err	error
}

func NewChanFramer() *ChanFramer {
	return &ChanFramer{
		ch:	make(chan Frame, 4096),
	}
}

func (framer *ChanFramer) WriteFrame(frame Frame) error {
	if framer.err != nil {
		return framer.err
	}
	framer.ch <- frame
	return nil
}

func (framer *ChanFramer) ReadFrame() (Frame, error) {
	/* This will not block if the channel is closed and empty */
	frame, ok := <-framer.ch
	if !ok {
		return nil, framer.err
	}
	return frame, nil
}

func (framer *ChanFramer) Error(err error) {
	if framer.err != nil {
		return
	}
	framer.err = err
	close(framer.ch)
}

func (framer *ChanFramer) Close() {
	framer.Error(io.EOF)
}

func (framer *ChanFramer) Closed() bool {
	return framer.err != nil
}
