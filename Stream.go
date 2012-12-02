
package spdy

import (
    "code.google.com/p/go.net/spdy"
    "io"
    "bytes"
    "bufio"
    "net/http"
    "errors"
)



type Receiver interface {
    Receive() (*[]byte, error)
}

type Sender interface {
    Send(*[]byte) error
}


type Stream struct {
    session         *Session
    Id              uint32
    Input           *StreamReader
    Output          *StreamWriter
    IsMine          bool // Was this stream created locally?
}

func newStream(session *Session, id uint32, IsMine bool) *Stream {
    stream := &Stream{
        session,
        id,
        nil,
        nil,
        IsMine,
    }
    stream.Input = &StreamReader{
        stream:     stream,
        data:       NewMQ(),
        headers:    http.Header{},
    }
    stream.Output = &StreamWriter{
        stream:     stream,
        headers:    http.Header{},
    }
    return stream
}


/*
** StreamReader: read data and headers from a stream
*/


type StreamReader struct {
    stream  *Stream
    headers http.Header
    data    *MQ
    readBuffer  bytes.Buffer
}

func (r *StreamReader) Read(data []byte) (int, error) {
    debug("Read(max %d)\n", len(data))
    for r.readBuffer.Len() == 0 {
        msg, err := r.Receive()
        if err != nil {
            return 0, err
        }
        if msg != nil {
            r.readBuffer.Write(*msg)
        }
    }
    n, err := r.readBuffer.Read(data)
    debug("-> %v, %v\n", n, err)
    return n, err
}

func (reader *StreamReader) WriteTo(dst io.Writer) (int64, error) {
    var written int64 = 0
    for {
        data, err := reader.Receive()
        if err != nil && err != io.EOF {
            return written, err
        }
        if data != nil {
            n, err := dst.Write(*data)
            if err != nil {
                return written, err
            }
            written += int64(n)
        }
        if err == io.EOF {
            return written, nil
        }
    }
    return written, nil
}


func (reader *StreamReader) Headers() *http.Header {
    return &reader.headers
}

func (reader *StreamReader) Receive() (*[]byte, error) {
    msg, err := reader.data.Receive()
    return msg.(*[]byte), err
}

/* Receive and discard all incoming messages until `headerName` is set, then return its value */
func (reader *StreamReader) WaitForHeader(key string) (string, error) {
    for reader.Headers().Get(key) == "" {
        _, err := reader.Receive()
        if err != nil {
            return "", err
        }
    }
    return reader.Headers().Get(key), nil
}


func (reader *StreamReader) Push(data *[]byte) error {
    return reader.data.Send(data)
}

func (reader *StreamReader) Error(err error) {
    reader.data.Error(err)
}

func (reader *StreamReader) Close() {
    debug("[%d] closing input\n", reader.stream.Id)
    reader.data.Close()
}


func (reader *StreamReader) Closed() bool {
    return reader.data.Closed()
}


/*
** StreamWriter: write data and headers to a stream
*/


type StreamWriter struct {
    stream      *Stream
    headers     http.Header
    nFramesSent uint32
    closed      bool
}

func (writer *StreamWriter) Send(data *[]byte) error {
    if writer.nFramesSent == 0 {
        debug("Calling SendHeaders() before sending data\n")
        writer.SendHeaders(false)
    }
    debug("Sending data: %s\n", data)
    return writer.writeFrame(&spdy.DataFrame{
        StreamId:   writer.stream.Id,
        Data:       *data,
    })
}

func (writer *StreamWriter) Write(data []byte) (int, error) {
    err := writer.Send(&data)
    if err != nil {
        return 0, err
    }
    return len(data), nil
}

func (writer *StreamWriter) ReadFrom(src io.Reader) (int64, error) {
    var written int64 = 0
    data := make([]byte, 4096)
    for {
        nRead, err := src.Read(data)
        if err != nil && err != io.EOF {
            return written, err
        }
        eof := (err == io.EOF)
        if nRead != 0 {
            nWritten, err := writer.Write(data[:nRead])
            if err != nil {
                return written, err
            }
            written += int64(nWritten)
        }
        if eof {
            break
        }
    }
    return written, nil
}


func (writer *StreamWriter) SendLines(lines *bufio.Reader) error {
    debug("Sending lines\n")
    for {
        line, _, err := lines.ReadLine()
        if err != nil && err != io.EOF {
            return err
        }
        eof := (err == io.EOF)
        if len(line) != 0 {
            err = writer.Send(&line)
            if err != nil {
                return err
            }
        }
        if eof {
            debug("Received EOF from input\n")
            return io.EOF
        }
   }
   return nil
}

func (writer *StreamWriter) Headers() *http.Header {
    return &writer.headers
}

func (writer *StreamWriter) Close() error {
    debug("[%s] closing output\n", writer.stream.Id)
    /* Send a zero-length data frame with FLAG_FIN set */
    return writer.writeFrame(&spdy.DataFrame{
        StreamId:   writer.stream.Id,
        Data:       []byte{},
        Flags:      0x0|spdy.DataFlagFin,
    })
}

func (writer *StreamWriter) Closed() bool {
    return writer.closed
}

func (writer *StreamWriter) SendHeaders(final bool) error {
    // FIXME
    // Optimization: don't resend all headers every time
    var flags spdy.ControlFlags
    if final {
        // If final == true, send FIN flag to close the connection
        flags |= spdy.ControlFlagFin
    }
    if writer.nFramesSent == 0 {
        if writer.stream.IsMine {
            // If this is the first message we send and we initiated the stream, send SYN_STREAM + headers
            err := writer.writeFrame(&spdy.SynStreamFrame{
                    StreamId: writer.stream.Id,
                    CFHeader: spdy.ControlFrameHeader{Flags: flags},
                    Headers: writer.headers,
            })
            if err != nil {
                return err
            }
        } else {
            // If this is the first message we send and we didn't initiate the stream, send SYN_REPLY + headers
            err := writer.writeFrame(&spdy.SynReplyFrame{
                CFHeader:       spdy.ControlFrameHeader{Flags: flags},
                Headers:        writer.headers,
                StreamId:       writer.stream.Id,
            })
            if err != nil {
                return err
            }
        }
    } else {
        // If this is not the first message we send, send HEADERS + headers
        err := writer.writeFrame(&spdy.HeadersFrame{
            CFHeader:       spdy.ControlFrameHeader{Flags: flags},
            Headers:        writer.headers,
            StreamId:       writer.stream.Id,
        })
        if err != nil {
            return err
        }
    }
    if final {
        writer.closed = true
    }
    return nil
}

func (writer *StreamWriter) writeFrame(frame spdy.Frame) error {
    if writer.Closed() {
        return errors.New("Stream output is closed")
    }
    debug("Writing frame %s", frame)
    err := writer.stream.session.WriteFrame(frame)
    if err != nil {
        return err
    }
    writer.nFramesSent += 1
    return nil
}


func (stream *Stream) Session() *Session {
    // FIXME: simpler to just expose the field
    return stream.session
}

