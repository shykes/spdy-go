package wire

import (
	"code.google.com/p/go.net/spdy"
	"errors"
	"fmt"
	"io"
	"net"
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
	Framer
	Server       bool   // Are we the server? (necessary for stream ID numbering)
	lastStreamIdOut uint32 // Last (and highest-numbered) stream ID we allocated
	lastStreamIdIn	uint32 // Last (and highest-numbered) stream ID we received
	streams      map[uint32]*Stream
	handler      Handler
	conn         net.Conn
	closed       bool
}


func NewSession(framer Framer, handler Handler, server bool) *Session {
	if handler == nil {
		return nil
	}
	session := &Session{
		Framer:		framer,
		Server:		server,
		streams:	make(map[uint32]*Stream),
		handler:	handler,
	}
	go session.run()
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
	if err := session.newStream(newId, true); err != nil {
		return nil, err
	}
	return session.streams[newId], nil
}


/*
 * Create a new stream and register it at `id` in `session`
 *
 * If `id` is invalid or already registered, the call will fail.
 */

func (session *Session) newStream(id uint32, local bool) error {
	/* Is this ID valid? */
	if local {
		if !session.isLocalId(id) || id != session.nextIdOut() {
			return errors.New("Invalid local stream id")
		}
	} else {
		if session.isLocalId(id) || id != session.nextIdIn() {
			return errors.New("Invalid remote stream id")
		}
	}
	debug("ID=%d (isLocalID: %v) local=%v: ok", id, session.isLocalId(id), local)
	/* Is this ID already in use? */
	if _, alreadyExists := session.streams[id]; alreadyExists {
		return errors.New(fmt.Sprintf("Stream %d already exists", id))
	}
	stream := NewStream(id, local)
	session.streams[id] = stream
	if local {
		session.lastStreamIdOut = id
	} else {
		session.lastStreamIdIn = id
	}
	/* Copy stream output to session output */
	go func() {
		err := Copy(session, stream.Output)
		/* Close the stream if there's an error */
		if err != nil {
			session.CloseStream(id)
		}
		/* If stream is already half-closed, close it */
		if stream.Input.Closed() {
			session.CloseStream(id)
		}
	}()
	go session.handler.ServeSPDY(stream)
	return nil
}

func (session *Session) CloseStream(id uint32) error {
	stream, exists := session.streams[id]
	if !exists {
		return errors.New(fmt.Sprintf("No such stream: %v", id))
	}
	stream.Input.Close()
	delete(session.streams, id)
	return nil
}

/*
** Listen for new frames and process them
 */

func (session *Session) run() error {
	debug("Starting receive loop\n")
	for {
		rawframe, err := session.ReadFrame()
		if err != nil {
			session.Close()
			return err
		}
		debug("Received frame %s\n", rawframe)
		session.processFrame(rawframe)
	}
	return nil
}


/*
** Return the number of open streams
*/

func (session *Session) NStreams() int {
	return len(session.streams)
}



func (session *Session) processFrame(frame spdy.Frame) {
	/* Is this frame stream-specific? */
	if streamId := FrameStreamId(frame); streamId != 0 {
		/* SYN_STREAM frame: create the stream */
		if _, ok := frame.(*spdy.SynStreamFrame); ok {
			debug("SYN_STREAM: creating new stream")
			if err := session.newStream(streamId, false); err != nil {
				/* protocol error */
				debug("Protocol error")
				session.WriteFrame(&spdy.RstStreamFrame{
					StreamId: streamId,
					Status: spdy.ProtocolError,
				})
				return
			}
		}
		stream, exists := session.streams[streamId]
		if !exists {
			/* protocol error */
			debug("Protocol error")
			session.WriteFrame(&spdy.RstStreamFrame{
				StreamId: streamId,
				Status: spdy.ProtocolError,
			})
			return
		}
		debug("Sending frame %v to stream %d", frame, streamId)
		err := stream.Input.WriteFrame(frame)
		debug("done")
		if err == io.EOF {
			debug("Stream %d input closed", streamId)
			/* If stream is already half-closed, close it */
			if stream.Output.Closed() {
				debug("Stream %d output was already closed, de-registering", streamId)
				session.CloseStream(streamId)
			}
		} else if err != nil {
		/* Close the stream if there's an error */
			session.CloseStream(streamId)
			return
		}
	/* Is this frame session-wide? */
	} else {
		switch frame.(type) {
			case *spdy.SettingsFrame:	debug("SETTINGS\n")
			case *spdy.NoopFrame:		debug("NOOP\n")
			case *spdy.PingFrame:		debug("PING\n")
			case *spdy.GoAwayFrame:		debug("GOAWAY\n")
		}
	}
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


func FrameStreamId(rawframe spdy.Frame) uint32 {
	switch frame := rawframe.(type) {
		case *spdy.SynReplyFrame:	return frame.StreamId
		case *spdy.SynStreamFrame:	return frame.StreamId
		case *spdy.DataFrame:		return frame.StreamId
		case *spdy.HeadersFrame:	return frame.StreamId
		case *spdy.RstStreamFrame:	return frame.StreamId
		case *spdy.SettingsFrame:	return 0
		case *spdy.NoopFrame:		return 0
		case *spdy.PingFrame:		return 0
		case *spdy.GoAwayFrame:		return 0
	}
	return 0
}

/*
 * Returns true if a given frame's FIN flag is set, false otherwise.
 */

func FrameFinFlag(rawframe spdy.Frame) bool {
	switch f := rawframe.(type) {
		case *spdy.DataFrame:		return f.Flags&spdy.DataFlagFin != 0
		case *spdy.SynReplyFrame:	return f.CFHeader.Flags&spdy.ControlFlagFin != 0
		case *spdy.SynStreamFrame:	return f.CFHeader.Flags&spdy.ControlFlagFin != 0
		case *spdy.HeadersFrame:	return f.CFHeader.Flags&spdy.ControlFlagFin != 0
	}
	return false
}


/*
 * Return a frame's headers if it can carry any. Return nil otherwise.
 */

func FrameHeaders(frame spdy.Frame) *http.Header {
	switch f := frame.(type) {
		case *spdy.HeadersFrame:	return &f.Headers
		case *spdy.SynStreamFrame:	return &f.Headers
		case *spdy.SynReplyFrame:	return &f.Headers
	}
	return nil
}
