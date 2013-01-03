package spdy

import (
	"io"
)

/*
 *
 */


type DataReader struct {
	io.Reader
	StreamId uint32
}

func (reader *DataReader) ReadFrame() (Frame, error) {
	data := make([]byte, 4096)
	n, err := reader.Read(data)
	if n != 0 {
		return &DataFrame{StreamId: reader.StreamId, Data: data[:n]}, err
	}
	return nil, err
}


type DataWriter struct {
	io.Writer
}

func (writer *DataWriter) WriteFrame(frame Frame) error {
	switch f := frame.(type) {
		case *DataFrame: {
			_, err := writer.Write(f.Data)
			return err
		}
	}
	return nil
}


type DataFramer struct {
	r		io.Reader
	w		io.WriteCloser
	streamId	uint32
}

func NewDataFramer(r io.Reader, w io.WriteCloser, streamId uint32) *DataFramer {
	return &DataFramer{r:r, w:w, streamId:streamId}
}

func (framer *DataFramer) ReadFrame() (Frame, error) {
	data := make([]byte, 4096)
	n, err := framer.r.Read(data)
	if n != 0 {
		return &DataFrame{StreamId: framer.streamId, Data: data[:n]}, err
	}
	return nil, err
}

func (framer *DataFramer) WriteFrame(frame Frame) error {
	debug("passing frame: %s\n", frame)
	switch f := frame.(type) {
		case *DataFrame: {
			_, err := framer.w.Write(f.Data)
			return err
		}
		case *RstStreamFrame: {
			debug("Received RST_STREAM frame: closing writer %s\n", framer.w)
			framer.w.Close()
		}
	}
	return nil
}


