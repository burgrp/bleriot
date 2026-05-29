// Package link defines the COBS-framed serial protocol spoken between the host
// hub and the MCU "dumb radio modem" (UART now, USB-CDC later).
//
// The package is deliberately dependency-free and allocation-friendly so the
// same source compiles unchanged into the TinyGo firmware. Frames are delimited
// by a single zero byte; payloads are COBS-encoded so they never contain a zero,
// which gives unambiguous frame boundaries and cheap resynchronisation after
// line noise.
package link

import "errors"

// Delimiter terminates every COBS frame on the wire. It never appears inside an
// encoded frame.
const Delimiter byte = 0x00

var (
	errZeroCode = errors.New("link: zero code byte in COBS data")
	errOverrun  = errors.New("link: COBS block overruns input")
)

// cobsEncode appends the COBS encoding of src to dst and returns the extended
// slice. The result never contains a zero byte. It does NOT append the frame
// delimiter; callers add Delimiter themselves.
func cobsEncode(dst, src []byte) []byte {
	codeIdx := len(dst)
	dst = append(dst, 0) // reserve the first code byte
	code := byte(1)

	for _, b := range src {
		if b != 0 {
			dst = append(dst, b)
			code++
		}
		if b == 0 || code == 0xFF {
			dst[codeIdx] = code
			codeIdx = len(dst)
			dst = append(dst, 0) // reserve the next code byte
			code = 1
		}
	}
	dst[codeIdx] = code
	return dst
}

// cobsDecode appends the decoding of a single COBS frame (without the trailing
// Delimiter) to dst and returns the extended slice. It returns an error if the
// input is malformed.
func cobsDecode(dst, src []byte) ([]byte, error) {
	i := 0
	for i < len(src) {
		code := src[i]
		if code == 0 {
			return nil, errZeroCode
		}
		i++
		end := i + int(code) - 1
		if end > len(src) {
			return nil, errOverrun
		}
		for ; i < end; i++ {
			dst = append(dst, src[i])
		}
		// An implicit zero separates blocks, except after a 0xFF block and
		// except at the very end of the frame.
		if code != 0xFF && i < len(src) {
			dst = append(dst, 0)
		}
	}
	return dst, nil
}
