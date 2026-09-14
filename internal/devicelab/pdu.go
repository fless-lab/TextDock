package devicelab

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
	"unicode/utf16"

	"github.com/fless-lab/TextDock/internal/message"
)

// DeliverPDUs creates incoming GSM SMS-DELIVER PDUs with UTF-16BE user data.
// Explicit surrogate pairs avoid non-BMP truncation in the emulator's `sms send`
// convenience command. Only hexadecimal data reaches the emulator console.
func DeliverPDUs(sender, body, messageID string, at time.Time) ([]string, error) {
	if !message.ValidRecipient(sender) {
		return nil, ErrInput
	}
	if err := ValidateBody(body); err != nil {
		return nil, err
	}
	units := utf16.Encode([]rune(body))
	chunks := [][]uint16{units}
	if len(units) > 70 {
		chunks = nil
		// 16-bit concatenation UDH: 7 bytes, leaving 66 complete UTF-16 units.
		for len(units) > 0 {
			n := min(66, len(units))
			if n < len(units) && units[n-1] >= 0xd800 && units[n-1] <= 0xdbff {
				n--
			}
			chunks = append(chunks, units[:n])
			units = units[n:]
		}
	}
	digits := sender[1:]
	address := make([]byte, (len(digits)+1)/2)
	for i := range address {
		low := digits[i*2] - '0'
		high := byte(0xf)
		if i*2+1 < len(digits) {
			high = digits[i*2+1] - '0'
		}
		address[i] = low | high<<4
	}
	at = at.UTC()
	stamp := []byte{swappedBCD(at.Year() % 100), swappedBCD(int(at.Month())), swappedBCD(at.Day()), swappedBCD(at.Hour()), swappedBCD(at.Minute()), swappedBCD(at.Second()), 0}
	ref := sha256.Sum256([]byte(messageID))
	out := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		flags := byte(0x04)
		var payload []byte
		if len(chunks) > 1 {
			flags |= 0x40
			payload = []byte{6, 8, 4, ref[0], ref[1], byte(len(chunks)), byte(i + 1)}
		}
		for _, unit := range chunk {
			payload = append(payload, byte(unit>>8), byte(unit))
		}
		pdu := []byte{0, flags, byte(len(digits)), 0x91}
		pdu = append(pdu, address...)
		pdu = append(pdu, 0, 8)
		pdu = append(pdu, stamp...)
		pdu = append(pdu, byte(len(payload)))
		pdu = append(pdu, payload...)
		out = append(out, hex.EncodeToString(pdu))
	}
	return out, nil
}
func swappedBCD(value int) byte { return byte(value/10) | byte(value%10)<<4 }
