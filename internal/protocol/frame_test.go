package protocol

import (
	"testing"
)

func TestEncodeDecodeBlock(t *testing.T) {
	msg := BlockMsg{
		V:       ProtocolVersion,
		Type:    TypeBlock,
		ID:      "550e8400-e29b-41d4-a716-446655440000",
		TS:      "2026-04-04T12:00:00.000Z",
		QueueID: "smp15.simplex.im#abc123",
		GroupID: "group_def456",
		Sender: Sender{
			MemberID:    "mem_789",
			DisplayName: "Alice",
			Role:        "member",
			ContactID:   "contact_012",
		},
		Payload: "dGVzdCBwYXlsb2Fk", // "test payload" base64
	}

	frame, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	if len(frame) != FrameSize {
		t.Fatalf("frame length = %d, want %d", len(frame), FrameSize)
	}

	var decoded BlockMsg
	if err := DecodeAs(frame, &decoded); err != nil {
		t.Fatalf("DecodeAs() error: %v", err)
	}

	if decoded.Type != TypeBlock {
		t.Errorf("Type = %q, want %q", decoded.Type, TypeBlock)
	}
	if decoded.V != ProtocolVersion {
		t.Errorf("V = %d, want %d", decoded.V, ProtocolVersion)
	}
	if decoded.ID != msg.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, msg.ID)
	}
	if decoded.Sender.DisplayName != "Alice" {
		t.Errorf("Sender.DisplayName = %q, want %q", decoded.Sender.DisplayName, "Alice")
	}
	if decoded.Payload != msg.Payload {
		t.Errorf("Payload = %q, want %q", decoded.Payload, msg.Payload)
	}
}

func TestEncodeDecodeSmallMessage(t *testing.T) {
	// Small messages like ping/pong/ack use extended PKCS#7
	// because padding exceeds 255 bytes.
	msg := PingMsg{
		V:    ProtocolVersion,
		Type: TypePing,
		ID:   "550e8400-e29b-41d4-a716-446655440001",
		TS:   "2026-04-04T12:00:30.000Z",
		Seq:  42,
	}

	frame, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	if len(frame) != FrameSize {
		t.Fatalf("frame length = %d, want %d", len(frame), FrameSize)
	}

	// Verify extended padding: last byte must be 0x00.
	if frame[FrameSize-1] != 0x00 {
		t.Errorf("last byte = 0x%02x, want 0x00 (extended PKCS#7)", frame[FrameSize-1])
	}

	var decoded PingMsg
	if err := DecodeAs(frame, &decoded); err != nil {
		t.Fatalf("DecodeAs() error: %v", err)
	}

	if decoded.Type != TypePing {
		t.Errorf("Type = %q, want %q", decoded.Type, TypePing)
	}
	if decoded.Seq != 42 {
		t.Errorf("Seq = %d, want %d", decoded.Seq, 42)
	}
}

func TestEncodeDecodeResult(t *testing.T) {
	msg := ResultMsg{
		V:         ProtocolVersion,
		Type:      TypeResult,
		ID:        "660e8400-e29b-41d4-a716-446655440000",
		TS:        "2026-04-04T12:00:00.350Z",
		RefID:     "550e8400-e29b-41d4-a716-446655440000",
		Seq:       1001,
		HasAction: true,
		Action: Action{
			Command:        CmdKick,
			TargetMemberID: "mem_789",
			GroupID:        "group_def456",
			ReplyText:      "User removed for spam.",
			BlockHash:      "sha256:abcdef1234567890",
		},
		Signature: "base64signaturedata",
	}

	frame, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var decoded ResultMsg
	if err := DecodeAs(frame, &decoded); err != nil {
		t.Fatalf("DecodeAs() error: %v", err)
	}

	if decoded.HasAction != true {
		t.Error("HasAction should be true")
	}
	if decoded.Action.Command != CmdKick {
		t.Errorf("Command = %q, want %q", decoded.Action.Command, CmdKick)
	}
	if decoded.Action.ReplyText != "User removed for spam." {
		t.Errorf("ReplyText = %q, want %q", decoded.Action.ReplyText, "User removed for spam.")
	}
	if decoded.Seq != 1001 {
		t.Errorf("Seq = %d, want %d", decoded.Seq, 1001)
	}
}

