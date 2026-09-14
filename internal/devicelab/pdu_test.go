package devicelab

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestDeliverPDUReferenceAndSurrogateBoundaries(t *testing.T) {
	at := time.Date(2026, 9, 14, 22, 30, 45, 0, time.UTC)
	pdus, err := DeliverPDUs("+1234567", "Hi", "one", at)
	if err != nil || len(pdus) != 1 || pdus[0] != "00040791214365f70008629041220354000400480069" {
		t.Fatalf("SMS-DELIVER vector: %v %v", pdus, err)
	}
	body := strings.Repeat("a", 65) + "🙂" + strings.Repeat("b", 80)
	pdus, err = DeliverPDUs("+1234567", body, "multipart", at)
	if err != nil || len(pdus) != 3 {
		t.Fatalf("multipart encoding: %d %v", len(pdus), err)
	}
	var reconstructed strings.Builder
	for i, text := range pdus {
		data, err := hex.DecodeString(text)
		if err != nil {
			t.Fatal(err)
		}
		if data[1] != 0x44 || int(data[17]) > 140 {
			t.Fatal("invalid delivery header or oversized payload")
		}
		user := data[18:]
		if user[0] != 6 || user[1] != 8 || int(user[5]) != len(pdus) || int(user[6]) != i+1 {
			t.Fatalf("invalid concatenation header: %x", user[:7])
		}
		units := []uint16{}
		for pos := 7; pos < len(user); pos += 2 {
			units = append(units, binary.BigEndian.Uint16(user[pos:pos+2]))
		}
		decoded := string(utf16.Decode(units))
		if strings.ContainsRune(decoded, '\ufffd') {
			t.Fatal("surrogate pair split across SMS segments")
		}
		reconstructed.WriteString(decoded)
	}
	if reconstructed.String() != body {
		t.Fatal("multipart SMS text changed")
	}
}
