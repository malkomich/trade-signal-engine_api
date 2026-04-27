package rtdb

import (
	"strconv"
	"strings"
	"time"
)

var keyPartReplacer = strings.NewReplacer(
	".", "_",
	"#", "_",
	"$", "_",
	"[", "_",
	"]", "_",
	"/", "_",
)

func SafeKeyPart(value string) string {
	return keyPartReplacer.Replace(value)
}

func SafeTimestampKey(value time.Time) string {
	return strconv.FormatInt(value.UTC().UnixNano(), 10)
}