func TestEncodeDecodeDummy(t *testing.T) {
	// Dummy result must encode/decode identically to real result.
	msg := ResultMsg{
		V:         ProtocolVersion,
		Type:      TypeResult,
		ID:        "660e8400-e29b-41d4-a716-446655440001",
		TS:        "2026-04-04T12:00:00.420Z",
		RefID:     "550e8400-e29b-41d4-a716-446655440003",
		Seq:       1002,
		HasAction: false,
		Action: Action{
			Command:        CmdNone,
			TargetMemberID: "",
			GroupID:        "",
			ReplyText:      "",
			BlockHash:      "",
		},
		Signature: "base64dummysignature",
	}

	frame, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var decoded ResultMsg
	if err := DecodeAs(frame, &decoded); err != nil {
		t.Fatalf("DecodeAs() error: %v", err)
	}

	if decoded.HasAction != false {
		t.Error("HasAction should be false for dummy")
	}
	if decoded.Action.Command != CmdNone {
		t.Errorf("Command = %q, want %q", decoded.Action.Command, CmdNone)
	}
}

func TestEncodeDecodeHello(t *testing.T) {
	msg := HelloMsg{
		V:               ProtocolVersion,
		Type:            TypeHello,
		ID:              "660e8400-e29b-41d4-a716-446655440099",
		TS:              "2026-04-04T12:00:00.010Z",
		ProtocolVersion: ProtocolVersion,
		PubkeyFprint:    "sha256:a1b2c3d4e5f6",
		LastSeq:         1000,
		FirmwareVersion: "1.0.0",
		KnownQueues:     []string{"smp15.simplex.im#abc123", "smp15.simplex.im#def456"},
	}

	frame, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	var decoded HelloMsg
	if err := DecodeAs(frame, &decoded); err != nil {
		t.Fatalf("DecodeAs() error: %v", err)
	}

	if len(decoded.KnownQueues) != 2 {
		t.Fatalf("KnownQueues length = %d, want 2", len(decoded.KnownQueues))
	}
	if decoded.KnownQueues[0] != "smp15.simplex.im#abc123" {
		t.Errorf("KnownQueues[0] = %q, want %q", decoded.KnownQueues[0], "smp15.simplex.im#abc123")
	}
	if decoded.FirmwareVersion != "1.0.0" {
		t.Errorf("FirmwareVersion = %q, want %q", decoded.FirmwareVersion, "1.0.0")
	}
}

func TestEncodePayloadTooLarge(t *testing.T) {
	// Create a message with a payload that exceeds frame size.
	msg := BlockMsg{
		V:       ProtocolVersion,
		Type:    TypeBlock,
		Payload: string(make([]byte, FrameSize)),
	}

	_, err := Encode(msg)
	if err == nil {
		t.Fatal("Encode() should return error for oversized payload")
	}
}

func TestDecodeWrongSize(t *testing.T) {
	_, err := Decode([]byte("too short"))
	if err != ErrFrameWrongSize {
		t.Errorf("error = %v, want ErrFrameWrongSize", err)
	}
}

func TestDecodeInvalidPadding(t *testing.T) {
	// Create a frame with inconsistent standard padding.
	frame := make([]byte, FrameSize)
	frame[FrameSize-1] = 5
	frame[FrameSize-2] = 5
	frame[FrameSize-3] = 5
	frame[FrameSize-4] = 99 // wrong
	frame[FrameSize-5] = 5

	_, err := Decode(frame)
	if err != ErrInvalidPadding {
		t.Errorf("error = %v, want ErrInvalidPadding", err)
	}
}

func TestFrameSizeConsistency(t *testing.T) {
	// Both real and dummy results must produce identical frame sizes.
	real := ResultMsg{
		V: ProtocolVersion, Type: TypeResult,
		ID: "a", TS: "t", RefID: "r", Seq: 1,
		HasAction: true,
		Action: Action{
			Command: CmdKick, TargetMemberID: "m",
			GroupID: "g", ReplyText: "kicked", BlockHash: "h",
		},
		Signature: "sig",
	}

	dummy := ResultMsg{
		V: ProtocolVersion, Type: TypeResult,
		ID: "b", TS: "t", RefID: "r", Seq: 2,
		HasAction: false,
		Action: Action{
			Command: CmdNone, TargetMemberID: "",
			GroupID: "", ReplyText: "", BlockHash: "",
		},
		Signature: "sig",
	}

	frameReal, err := Encode(real)
	if err != nil {
		t.Fatalf("Encode(real) error: %v", err)
	}
	frameDummy, err := Encode(dummy)
	if err != nil {
		t.Fatalf("Encode(dummy) error: %v", err)
	}

	if len(frameReal) != len(frameDummy) {
		t.Errorf("real frame = %d bytes, dummy frame = %d bytes - must be equal",
			len(frameReal), len(frameDummy))
	}
}
