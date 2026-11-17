package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errFrameTooLarge = errors.New("ipc frame too large")

// frameReader reads one newline-terminated JSON message at a time with a hard
// size bound, so a misbehaving local client cannot exhaust memory.
type frameReader struct {
	reader *bufio.Reader
}

func newFrameReader(reader io.Reader) *frameReader {
	return &frameReader{reader: bufio.NewReaderSize(reader, 32*1024)}
}

func (f *frameReader) read() (Message, error) {
	var frame []byte
	for {
		chunk, err := f.reader.ReadSlice('\n')
		frame = append(frame, chunk...)
		if len(frame) > MaxFrame {
			return Message{}, errFrameTooLarge
		}
		if err == nil {
			break
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(frame) == 0 {
				return Message{}, ErrClosed
			}
			return Message{}, fmt.Errorf("truncated ipc frame: %w", err)
		}
		return Message{}, fmt.Errorf("read ipc frame: %w", err)
	}

	var message Message
	if err := json.Unmarshal(frame, &message); err != nil {
		return Message{}, fmt.Errorf("decode ipc frame: %w", err)
	}
	return message, nil
}
