package utils

import "fmt"

func printHex(buffer []byte, n int) {
	fmt.Println("start packet:")
	for i := 0; i < n; i++ {
		if i > 0 {
			fmt.Print(":")
		}
		fmt.Printf("%.02x", buffer[i])
	}
	fmt.Println("\nend packet")
}
