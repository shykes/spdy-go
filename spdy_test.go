// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spdy

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"errors"
	"fmt"
	"sync"
)

func TestHeaderParsing(t *testing.T) {
	headers := http.Header{
		"Url":     []string{"http://www.google.com/"},
		"Method":  []string{"get"},
		"Version": []string{"http/1.1"},
	}
	var headerValueBlockBuf bytes.Buffer
	writeHeaderValueBlock(&headerValueBlockBuf, headers)

	const bogusStreamId = 1
	newHeaders, err := parseHeaderValueBlock(&headerValueBlockBuf, bogusStreamId)
	if err != nil {
		t.Fatal("parseHeaderValueBlock:", err)
	}

	if !reflect.DeepEqual(headers, newHeaders) {
		t.Fatal("got: ", newHeaders, "\nwant: ", headers)
	}
}

func TestCreateParseSynStreamFrame(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer := &Framer{
		headerCompressionDisabled: true,
		w:         buffer,
		headerBuf: new(bytes.Buffer),
		r:         buffer,
	}
	synStreamFrame := SynStreamFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeSynStream,
		},
		StreamId: 2,
		Headers: http.Header{
			"Url":     []string{"http://www.google.com/"},
			"Method":  []string{"get"},
			"Version": []string{"http/1.1"},
		},
	}
	if err := framer.WriteFrame(&synStreamFrame); err != nil {
		t.Fatal("WriteFrame without compression:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame without compression:", err)
	}
	parsedSynStreamFrame, ok := frame.(*SynStreamFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(synStreamFrame, *parsedSynStreamFrame) {
		t.Fatal("got: ", *parsedSynStreamFrame, "\nwant: ", synStreamFrame)
	}

	// Test again with compression
	buffer.Reset()
	framer, err = NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	if err := framer.WriteFrame(&synStreamFrame); err != nil {
		t.Fatal("WriteFrame with compression:", err)
	}
	frame, err = framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame with compression:", err)
	}
	parsedSynStreamFrame, ok = frame.(*SynStreamFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(synStreamFrame, *parsedSynStreamFrame) {
		t.Fatal("got: ", *parsedSynStreamFrame, "\nwant: ", synStreamFrame)
	}
}

func TestCreateParseSynReplyFrame(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer := &Framer{
		headerCompressionDisabled: true,
		w:         buffer,
		headerBuf: new(bytes.Buffer),
		r:         buffer,
	}
	synReplyFrame := SynReplyFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeSynReply,
		},
		StreamId: 2,
		Headers: http.Header{
			"Url":     []string{"http://www.google.com/"},
			"Method":  []string{"get"},
			"Version": []string{"http/1.1"},
		},
	}
	if err := framer.WriteFrame(&synReplyFrame); err != nil {
		t.Fatal("WriteFrame without compression:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame without compression:", err)
	}
	parsedSynReplyFrame, ok := frame.(*SynReplyFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(synReplyFrame, *parsedSynReplyFrame) {
		t.Fatal("got: ", *parsedSynReplyFrame, "\nwant: ", synReplyFrame)
	}

	// Test again with compression
	buffer.Reset()
	framer, err = NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	if err := framer.WriteFrame(&synReplyFrame); err != nil {
		t.Fatal("WriteFrame with compression:", err)
	}
	frame, err = framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame with compression:", err)
	}
	parsedSynReplyFrame, ok = frame.(*SynReplyFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(synReplyFrame, *parsedSynReplyFrame) {
		t.Fatal("got: ", *parsedSynReplyFrame, "\nwant: ", synReplyFrame)
	}
}

func TestCreateParseRstStream(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer, err := NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	rstStreamFrame := RstStreamFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeRstStream,
		},
		StreamId: 1,
		Status:   InvalidStream,
	}
	if err := framer.WriteFrame(&rstStreamFrame); err != nil {
		t.Fatal("WriteFrame:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame:", err)
	}
	parsedRstStreamFrame, ok := frame.(*RstStreamFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(rstStreamFrame, *parsedRstStreamFrame) {
		t.Fatal("got: ", *parsedRstStreamFrame, "\nwant: ", rstStreamFrame)
	}
}

