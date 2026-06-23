package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const headerBytes = 4

var ErrFrameTooLarge = errors.New("frame too large")

func ReadFrame(r io.Reader, maxBytes uint32) ([]byte, error) {
	var header [headerBytes]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 {
		return nil, errors.New("empty frame")
	}
	if maxBytes > 0 && n > maxBytes {
		return nil, fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, n, maxBytes)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func WriteFrame(w io.Writer, payload []byte, maxBytes uint32) error {
	if len(payload) == 0 {
		return errors.New("empty frame")
	}
	if maxBytes > 0 && uint64(len(payload)) > uint64(maxBytes) {
		return fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, len(payload), maxBytes)
	}
	if len(payload) > int(^uint32(0)) {
		return ErrFrameTooLarge
	}
	var header [headerBytes]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}
