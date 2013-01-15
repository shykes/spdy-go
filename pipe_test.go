package spdy

import (
	"testing"
	"io"
)

func TestSendOneFrame(t *testing.T) {
	pipeR, pipeW := Pipe(1)
	f_in := NoopFrame{}
	pipeW.WriteFrame(&f_in)
	f_out, _ := pipeR.ReadFrame()
	if f_in != *f_out.(*NoopFrame) {
		t.Errorf("Sent %v, received %v\n", f_in, f_out)
	}
}

func TestCloseWriter(t *testing.T) {
	r, w := Pipe(1)
	frame_in := &NoopFrame{}
	w.WriteFrame(frame_in)
	w.Close()
	if frame_out, err := r.ReadFrame(); err != nil {
		t.Error(err)
	} else if frame_in != frame_out.(*NoopFrame) {
		t.Errorf("Received wrong frame after closing (%#v)", frame_out)
	}
	if frame_out, err := r.ReadFrame(); err != io.EOF || frame_out != nil {
		t.Errorf("ReadFrame() on a closed pipe returned (%#v, %#v) instead of (nil, EOF)", frame_out, err)
	}
}

func TestCloseReader(t *testing.T) {
	r, w := Pipe(1)
	frame_in := &NoopFrame{}
	r.Close()
	if err := w.WriteFrame(frame_in); err != io.ErrClosedPipe {
		t.Errorf("WriteFrame() on a closed pipe returned %#v instead of ErrClosedPipe", err)
	}
	if frame, err := r.ReadFrame(); err != io.ErrClosedPipe || frame != nil  {
		t.Errorf("ReadFrame() on a closed pipe reader returned (%#v, %#v) instead of (nil, ErrClosedPipe)", frame, err)
	}
}