func TestCreateParseSettings(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer, err := NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	settingsFrame := SettingsFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeSettings,
		},
		FlagIdValues: []SettingsFlagIdValue{
			{FlagSettingsPersistValue, SettingsCurrentCwnd, 10},
			{FlagSettingsPersisted, SettingsUploadBandwidth, 1},
		},
	}
	if err := framer.WriteFrame(&settingsFrame); err != nil {
		t.Fatal("WriteFrame:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame:", err)
	}
	parsedSettingsFrame, ok := frame.(*SettingsFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(settingsFrame, *parsedSettingsFrame) {
		t.Fatal("got: ", *parsedSettingsFrame, "\nwant: ", settingsFrame)
	}
}

func TestCreateParseNoop(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer, err := NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	noopFrame := NoopFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeNoop,
		},
	}
	if err := framer.WriteFrame(&noopFrame); err != nil {
		t.Fatal("WriteFrame:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame:", err)
	}
	parsedNoopFrame, ok := frame.(*NoopFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(noopFrame, *parsedNoopFrame) {
		t.Fatal("got: ", *parsedNoopFrame, "\nwant: ", noopFrame)
	}
}

func TestCreateParsePing(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer, err := NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	pingFrame := PingFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypePing,
		},
		Id: 31337,
	}
	if err := framer.WriteFrame(&pingFrame); err != nil {
		t.Fatal("WriteFrame:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame:", err)
	}
	parsedPingFrame, ok := frame.(*PingFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(pingFrame, *parsedPingFrame) {
		t.Fatal("got: ", *parsedPingFrame, "\nwant: ", pingFrame)
	}
}

func TestCreateParseGoAway(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer, err := NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	goAwayFrame := GoAwayFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeGoAway,
		},
		LastGoodStreamId: 31337,
	}
	if err := framer.WriteFrame(&goAwayFrame); err != nil {
		t.Fatal("WriteFrame:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame:", err)
	}
	parsedGoAwayFrame, ok := frame.(*GoAwayFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(goAwayFrame, *parsedGoAwayFrame) {
		t.Fatal("got: ", *parsedGoAwayFrame, "\nwant: ", goAwayFrame)
	}
}

func TestCreateParseHeadersFrame(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer := &Framer{
		headerCompressionDisabled: true,
		w:         buffer,
		headerBuf: new(bytes.Buffer),
		r:         buffer,
	}
	headersFrame := HeadersFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeHeaders,
		},
		StreamId: 2,
	}
	headersFrame.Headers = http.Header{
		"Url":     []string{"http://www.google.com/"},
		"Method":  []string{"get"},
		"Version": []string{"http/1.1"},
	}
	if err := framer.WriteFrame(&headersFrame); err != nil {
		t.Fatal("WriteFrame without compression:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame without compression:", err)
	}
	parsedHeadersFrame, ok := frame.(*HeadersFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(headersFrame, *parsedHeadersFrame) {
		t.Fatal("got: ", *parsedHeadersFrame, "\nwant: ", headersFrame)
	}

	// Test again with compression
	buffer.Reset()
	framer, err = NewFramer(buffer, buffer)
	if err := framer.WriteFrame(&headersFrame); err != nil {
		t.Fatal("WriteFrame with compression:", err)
	}
	frame, err = framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame with compression:", err)
	}
	parsedHeadersFrame, ok = frame.(*HeadersFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(headersFrame, *parsedHeadersFrame) {
		t.Fatal("got: ", *parsedHeadersFrame, "\nwant: ", headersFrame)
	}
}

