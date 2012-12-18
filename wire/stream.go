package wire

import (
	"net/http"
	"code.google.com/p/go.net/spdy"
	"errors"
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

func (s *Stream) Rst(status spdy.StatusCode) error {
	return s.Output.WriteFrame(&spdy.RstStreamFrame{StreamId: s.Id, Status: status})
}

func (s *Stream) ProtocolError() error {
	return s.Rst(spdy.ProtocolError)
}


func (s *Stream) Close() {
	s.Input.HalfStream.Close()
	s.Output.HalfStream.Close()
}


type HalfStream struct {
	stream *Stream
	*ChanFramer
	Headers	http.Header
	nFrames	uint32
}

func NewHalfStream(s *Stream) *HalfStream {
	return &HalfStream{
		stream:		s,
		ChanFramer:	NewChanFramer(),
		Headers:	http.Header{},
	}
}

func (s *HalfStream) WriteFrame(frame spdy.Frame) error {
	/* If we sent a frame with FLAG_FIN, mark the output as closed */
	if FrameFinFlag(frame) {
		s.Close()
	}
	/* If we sent headers, store them */
	if headers := FrameHeaders(frame); headers != nil {
		UpdateHeaders(&s.Headers, headers)
	}
	/* If we sent a RST_STREAM frame, mark input and output as closed */
	if _, isRst := frame.(*spdy.RstStreamFrame); isRst {
		s.stream.Close()
	}
	s.nFrames += 1
	return s.ChanFramer.WriteFrame(frame)
}


type StreamInput struct {
	*HalfStream
}


func (s *StreamInput) WriteFrame(frame spdy.Frame) error {
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
		if _, isRst := frame.(*spdy.RstStreamFrame); !isRst {
			s.stream.Rst(9) // STREAM_ALREADY_CLOSED, introduced in version 3
		}
		return nil
	}
	debug("[StreamInput.WriteFrame] checking frame type")
	switch frame.(type) {
		case *spdy.SynStreamFrame: {
			if s.nFrames > 0 || s.stream.local {
				debug("[StreamInput.WriteFrame] synstream at the wrong time")
				s.stream.ProtocolError()
				return nil
				// ("Received invalid SYN_STREAM frame")
			}
		}
		case *spdy.SynReplyFrame: {
			if s.nFrames > 0 || !s.stream.local {
				s.stream.ProtocolError()
				return nil
				// ("Received invalid SYN_REPLY frame")
			}
		}
		case *spdy.HeadersFrame, *spdy.DataFrame: {
			if s.nFrames == 0 {
				s.stream.ProtocolError()
				return nil
				// ("Received invalid first frame")
			}
		}
		default: {
			s.stream.ProtocolError()
			return nil
			// ("Received invalid frame")
		}
	}
	return s.HalfStream.WriteFrame(frame)

}


type StreamOutput struct {
	*HalfStream
}


func (s *StreamOutput) WriteFrame(frame spdy.Frame) error {
	if s.Closed() {
		return errors.New("Output closed")
	}
	/* Is this frame type allowed at this point? */
	switch frame.(type) {
		case *spdy.SynStreamFrame: {
			if s.nFrames > 0 || !s.stream.local {
				return errors.New("Won't send invalid SYN_STREAM frame")
			}
		}
		case *spdy.SynReplyFrame: {
			if s.nFrames > 0 || s.stream.local {
				return errors.New("Won't send invalid SYN_REPLY frame")
			}
		}
		case *spdy.HeadersFrame, *spdy.DataFrame: {
			if s.nFrames == 0 {
				return errors.New("First frame sent must be SYN_STREAM or SYN_REPLY")
			}
		}
		default: {
			return errors.New("Won't send invalid frame type")
		}
	}
	return s.HalfStream.WriteFrame(frame)
}

