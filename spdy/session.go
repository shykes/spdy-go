// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package spdy implements SPDY protocol which is described in
// draft-mbelshe-httpbis-spdy-00.
//
// http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00
package spdy

import (
	"errors"
	"fmt"
	"net/http"
)

/*
** type Session
**
** A high-level representation of a SPDY connection
**  <<
**      connection: A transport-level connection between two endpoints.
**      session: A synonym for a connection.
**  >>
**  (http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00#section-1.2)
** 
 */

type Session struct {
	Server       bool   // Are we the server? (necessary for stream ID numbering)
	lastStreamIdOut uint32 // Last (and highest-numbered) stream ID we allocated
	lastStreamIdIn	uint32 // Last (and highest-numbered) stream ID we received
	streams      map[uint32]*Stream
	handler      http.Handler
	closed       bool
	outputR	     *PipeReader
	outputW      *PipeWriter
}


func NewSession(handler http.Handler, server bool) *Session {
	outputR, outputW := Pipe(4096)
	session := &Session{
		Server:		server,
		streams:	make(map[uint32]*Stream),
		handler:	handler,
		outputR:	outputR,
		outputW:	outputW,
	}
	if session.handler == nil {
		session.outputW.WriteFrame(&GoAwayFrame{})
	}
	return session
}

func (session *Session) Close() {
	session.closed = true
	for id := range session.streams {
		session.CloseStream(id)
	}
}

func (session *Session) Closed() bool {
	return session.closed
}

/*
** Compute the ID which should be used to open the next stream 
** 
** Per http://tools.ietf.org/html/draft-mbelshe-httpbis-spdy-00#section-2.3.2
** <<
** If the server is initiating the stream,
**    the Stream-ID must be even.  If the client is initiating the stream,
**    the Stream-ID must be odd. 0 is not a valid Stream-ID.  Stream-IDs
**    from each side of the connection must increase monotonically as new
**    streams are created.  E.g.  Stream 2 may be created after stream 3,
**    but stream 7 must not be created after stream 9.  Stream IDs do not
**    wrap: when a client or server cannot create a new stream id without
**    exceeding a 31 bit value, it MUST NOT create a new stream.
** >>
 */
func (session *Session) nextIdOut() uint32 {
	if session.lastStreamIdOut == 0 {
		if session.Server {
			return 2
		} else {
			return 1
		}
	}
	// FIXME: optionally return an error on wrap
	// (ping IDs are allowed to wrap, but stream IDs aren't)
	return session.lastStreamIdOut + 2
}

func (session *Session) nextIdIn() uint32 {
	if session.lastStreamIdIn == 0 {
		if session.Server {
			return 1
		} else {
			return 2
		}
	}
	return session.lastStreamIdIn + 2
}

/*
** OpenStream() initiates a new local stream. It does not send SYN_STREAM or
** any other frame. That is the responsibility of the caller. 
*/

func (session *Session) OpenStream() (*Stream, error) {
	newId := session.nextIdOut()
	if stream, err := session.newStream(newId, true); err != nil {
		return nil, err
	} else {
		return stream, nil
	}
	return nil, nil
}


/*
 * Create a new stream and register it at `id` in `session`
 *
 * If `id` is invalid or already registered, the call will fail.
 */

func (session *Session) newStream(id uint32, local bool) (*Stream, error) {
	/* If the ID is valid, register the stream. Otherwise, send a protocol error */
	if !session.streamIdIsValid(id, local) {
		return nil, &RstError{ProtocolError, Error{InvalidStreamId, id}}
	}
	stream, streamPeer := NewStream(id, local)
	session.streams[id] = streamPeer
	if local {
		session.lastStreamIdOut = id
	} else {
		session.lastStreamIdIn = id
	}
	/* Copy stream output to session output */
	go func() {
		err := Copy(session.outputW, streamPeer)
		/* Close the stream if there's an error (inluding EOF) */
		if err != nil {
			session.CloseStream(id)
		} else {
			if streamPeer.Closed {
				session.CloseStream(id)
			}
		}
	}()
	return stream, nil
}

func (session *Session) streamIdIsValid(id uint32, local bool) bool {
	/* Is this ID valid? */
	if local {
		if !session.isLocalId(id) || id != session.nextIdOut() {
			return false
		}
	} else {
		if session.isLocalId(id) || id != session.nextIdIn() {
			return false
		}
	}
	return true
}


func (session *Session) CloseStream(id uint32) error {
	stream, exists := session.streams[id]
	if !exists {
		return errors.New(fmt.Sprintf("No such stream: %v", id))
	}
	stream.Close()
	delete(session.streams, id)
	return nil
}


/*
** Return the number of open streams
*/

func (session *Session) NStreams() int {
	return len(session.streams)
}

func (session *Session) ReadFrame() (Frame, error) {
	frame, err := session.outputR.ReadFrame()
	if err != nil {
		return nil, err
	}
	debug("Sending frame: %#v", frame)
	return frame, nil
}

func (session *Session) WriteFrame(frame Frame) error {
	debug("Received frame: %#v", frame)
	/* Is this frame stream-specific? */
	if streamId := frame.GetStreamId(); streamId != 0 {
		/* SYN_STREAM frame: create the stream */
		if _, ok := frame.(*SynStreamFrame); ok {
			debug("SYN_STREAM: creating new stream")
			if stream, err := session.newStream(streamId, false); err != nil {
				if rstErr, isRstErr := err.(*RstError); isRstErr {
					session.outputW.WriteFrame(&RstStreamFrame{StreamId: streamId, Status: rstErr.Status})
					return nil
				} else {
					return err
				}
			} else {
				go stream.Serve(session.handler)
			}
		}
		streamPeer, exists := session.streams[streamId]
		if !exists {
			session.outputW.WriteFrame(&RstStreamFrame{StreamId: streamId, Status: ProtocolError})
			return nil
		}
		err := streamPeer.WriteFrame(frame)
		if err != nil {
			debug("Error while passing frame to stream: %s. Closing stream.", err)
			session.CloseStream(streamId)
			return err
		} else if streamPeer.Closed {
			debug("Stream %d is fully closed. De-registering", streamId)
		}
	/* Is this frame session-wide? */
	} else {
		switch frame.(type) {
			case *SettingsFrame:		debug("SETTINGS\n")
			case *NoopFrame:		debug("NOOP\n")
			case *PingFrame:		debug("PING\n")
			case *GoAwayFrame:		debug("GOAWAY\n")
			default:			debug("Unknown frame type!")
		}
	}
	return nil
}


func (session *Session) Serve(peer ReadWriter) error {
	defer session.Close()
	if err := Splice(session, peer, true); err != nil {
		return err
	}
	return nil
}

/*
 * Return true if it's legal for `id` to be locally created
 * (eg. even-numbered if we're the server, odd-numbered if we're the client)
 */
func (session *Session) isLocalId(id uint32) bool {
	if session.Server {
		return (id%2 == 0) /* Return true if id is even */
	}
	return (id%2 != 0) /* Return true if id is odd */
}