func TestCreateParseDataFrame(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer, err := NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	dataFrame := DataFrame{
		StreamId: 1,
		Data:     []byte{'h', 'e', 'l', 'l', 'o'},
	}
	if err := framer.WriteFrame(&dataFrame); err != nil {
		t.Fatal("WriteFrame:", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame:", err)
	}
	parsedDataFrame, ok := frame.(*DataFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(dataFrame, *parsedDataFrame) {
		t.Fatal("got: ", *parsedDataFrame, "\nwant: ", dataFrame)
	}
}

func TestCompressionContextAcrossFrames(t *testing.T) {
	buffer := new(bytes.Buffer)
	framer, err := NewFramer(buffer, buffer)
	if err != nil {
		t.Fatal("Failed to create new framer:", err)
	}
	headersFrame := HeadersFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeHeaders,
		},
		StreamId: 2,
		Headers: http.Header{
			"Url":     []string{"http://www.google.com/"},
			"Method":  []string{"get"},
			"Version": []string{"http/1.1"},
		},
	}
	if err := framer.WriteFrame(&headersFrame); err != nil {
		t.Fatal("WriteFrame (HEADERS):", err)
	}
	synStreamFrame := SynStreamFrame{ControlFrameHeader{Version, TypeSynStream, 0, 0}, 2, 0, 0, nil}
	synStreamFrame.Headers = http.Header{
		"Url":     []string{"http://www.google.com/"},
		"Method":  []string{"get"},
		"Version": []string{"http/1.1"},
	}
	if err := framer.WriteFrame(&synStreamFrame); err != nil {
		t.Fatal("WriteFrame (SYN_STREAM):", err)
	}
	frame, err := framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame (HEADERS):", err, buffer.Bytes())
	}
	parsedHeadersFrame, ok := frame.(*HeadersFrame)
	if !ok {
		t.Fatalf("expected HeadersFrame; got %T %v", frame, frame)
	}
	if !reflect.DeepEqual(headersFrame, *parsedHeadersFrame) {
		t.Fatal("got: ", *parsedHeadersFrame, "\nwant: ", headersFrame)
	}
	frame, err = framer.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame (SYN_STREAM):", err, buffer.Bytes())
	}
	parsedSynStreamFrame, ok := frame.(*SynStreamFrame)
	if !ok {
		t.Fatalf("expected SynStreamFrame; got %T %v", frame, frame)
	}
	if !reflect.DeepEqual(synStreamFrame, *parsedSynStreamFrame) {
		t.Fatal("got: ", *parsedSynStreamFrame, "\nwant: ", synStreamFrame)
	}
}

func TestMultipleSPDYFrames(t *testing.T) {
	// Initialize the framers.
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()
	writer, err := NewFramer(pw1, pr2)
	if err != nil {
		t.Fatal("Failed to create writer:", err)
	}
	reader, err := NewFramer(pw2, pr1)
	if err != nil {
		t.Fatal("Failed to create reader:", err)
	}

	// Set up the frames we're actually transferring.
	headersFrame := HeadersFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeHeaders,
		},
		StreamId: 2,
		Headers: http.Header{
			"Url":     []string{"http://www.google.com/"},
			"Method":  []string{"get"},
			"Version": []string{"http/1.1"},
		},
	}
	synStreamFrame := SynStreamFrame{
		CFHeader: ControlFrameHeader{
			version:   Version,
			frameType: TypeSynStream,
		},
		StreamId: 2,
		Headers: http.Header{
			"Url":     []string{"http://www.google.com/"},
			"Method":  []string{"get"},
			"Version": []string{"http/1.1"},
		},
	}

	// Start the goroutines to write the frames.
	go func() {
		if err := writer.WriteFrame(&headersFrame); err != nil {
			t.Fatal("WriteFrame (HEADERS): ", err)
		}
		if err := writer.WriteFrame(&synStreamFrame); err != nil {
			t.Fatal("WriteFrame (SYN_STREAM): ", err)
		}
	}()

	// Read the frames and verify they look as expected.
	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame (HEADERS): ", err)
	}
	parsedHeadersFrame, ok := frame.(*HeadersFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type:", frame)
	}
	if !reflect.DeepEqual(headersFrame, *parsedHeadersFrame) {
		t.Fatal("got: ", *parsedHeadersFrame, "\nwant: ", headersFrame)
	}
	frame, err = reader.ReadFrame()
	if err != nil {
		t.Fatal("ReadFrame (SYN_STREAM):", err)
	}
	parsedSynStreamFrame, ok := frame.(*SynStreamFrame)
	if !ok {
		t.Fatal("Parsed incorrect frame type.")
	}
	if !reflect.DeepEqual(synStreamFrame, *parsedSynStreamFrame) {
		t.Fatal("got: ", *parsedSynStreamFrame, "\nwant: ", synStreamFrame)
	}
}

