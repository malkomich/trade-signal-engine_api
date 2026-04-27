package rtdb

import (
	"testing"
	"time"
)

func TestSafeKeyPartReplacesAllForbiddenCharacters(t *testing.T) {
	got := SafeKeyPart("A.B#C$D[E]F/G")
	if got != "A_B_C_D_E_F_G" {
		t.Fatalf("unexpected sanitized key part: %q", got)
	}
}

func TestSafeTimestampKeyUsesUnixNanoDecimal(t *testing.T) {
	got := SafeTimestampKey(time.Date(2026, 4, 27, 19, 42, 1, 123456789, time.UTC))
	if got != "1777318921123456789" {
		t.Fatalf("unexpected timestamp key: %q", got)
	}
}
