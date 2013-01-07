package spdy

type Socket struct {
	FrameReadCloser
	FrameWriteCloser
}

func (s *Socket) Close() error {
	s.FrameReadCloser.Close()
	s.FrameWriteCloser.Close()
	return nil
}