func TestReadMalformedZlibHeader(t *testing.T) {
	// These were constructed by corrupting the first byte of the zlib
	// header after writing.
	malformedStructs := map[string]string{
		"SynStreamFrame": "gAIAAQAAABgAAAACAAAAAAAAF/nfolGyYmAAAAAA//8=",
		"SynReplyFrame":  "gAIAAgAAABQAAAACAAAX+d+iUbJiYAAAAAD//w==",
		"HeadersFrame":   "gAIACAAAABQAAAACAAAX+d+iUbJiYAAAAAD//w==",
	}
	for name, bad := range malformedStructs {
		b, err := base64.StdEncoding.DecodeString(bad)
		if err != nil {
			t.Errorf("Unable to decode base64 encoded frame %s: %v", name, err)
		}
		buf := bytes.NewBuffer(b)
		reader, err := NewFramer(buf, buf)
		if err != nil {
			t.Fatalf("NewFramer: %v", err)
		}
		_, err = reader.ReadFrame()
		if err != zlib.ErrHeader {
			t.Errorf("Frame %s, expected: %#v, actual: %#v", name, zlib.ErrHeader, err)
		}
	}
}

type zeroStream struct {
	frame   Frame
	encoded string
}

var streamIdZeroFrames = map[string]zeroStream{
	"SynStreamFrame": {
		&SynStreamFrame{StreamId: 0},
		"gAIAAQAAABgAAAAAAAAAAAAAePnfolGyYmAAAAAA//8=",
	},
	"SynReplyFrame": {
		&SynReplyFrame{StreamId: 0},
		"gAIAAgAAABQAAAAAAAB4+d+iUbJiYAAAAAD//w==",
	},
	"RstStreamFrame": {
		&RstStreamFrame{StreamId: 0},
		"gAIAAwAAAAgAAAAAAAAAAA==",
	},
	"HeadersFrame": {
		&HeadersFrame{StreamId: 0},
		"gAIACAAAABQAAAAAAAB4+d+iUbJiYAAAAAD//w==",
	},
	"DataFrame": {
		&DataFrame{StreamId: 0},
		"AAAAAAAAAAA=",
	},
	"PingFrame": {
		&PingFrame{Id: 0},
		"gAIABgAAAAQAAAAA",
	},
}

func TestNoZeroStreamId(t *testing.T) {
	for name, f := range streamIdZeroFrames {
		b, err := base64.StdEncoding.DecodeString(f.encoded)
		if err != nil {
			t.Errorf("Unable to decode base64 encoded frame %s: %v", f, err)
			continue
		}
		framer, err := NewFramer(ioutil.Discard, bytes.NewReader(b))
		if err != nil {
			t.Fatalf("NewFramer: %v", err)
		}
		err = framer.WriteFrame(f.frame)
		checkZeroStreamId(t, name, "WriteFrame", err)

		_, err = framer.ReadFrame()
		checkZeroStreamId(t, name, "ReadFrame", err)
	}
}

func checkZeroStreamId(t *testing.T, frame string, method string, err error) {
	if err == nil {
		t.Errorf("%s ZeroStreamId, no error on %s", method, frame)
		return
	}
	eerr, ok := err.(*Error)
	if !ok || eerr.Err != ZeroStreamId {
		t.Errorf("%s ZeroStreamId, incorrect error %#v, frame %s", method, eerr, frame)
	}
}

// [...] If the server is initiating the stream, the Stream-ID must be even. [...]

