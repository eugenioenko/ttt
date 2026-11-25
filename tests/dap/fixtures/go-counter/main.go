package main

import "fmt"

func increment(value int) int {
	return value + 1
}
func main() {
	count := 41
	count++
	count = increment(count)
	fmt.Println("count =", count)
}
