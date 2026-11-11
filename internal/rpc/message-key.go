package rpc

import "crypto/sha256"

// MessageKey identifies the original event, before decoding or database timestamp
// rounding. Including the subject isolates clients sharing one JetStream stream.
func MessageKey(subject string, payload []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte("svarog:log:v1\x00"))
	h.Write([]byte(subject))
	h.Write([]byte{0})
	h.Write(payload)
	return [32]byte(h.Sum(nil))
}
