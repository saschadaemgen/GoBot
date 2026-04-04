// Package protocol defines the GoKey Wire Protocol message types
// and constants. See docs/GOKEY-WIRE-PROTOCOL.md v0.2.0 for the
// full specification.
package protocol

// Protocol constants from the wire protocol specification.
const (
	FrameSize            = 24576 // bytes (24 KB)
	ProtocolVersion      = 1
	HeartbeatInterval    = 30  // seconds
	HeartbeatTimeout     = 10  // seconds
	MaxMissedPongs       = 3
	TimestampWindow      = 30  // seconds
	DriftWarnThreshold   = 15  // seconds
	DriftAlertThreshold  = 25  // seconds
	ResponseDelayMin     = 100 // milliseconds
	ResponseDelayMax     = 500 // milliseconds
	BufferMaxBlocks      = 1000
	BufferMaxAge         = 300 // seconds (5 minutes)
	AckTimeout           = 5   // seconds
	MaxRetransmits       = 3
	HandshakeTimeout     = 5   // seconds
	ReconnectBase        = 1   // seconds
	ReconnectMax         = 30  // seconds
	NTPResyncInterval    = 21600 // seconds (6 hours)
	WSSPortDefault       = 6000
	ReplyTextMaxLength   = 2048 // characters
)

// Message type identifiers.
const (
	TypeBlock   = "block"
	TypeResult  = "result"
	TypePing    = "ping"
	TypePong    = "pong"
	TypeAck     = "ack"
	TypeError   = "error"
	TypeHello   = "hello"
	TypeWelcome = "welcome"
)

// Command identifiers for action.command field.
const (
	CmdKick  = "kick"
	CmdBan   = "ban"
	CmdMute  = "mute"
	CmdBlock = "block"
	CmdWarn  = "warn"
	CmdReply = "reply"
	CmdNone  = "none"
)

// Sender contains metadata about the message sender,
// extracted from the SMP frame by GoBot.
type Sender struct {
	MemberID    string `json:"member_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	ContactID   string `json:"contact_id"`
}

// Action represents a command result from GoKey.
// Both real commands and dummy responses use this structure
// with identical field count for constant-size framing.
type Action struct {
	Command        string `json:"command"`
	TargetMemberID string `json:"target_member_id"`
	GroupID        string `json:"group_id"`
	ReplyText      string `json:"reply_text"`
	BlockHash      string `json:"block_hash"`
}

// --- GoBot -> GoKey messages ---

// BlockMsg is sent when GoBot receives an encrypted block
// from an SMP queue and forwards it to GoKey for decryption.
type BlockMsg struct {
	V       int    `json:"v"`
	Type    string `json:"type"`
	ID      string `json:"id"`
	TS      string `json:"ts"`
	QueueID string `json:"queue_id"`
	GroupID string `json:"group_id"`
	Sender  Sender `json:"sender"`
	Payload string `json:"payload"`
}

// PingMsg is a heartbeat sent by GoBot every 30 seconds.
type PingMsg struct {
	V    int    `json:"v"`
	Type string `json:"type"`
	ID   string `json:"id"`
	TS   string `json:"ts"`
	Seq  int64  `json:"seq"`
}

// AckMsg acknowledges receipt and validation of a message.
// Used by both GoBot and GoKey.
type AckMsg struct {
	V     int    `json:"v"`
	Type  string `json:"type"`
	ID    string `json:"id"`
	TS    string `json:"ts"`
	RefID string `json:"ref_id"`
}

// WelcomeMsg is GoBot's response to GoKey's hello handshake.
type WelcomeMsg struct {
	V               int    `json:"v"`
	Type            string `json:"type"`
	ID              string `json:"id"`
	TS              string `json:"ts"`
	RefID           string `json:"ref_id"`
	AcceptedVersion int    `json:"accepted_version"`
	ServerTime      string `json:"server_time"`
}

// --- GoKey -> GoBot messages ---

// HelloMsg is the first message sent by GoKey after the
// WSS/mTLS connection is established.
type HelloMsg struct {
	V                int      `json:"v"`
	Type             string   `json:"type"`
	ID               string   `json:"id"`
	TS               string   `json:"ts"`
	ProtocolVersion  int      `json:"protocol_version"`
	PubkeyFprint     string   `json:"pubkey_fingerprint"`
	LastSeq          int64    `json:"last_seq"`
	FirmwareVersion  string   `json:"firmware_version"`
	KnownQueues      []string `json:"known_queues"`
}

// ResultMsg is GoKey's response to every block message.
// Contains either a real command result or an indistinguishable
// dummy. Both use identical structure for constant-size framing.
type ResultMsg struct {
	V         int    `json:"v"`
	Type      string `json:"type"`
	ID        string `json:"id"`
	TS        string `json:"ts"`
	RefID     string `json:"ref_id"`
	Seq       int64  `json:"seq"`
	HasAction bool   `json:"has_action"`
	Action    Action `json:"action"`
	Signature string `json:"signature"`
}

// PongMsg is GoKey's response to a heartbeat ping.
type PongMsg struct {
	V     int    `json:"v"`
	Type  string `json:"type"`
	ID    string `json:"id"`
	TS    string `json:"ts"`
	RefID string `json:"ref_id"`
	Seq   int64  `json:"seq"`
}

// ErrorMsg is a signed error from GoKey. Errors are signed to
// prevent a compromised VPS from injecting spoofed errors.
type ErrorMsg struct {
	V         int    `json:"v"`
	Type      string `json:"type"`
	ID        string `json:"id"`
	TS        string `json:"ts"`
	RefID     string `json:"ref_id"`
	Seq       int64  `json:"seq"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

// Error codes from the wire protocol specification.
const (
	ErrDecryptFailed  = "DECRYPT_FAILED"
	ErrInvalidFormat  = "INVALID_FORMAT"
	ErrPayloadTooLarge = "PAYLOAD_TOO_LARGE"
	ErrSeqMismatch    = "SEQ_MISMATCH"
	ErrSigInvalid     = "SIG_INVALID"
	ErrTimestampExp   = "TIMESTAMP_EXPIRED"
	ErrRatchetError   = "RATCHET_ERROR"
	ErrNVSWriteFailed = "NVS_WRITE_FAILED"
	ErrUnknownQueue   = "UNKNOWN_QUEUE"
	ErrRateLimited    = "RATE_LIMITED"
	ErrVersionMismatch = "VERSION_MISMATCH"
	ErrPubkeyMismatch = "PUBKEY_MISMATCH"
	ErrSeqRollback    = "SEQ_ROLLBACK"
)
