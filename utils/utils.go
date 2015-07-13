package utils

import "fmt"

// PrintHex prints the content of the buffer as hex
func PrintHex(buffer []byte, n int) {
	fmt.Println("start packet:")
	for i := 0; i < n; i++ {
		if i > 0 {
			fmt.Print(":")
		}
		fmt.Printf("%.02x", buffer[i])
	}
	fmt.Println("\nend packet")
}
