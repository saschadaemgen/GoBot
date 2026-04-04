package protocol

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrFrameTooLarge  = errors.New("payload exceeds frame size")
	ErrFrameWrongSize = errors.New("frame is not exactly FRAME_SIZE bytes")
	ErrInvalidPadding = errors.New("invalid padding")
)

// Encode serializes a message to JSON and pads it to exactly
// FrameSize bytes using PKCS#7 (standard or extended).
func Encode(msg any) ([]byte, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}

	padLen := FrameSize - len(payload)
	if padLen < 1 {
		return nil, ErrFrameTooLarge
	}

	frame := make([]byte, FrameSize)
	copy(frame, payload)

	if padLen <= 255 {
		// Standard PKCS#7: all padding bytes equal padLen.
		for i := len(payload); i < FrameSize; i++ {
			frame[i] = byte(padLen)
		}
	} else {
		// Extended PKCS#7: fill with 0x01, last 3 bytes are
		// [pad_len_hi] [pad_len_lo] [0x00].
		for i := len(payload); i < FrameSize-3; i++ {
			frame[i] = 0x01
		}
		binary.BigEndian.PutUint16(frame[FrameSize-3:FrameSize-1], uint16(padLen))
		frame[FrameSize-1] = 0x00
	}

	return frame, nil
}

// Decode strips padding from a FrameSize-byte frame and
// returns the raw JSON payload.
func Decode(frame []byte) ([]byte, error) {
	if len(frame) != FrameSize {
		return nil, ErrFrameWrongSize
	}

	lastByte := frame[FrameSize-1]

	var padLen int
	if lastByte > 0 {
		// Standard PKCS#7.
		padLen = int(lastByte)
		// Verify all padding bytes match.
		for i := FrameSize - padLen; i < FrameSize; i++ {
			if frame[i] != lastByte {
				return nil, ErrInvalidPadding
			}
		}
	} else {
		// Extended PKCS#7: last byte is 0x00, preceding 2 bytes
		// are big-endian uint16 pad length.
		padLen = int(binary.BigEndian.Uint16(frame[FrameSize-3 : FrameSize-1]))
		if padLen < 256 || padLen > FrameSize-1 {
			return nil, ErrInvalidPadding
		}
	}

	payloadLen := FrameSize - padLen
	if payloadLen < 2 {
		return nil, ErrInvalidPadding
	}

	payload := make([]byte, payloadLen)
	copy(payload, frame[:payloadLen])
	return payload, nil
}

// DecodeAs decodes a frame and unmarshals the JSON into dst.
func DecodeAs(frame []byte, dst any) error {
	payload, err := Decode(frame)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, dst)
}