func TestServerStreamIdMustBeEven(t *testing.T) {
    s := NewSession(new(DummyHandler), true)
	for i:=0; i<42; i+=1 {
	    stream, err := s.InitiateStream()
	 if err != nil {
	    t.Error(err)
	}
	if stream.Id % 2 != 0 {
	    t.Errorf("If the server is initiating the stream, the Stream-ID must be even.")
	}
    }
}

// [...] If the client is initiating the stream, the Stream-ID must be odd. [...]

func TestClientStreamIdMustBeOdd(t *testing.T) {
    s := NewSession(new(DummyHandler), false)
	for i:=0; i<42; i+=1 {
	    stream, err := s.InitiateStream()
	 if err != nil {
	    t.Error(err)
	}
	if stream.Id % 2 != 1 {
	    t.Errorf("If the client is initiating the stream, the Stream-ID must be odd.")
	}
    }
}

// [...] 0 is not a valid Stream-ID. [...]

func TestStreamIDZeroNotValid(t *testing.T) {
    for _, isServer := range []bool{true, false} {
	s := NewSession(new (DummyHandler), isServer)
	    stream, err := s.InitiateStream()
	    if err != nil {
		t.Error(err)
	    }
	    if stream.Id == 0 {
		t.Errorf("0 is not a valid Stream-ID.")
	    }
	    if err := s.WriteFrame(&SynStreamFrame{StreamId: 0}); err != nil {
		t.Error(err)
	    }
	    // Write a ping frame so there's always at least one frame
	    // to read
	    if err := s.WriteFrame(&PingFrame{}); err != nil {
		t.Error(err)
	    }
	    frame, err := s.ReadFrame()
	    if rstFrame, isRst := frame.(*RstStreamFrame); !isRst {
		t.Errorf("0 is not a valid Stream-ID.")
	    } else {
		if rstFrame.Status != ProtocolError {
		    t.Errorf("0 is not a valid Stream-ID.")
		}
		if rstFrame.StreamId != 0 {
		    t.Errorf("0 is not a valid Stream-ID.")
		}
	    }
	}
}

// [...] Stream-IDs from each side of the connection must increase monotonically as new
// streams are created [...]

func TestStreamIDIncrementLocal(t *testing.T) {
	for _, isServer := range []bool{true, false} {
		s := NewSession(new(DummyHandler), isServer)
		var previousId uint32
		for i:=0; i<42; i+=1 {
			stream, err := s.InitiateStream()
			if err != nil {
				t.Error(err)
			}
			if previousId != 0 && stream.Id != previousId + 2 {
				t.Error("Stream-IDs from each side of the connection must increase monotically as new streams are created")
			}
		}
	}
}

// [...] E.g. Stream 2 may be created after stream 3, [...]

func TestStream2AfterStream3(t *testing.T) {
	s := NewSession(new(DummyHandler), false)
	s.InitiateStream()
	stream3, err := s.InitiateStream(); if err != nil {
		t.Error(err)
	} else {
		if stream3.Id != 3 {
			t.Error("Second client-initiated stream should have ID=3")
		}
	}
	if _, err := SendExpect(s, &SynStreamFrame{StreamId: 2}, nil); err != nil {
		t.Error(err)
	}
}	

//    but stream 7 must not be created after stream 9.

