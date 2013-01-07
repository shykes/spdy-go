package spdy

import (
	"io"
)

func Pipe(buffer int) (*PipeReader, *PipeWriter) {
	p := &pipe{ch: make(chan Frame, buffer)}
	return &PipeReader{pipe: p}, &PipeWriter{pipe: p}
}


type pipe struct {
	ch	chan Frame
	err	error
}

type PipeReader struct {
	*pipe
	NFrames	int
}

type PipeWriter struct {
	*pipe
	NFrames int
}


func (p *pipe) CloseWithError(err error) error {
	if p.err != nil {
		return nil
	}
	p.err = err
	close(p.ch)
	return nil
}



func (writer *PipeWriter) WriteFrame(frame Frame) error {
	if writer.err != nil {
		return writer.err
	}
	writer.ch <- frame
	writer.NFrames += 1
	return nil
}

func (writer *PipeWriter) Close() error {
	return writer.CloseWithError(io.EOF)
}



func (reader *PipeReader) ReadFrame() (Frame, error) {
	/* This will not block if the channel is closed and empty */
	frame, ok := <-reader.ch
	if !ok {
		return nil, reader.err
	}
	reader.NFrames += 1
	return frame, nil
}

func (reader *PipeReader) Close() error {
	return reader.CloseWithError(io.ErrClosedPipe)
}

