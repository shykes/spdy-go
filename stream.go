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
	"fmt"
)


/*
** A stream is just a place holder for an id, frame reader and frame writer.
*/

type Stream struct {
	Id      	uint32
	input		*StreamPipeReader
	output		*StreamPipeWriter
	local		bool	// Was this stream created locally?
	sendErrors	bool
	Closed		bool
	// FIXME: unidirectional
	// FIXME: priority
}

func NewStream(id uint32, local bool) (*Stream, *Stream) {
	debug("NewStream(%d)", id)
	inputR, inputW := StreamPipe(id, local)
	outputR, outputW := StreamPipe(id, !local)
	stream := &Stream{input: inputR,  output: outputW, sendErrors: false, Id: id, local: local}
	peer   := &Stream{input: outputR, output:  inputW, sendErrors: true,  Id: id, local: local}
	return stream, peer
}

func (s *Stream) ReadFrame() (Frame, error) {
	frame, err := s.input.ReadFrame()
	if err != nil {
		return nil, err
	}
	if _, isRst := frame.(*RstStreamFrame); isRst {
		s.Close()
	}
	s.debug("Received %#v err=%#v", frame, err)
	return frame, nil
}

func (s *Stream) debug(msg string, args ...interface{}) {
	debug(fmt.Sprintf("[STREAM %d %p] %s", s.Id, s, msg), args...)
}

func (s *Stream) WriteFrame(frame Frame) error {
	s.debug("Passing %#v", frame)
	err := s.output.WriteFrame(frame)
	if err != nil {
		if rstErr, isRstErr := err.(*RstError); isRstErr && s.sendErrors {
			s.debug("Sending error: %#v", rstErr)
			s.Rst(rstErr.Status)
			return nil
		}
		return err
	}
	if _, isRst := frame.(*RstStreamFrame); isRst {
		s.Close()
	}
	return nil
}

func (s *Stream) Close() {
	if s.Closed {
		return
	}
	s.Closed = true
	s.output.Close()
	s.input.Close()
}


func (s *Stream) Reply(headers *http.Header, fin bool) error {
	if headers == nil {
		headers = new(http.Header)
	}
	var flags ControlFlags
	if fin {
		flags = ControlFlagFin
	}
	return s.WriteFrame(&SynReplyFrame{
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
	return s.WriteFrame(&SynStreamFrame{
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
	return s.WriteFrame(&HeadersFrame{
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
	return s.WriteFrame(&DataFrame{
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
	return s.WriteFrame(&RstStreamFrame{StreamId: s.Id, Status: status})
}

func (stream *Stream) Serve(handler http.Handler) {
	stream.debug("Running handler")
	if handler == nil {
		stream.Rst(RefusedStream)
		return
	}
	w := &ResponseWriter{Stream: stream}
	r, err := stream.ParseHTTPRequest(nil);
	if err != nil {
		// FIXME: send error
		stream.debug("Error parsing http request: %s\n", err)
		return
	}
	handler.ServeHTTP(w, r)
	stream.debug("Handler returned. Cleaning up.")
	stream.WriteDataFrame(nil, true) // Close the stream in case the handler hasn't
	_, err = io.Copy(ioutil.Discard, r.Body) // Drain all remaining input
	if err != nil {
		stream.debug("Error while draining: %s", err)
	}
	stream.debug("Done cleaning up")
}

func (s *Stream) ParseHTTPRequest(drain Writer) (*http.Request, error) {
	if s.input.NFrames > 0 {
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
	s.debug("headers = %#v", *headers)
	path := headers.Get("url")
	if path == "" {
		path = "/"
	}
	s.debug("path = %s", (*headers)["url"])
	bodyReader, bodyWriter := io.Pipe()
	go func() {
		Split(s, &DataWriter{bodyWriter}, drain, drain)
		s.debug("Closing request body")
		bodyWriter.Close()
	}()
	r, err := http.NewRequest(method, path, bodyReader)
	if err != nil {
		return nil, err
	}
	UpdateHeaders(&r.Header, headers)
	return r, nil
}


func StreamPipe(id uint32, reply bool) (*StreamPipeReader, *StreamPipeWriter) {
	pipeReader, pipeWriter := Pipe(4096) // Buffering is Ok after writing, but not before (for sendErrors)
	reader := &StreamPipeReader{PipeReader: pipeReader}
	writer := &StreamPipeWriter{PipeWriter: pipeWriter, id: id, reply: reply, Headers: make(http.Header)}
	return reader, writer
}

type StreamPipeReader struct {
	*PipeReader
}

type StreamPipeWriter struct {
	*PipeWriter
	reply	bool	// If true, must start with SYN_REPLY. Otherwise must start with SYN_STREAM
	closed	bool
	id	uint32
	Headers	http.Header
}

func (p *StreamPipeWriter) WriteFrame(frame Frame) error {
	if p.closed {
		/*
		 *                      "An endpoint MUST NOT send a RST_STREAM in
		 * response to an RST_STREAM, as doing so would lead to RST_STREAM
		 * loops."
		 *
		 * (http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00#section-2.4.2)
		 */
		if _, isRst := frame.(*RstStreamFrame); isRst {
			return &Error{StreamClosed, p.id}
		}
		return &RstError{StreamAlreadyClosed, Error{StreamClosed, p.id}}
	}
	if id, exists := frame.GetStreamId(); !exists || id != p.id {
		return errors.New("Wrong stream ID")
	}
	switch frame.(type) {
		case *SynStreamFrame: {
			if p.NFrames > 0 || p.reply {
				return &RstError{ProtocolError, Error{IllegalSynStream, p.id}}
			}
		}
		case *SynReplyFrame: {
			if p.NFrames > 0 || !p.reply {
				return &RstError{ProtocolError, Error{IllegalSynReply, p.id}}
			}
		}
		case *HeadersFrame, *DataFrame: {
			if p.NFrames == 0 {
				return &RstError{ProtocolError, Error{IllegalFirstFrame, p.id}}
			}
		}
		default: {
			return &Error{UnknownFrameType, p.id}
		}
	}
	if err := p.PipeWriter.WriteFrame(frame); err != nil {
		return err
	}
	/* If FLAG_FIN=true, close the pipe */
	if frame.GetFinFlag() {
		debug("FIN=1, closing StreamPipe")
		p.closed = true
		p.PipeWriter.Close()
	}
	/* On a RST_STREAM, close the pipe */
	if _, isRst := frame.(*RstStreamFrame); isRst {
		debug("Received RST_STREAM. Closing")
		p.closed = true
	}
	/* Store headers */
	if headers := frame.GetHeaders(); headers != nil {
		UpdateHeaders(&p.Headers, headers)
	}
	return nil
}