func TestStream7AfterStream9(t *testing.T) {
	s := NewSession(new(DummyHandler), true)
	if _, err := SendExpect(s, &SynStreamFrame{StreamId:9}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SendExpect(s, &SynStreamFrame{StreamId:7}, reflect.TypeOf(&RstStreamFrame{})); err != nil {
		t.Fatal(err)
	}
}	

//    Stream IDs do not
//    wrap: when a client or server cannot create a new stream id without
//    exceeding a 31 bit value, it MUST NOT create a new stream.

func TestIdWrap(t *testing.T) {
	for _, isServer := range []bool{true, false} {
		s := NewSession(new(DummyHandler), isServer)
		if isServer {
			s.lastStreamIdOut = 0xffffffff
		} else {
			s.lastStreamIdOut = 0xffffffff - 1
		}
		_, err := s.InitiateStream()
		if err == nil {
			t.Error("[...] when a client or server cannot create a new stream id without exceeding a 31 bit value, it MUST NOT create a new stream")
		}
		if s.NStreams() != 0 {
			t.Error("[...] when a client or server cannot create a new stream id without exceeding a 31 bit value, it MUST NOT create a new stream")

		}
	}
}

func SendExpect(s *Session, frameIn Frame, frameTypeOut reflect.Type) (Frame, error) {
	if err := s.WriteFrame(&PingFrame{Id:42}); err != nil {
		return nil, err
	}
	if err := s.WriteFrame(frameIn); err != nil {
		return nil, err
	}
	frame, err := s.ReadFrame()
	if err != nil {
		return nil, err
	}
	var receivedNothing bool
	switch ff := frame.(type) {
		case *PingFrame: {
			if ff.Id == 42 {
				receivedNothing = true
			}
		}
	}
	if receivedNothing {
		if frameTypeOut != nil {
			// Expected something but received nothing", frameTypeOut)
		}
	} else {
		if frameTypeOut == nil {
			// Expected nothing but received something
			return nil, errors.New(fmt.Sprintf("Expected nothing but received %#v", frame))
		} else if frameTypeOut != reflect.TypeOf(frame) {
			return nil, errors.New("Received wrong frame type")
		}
	}
	if frameTypeOut != nil {
		if _, err := s.ReadFrame(); err != nil {
			return nil, err
		}
	}
	return frame, nil
}

func TestSynStreamInvalidId(t *testing.T) {
	s := NewSession(new(DummyHandler), true)
	if err := s.WriteFrame(&SynStreamFrame{StreamId: 2}); err != nil {
		t.Error(err)
	}
	frame, err := s.ReadFrame()
	if err != nil {
		t.Error("Session didn't send protocol error on invalid stream id")
		return
	}
	if f, isRst := frame.(*RstStreamFrame); isRst {
		if f.Status != ProtocolError {
			t.Error("Session didn't send protocol error on invalid stream id")
		}
	} else {
		t.Error("Session didn't send protocol error on invalid stream id")
	}

}


func TestSessionInitiateStreamServer(t *testing.T) {
	session := NewSession(new(DummyHandler), true)
	stream, err := session.InitiateStream()
	if err != nil {
		t.Error(err)
	}
	if stream.Id != 2 {
		t.Errorf("First server-created stream should be 2 (not %d)", stream.Id)
	}
}

func TestSessionInitiateStreamClient(t *testing.T) {
	session := NewSession(new(DummyHandler), false)
	stream, err := session.InitiateStream()
	if err != nil {
		t.Error(err)
	}
	if stream.Id != 1 {
		t.Errorf("First client-created stream should be 1 (not %d)", stream.Id)
	}
}


func TestNStreams(t *testing.T) {
	session := NewSession(new(DummyHandler), true)
	if session.NStreams() != 0 {
		t.Errorf("NStreams() for empty session should be 0 (not %d)", session.NStreams())
	}
	session.InitiateStream()
	if session.NStreams() != 1 {
		t.Errorf("NStreams() should be 1 (not %d)", session.NStreams())
	}
}

func TestCloseStream(t *testing.T) {
	session := NewSession(new(DummyHandler), false)
	session.InitiateStream()
	session.CloseStream(1)
	if session.NStreams() != 0 {
		t.Errorf("CloseStream() did not delete the stream")
	}
}

func TestHeadersCrash(t *testing.T) {
	_, peer := NewStream(1, false)
	headers := http.Header{}
	headers.Add("foo", "bar")
	err := peer.WriteFrame(&SynStreamFrame{
		StreamId:	1,
		Headers:	headers,
	})
	if err != nil {
		t.Error(err)
	}
	if peer.output.Headers.Get("foo") != "bar" {
		t.Errorf("Stream input didn't store headers (%v != %v)", peer.output.Headers, headers)
	}
}

func TestBodyWrite(t *testing.T) {
	data := []byte("hello world\n")
	session := NewSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}), true)
	session.WriteFrame(&SynStreamFrame{StreamId: 1})
	session.ReadFrame()
	frame, err := session.ReadFrame()
	if err != nil {
		t.Error(err)
	}
	if dataframe, isData := frame.(*DataFrame); !isData {
		t.Error("Writing to ResponseWriter body failed")
	} else {
		if !bytes.Equal(dataframe.Data, data) {
			t.Error("ResponseWriter sent |%#v| instead of |%#v|", dataframe.Data, data)
		}
	}
}

