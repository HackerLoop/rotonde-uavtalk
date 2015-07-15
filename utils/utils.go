package utils

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
)

// PrintHex prints the content of the buffer as hex
func PrintHex(buffer []byte, n int) {
	l := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			l += ":"
		}
		l += fmt.Sprintf("%.02x", buffer[i])
	}
	log.Info(l)
}
