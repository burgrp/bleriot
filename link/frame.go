package link

import "errors"

// MsgType identifies a control/data message on the host↔modem link.
type MsgType byte

// A modem manages exactly one radio and is reached over its own serial port.
// The host fans out across several modems (one per port); that multiplexing
// lives above this package, so no radio index appears on the wire.
const (
	// MsgHello (modem → host) announces the modem on boot. Body: [Version],
	// the link protocol version, so the host can detect a firmware mismatch.
	MsgHello MsgType = 0x01

	// MsgConfigRadio (host → modem) configures the radio's channel and receive
	// address. Body: [Channel][Addr0..3].
	MsgConfigRadio MsgType = 0x02

	// MsgSend (host → modem) transmits one BleRiot packet to a destination
	// address. Body: [Dst0..3][Payload...].
	MsgSend MsgType = 0x03

	// MsgRecv (modem → host) delivers one received BleRiot packet.
	// Body: [Payload...].
	MsgRecv MsgType = 0x04

	// MsgError (modem → host) reports a modem-side error. Body: [Code].
	MsgError MsgType = 0x05
)

// ProtocolVersion is the current link protocol version, carried by MsgHello.
const ProtocolVersion byte = 0x01

// Error codes carried by MsgError.
const (
	ErrNone        byte = 0x00
	ErrTxFailed    byte = 0x01 // radio reported transmit failure
	ErrBadFrame    byte = 0x02 // malformed/short command frame
	ErrUnknownType byte = 0x03 // unrecognised MsgType
)

// AddrLen is the BleRiot device address length in bytes (§3).
const AddrLen = 4

var (
	errShortBody   = errors.New("link: message body too short")
	errUnknownType = errors.New("link: unknown message type")
)

// Message is a decoded host↔modem message. Only the fields relevant to Type are
// populated. Payload, when present, aliases the decoder's internal buffer and is
// only valid until the next decode; copy it if you need to retain it.
type Message struct {
	Type    MsgType
	Version uint8
	Channel uint8
	Addr    [AddrLen]byte
	Payload []byte
	Code    uint8
}

// AppendMessage marshals msg's body and appends a complete COBS frame
// (encoded body + Delimiter) to dst, returning the extended slice.
func AppendMessage(dst []byte, msg Message) []byte {
	var body []byte
	switch msg.Type {
	case MsgHello:
		body = []byte{byte(msg.Type), msg.Version}
	case MsgConfigRadio:
		body = []byte{byte(msg.Type), msg.Channel,
			msg.Addr[0], msg.Addr[1], msg.Addr[2], msg.Addr[3]}
	case MsgSend:
		body = make([]byte, 0, 5+len(msg.Payload))
		body = append(body, byte(msg.Type),
			msg.Addr[0], msg.Addr[1], msg.Addr[2], msg.Addr[3])
		body = append(body, msg.Payload...)
	case MsgRecv:
		body = make([]byte, 0, 1+len(msg.Payload))
		body = append(body, byte(msg.Type))
		body = append(body, msg.Payload...)
	case MsgError:
		body = []byte{byte(msg.Type), msg.Code}
	default:
		body = []byte{byte(msg.Type)}
	}

	dst = cobsEncode(dst, body)
	return append(dst, Delimiter)
}

// parseMessage decodes a message body (already COBS-decoded, no delimiter).
// The returned Message.Payload aliases body.
func parseMessage(body []byte) (Message, error) {
	if len(body) < 1 {
		return Message{}, errShortBody
	}
	m := Message{Type: MsgType(body[0])}
	switch m.Type {
	case MsgHello:
		if len(body) < 2 {
			return Message{}, errShortBody
		}
		m.Version = body[1]
	case MsgConfigRadio:
		if len(body) < 6 {
			return Message{}, errShortBody
		}
		m.Channel = body[1]
		copy(m.Addr[:], body[2:6])
	case MsgSend:
		if len(body) < 5 {
			return Message{}, errShortBody
		}
		copy(m.Addr[:], body[1:5])
		m.Payload = body[5:]
	case MsgRecv:
		m.Payload = body[1:]
	case MsgError:
		if len(body) < 2 {
			return Message{}, errShortBody
		}
		m.Code = body[1]
	default:
		return Message{}, errUnknownType
	}
	return m, nil
}

// Decoder reassembles messages from a byte stream one byte at a time. It holds
// no external dependencies and pre-allocates its buffers, so it is suitable for
// the TinyGo firmware as well as the host. It is not safe for concurrent use.
type Decoder struct {
	raw     []byte // accumulating COBS frame (no delimiter yet)
	decoded []byte // scratch for the decoded body
	maxRaw  int
}

// NewDecoder creates a Decoder that accepts COBS frames up to maxFrame bytes
// (the encoded size, excluding the delimiter). Frames exceeding this are
// discarded to bound memory.
func NewDecoder(maxFrame int) *Decoder {
	if maxFrame <= 0 {
		maxFrame = 64
	}
	return &Decoder{
		raw:     make([]byte, 0, maxFrame),
		decoded: make([]byte, 0, maxFrame),
		maxRaw:  maxFrame,
	}
}

// Push feeds one byte into the decoder. When a complete frame has been received,
// it returns the decoded Message and ok=true. A decode error (malformed frame)
// is reported via err with ok=false; the decoder resynchronises automatically on
// the next delimiter. The returned Message.Payload aliases internal storage and
// is only valid until the next Push.
func (d *Decoder) Push(b byte) (msg Message, ok bool, err error) {
	if b != Delimiter {
		if len(d.raw) < d.maxRaw {
			d.raw = append(d.raw, b)
		} else {
			// Overlong frame: mark the buffer poisoned so the frame is dropped
			// when its delimiter finally arrives.
			d.raw = d.raw[:0]
			d.raw = append(d.raw, 0xFF) // a lone 0xFF decodes/parses to an error
		}
		return Message{}, false, nil
	}

	// Delimiter: an empty frame is just idle/keepalive — ignore it.
	if len(d.raw) == 0 {
		return Message{}, false, nil
	}

	d.decoded = d.decoded[:0]
	d.decoded, err = cobsDecode(d.decoded, d.raw)
	d.raw = d.raw[:0]
	if err != nil {
		return Message{}, false, err
	}
	msg, err = parseMessage(d.decoded)
	if err != nil {
		return Message{}, false, err
	}
	return msg, true, nil
}

// Reset clears any partial frame, e.g. after a transport reconnect.
func (d *Decoder) Reset() { d.raw = d.raw[:0] }