func TestBodyRead(t *testing.T) {
	data := []byte("hello world\n")
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := make([]byte, len(data))
		n, err := r.Body.Read(result)
		if err != nil {
			t.Error(err)
		}
		if n != len(data) {
			t.Error("Request body received %d bytes instead of %d", n, len(data))
		}
		locker.Unlock()
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	if err := session.WriteFrame(&DataFrame{StreamId: 1, Data: data}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestBodyReadClose(t *testing.T) {
	data := []byte("hello world\n")
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := make([]byte, len(data))
		n, err := r.Body.Read(result)
		if err != nil {
			t.Error(err)
		}
		if n != len(data) {
			t.Error("Request body received %d bytes instead of %d", n, len(data))
		}
		n, err = r.Body.Read(result)
		if err != io.EOF {
			t.Error("Request body was not closed")
		}
		if n != 0 {
			t.Error("Body.Read() should have returned 0, returned %d instead", n)
		}
		locker.Unlock()
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	if err := session.WriteFrame(&DataFrame{StreamId: 1, Data: data, Flags: DataFlagFin}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestHeadersRead(t *testing.T) {
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("foo") != "bar" {
			t.Errorf("Request header 'foo' shouldbe 'bar', but it's '%s'", r.Header.Get("foo"))
		}
		locker.Unlock()
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1, Headers: http.Header{"foo": {"bar"}}}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestHeadersWrite(t *testing.T) {
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		w.Header().Set("foo", "bar")
		w.WriteHeader(200)
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	if frame, err := session.ReadFrame(); err != nil {
		t.Error(err)
	} else {
		if replyFrame, isReply := frame.(*SynReplyFrame); !isReply {
			t.Errorf("HTTPResponse.WriteHeader() did not send a SYN_REPLY frame (%#v)", frame)
		} else {
			if replyFrame.Headers.Get("foo") != "bar" {
				t.Errorf("Header foo should be bar, but it's %s", replyFrame.Headers.Get("foo"))
			}
		}
	}
}

func TestHeadersWriteDefault(t *testing.T) {
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	if frame, err := session.ReadFrame(); err != nil {
		t.Error(err)
	} else {
		if replyFrame, isReply := frame.(*SynReplyFrame); !isReply {
			t.Errorf("HTTPResponse.WriteHeader() did not send a SYN_REPLY frame (%#v)", frame)
		} else {
			if replyFrame.Headers.Get("status") != "200" {
				t.Errorf("Header status should be 200, but it's %s", replyFrame.Headers.Get("status"))
			}
		}
	}
}

func TestURL(t *testing.T) {
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/foo/bar" {
			t.Errorf("Request URL should be '/foo/bar', but it is '%s'", r.URL.Path)
		}
		locker.Unlock()
	}), true)
	headers := make(http.Header)
	headers.Set("url", "/foo/bar")
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1, Headers: headers}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestMethod(t *testing.T) {
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Request method should be 'POST', but it is '%s'", r.Method)
		}
		locker.Unlock()
	}), true)
	headers := make(http.Header)
	headers.Set("method", "POST")
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1, Headers: headers}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

func TestDefaultMethod(t *testing.T) {
	var locker sync.Mutex
	locker.Lock()
	session := NewSession(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Request method should be 'GET', but it is '%s'", r.Method)
		}
		locker.Unlock()
	}), true)
	if err := session.WriteFrame(&SynStreamFrame{StreamId: 1}); err != nil {
		t.Error(err)
	}
	locker.Lock()
}

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
