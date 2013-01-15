package spdy

type Socket struct {
	ReadCloser
	WriteCloser
}

func (s *Socket) Close() error {
	s.ReadCloser.Close()
	s.WriteCloser.Close()
	return nil
}
