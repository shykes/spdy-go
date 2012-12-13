package wire

import (
	"code.google.com/p/go.net/spdy"
	"errors"
	"fmt"
	"io"
	"net"
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
	lastStreamId uint32 // Last (and highest-numbered) stream ID we allocated
	lastStreamIdIn	uint32 // Last (and highest-numbered) stream ID we received
	streams      map[uint32]*Stream
	handler      Handler
	conn         net.Conn
	closed       bool
	err	     error
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

func (session *Session) Error(err error) {
	/* Mark the session as closed before passing the error to the streams,
	** so that stream handlers can reliably check for session state
	*/
	if err == io.EOF {
		err = errors.New("Session closed")
	}
	for _, stream := range session.streams {
		stream.Input.Error(err)
	}
}

func (session *Session) Close() {
	session.Error(io.EOF)
}

func (session *Session) Closed() bool {
	return session.err != nil
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
func (session *Session) nextId(lastId uint32) uint32 {
	if lastId == 0 {
		if session.Server {
			return 2
		} else {
			return 1
		}
	}
	// FIXME: optionally return an error on wrap
	// (ping IDs are allowed to wrap, but stream IDs aren't)
	return lastId + 2
}

func (session *Session) OpenStream() (*Stream, error) {
	newId := session.nextId(session.lastStreamId)
	stream := session.newStream(newId)
	session.lastStreamId = newId
	session.streams[newId] = stream
	return stream, nil
}

func (session *Session) serveStream(id uint32) {
	if session.isLocalId(id) {
		// FIXME: send protocol error
		return
	}
	if id <= session.lastStreamIdIn {
		// FIXME: send protocol error
		return
	}
	stream := session.newStream(id)
	session.lastStreamIdIn = id
	session.streams[id] = stream
}

func (session *Session) newStream(id uint32) *Stream {
	stream := &Stream{
		Id:	id,
		Input:	NewChanFramer(),
		Output:	NewChanFramer(),
	}
	go func() {
		if err := Copy(session.Framer, stream.Output); err != nil {
			session.CloseStream(id)
		}
	}()
	go session.handler.ServeSPDY(stream)
	return stream
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
** Session main loop: run the receive loop and ping loop in parallel,
** and return an error if either one fails.
 */

func (session *Session) run() error {
	debug("Session.run()")
	receiveChan := Promise(func() error { return session.receiveLoop() })
	for {
		var err error
		select {
			case err = <-receiveChan:
		}
		if err != nil {
			session.Error(err)
			return err
		}
	}
	return nil
}


/*
** Return the number of open streams
*/

func (session *Session) NStreams() int {
	return len(session.streams)
}


/*
** Listen for new frames and process them
 */

func (session *Session) receiveLoop() error {
	debug("Starting receive loop\n")
	for {
		rawframe, err := session.ReadFrame()
		if err != nil {
			return err
		}
		debug("Received frame %s\n", rawframe)
		/* Is this frame stream-specific? */
		if streamId := FrameStreamId(rawframe); streamId != 0 {
			/* SYN_STREAM frame: create the stream */
			if _, ok := rawframe.(*spdy.SynStreamFrame); ok {
				session.serveStream(streamId)
			}
			stream, exists := session.streams[streamId]
			if !exists {
				debug("Received frame for inactive stream id %v. Dropping.\n", streamId)
				continue
			}
			err := stream.Input.WriteFrame(rawframe)
			if err != nil {
				session.CloseStream(streamId)
				continue
			}
			/* RST_STREAM frame: destroy the stream */
			if _, ok := rawframe.(*spdy.RstStreamFrame); ok {
				session.CloseStream(streamId)
			}
		/* Is this frame session-wide? */
		} else {
			switch rawframe.(type) {
				case *spdy.SettingsFrame:	debug("SETTINGS\n")
				case *spdy.NoopFrame:		debug("NOOP\n")
				case *spdy.PingFrame:		debug("PING\n")
				case *spdy.GoAwayFrame:		debug("GOAWAY\n")
			}
		}
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
** A stream is just a place holder for an id, frame reader and frame writer.
*/

type Stream struct {
	Id      uint32
	Input	*ChanFramer
	Output	*ChanFramer
}

