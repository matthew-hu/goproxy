package main

import (
	`fmt`
	`bufio`
	`os`
)

func main() {
	freq := make(map[string]int)

	input := bufio.NewScanner(os.Stdin)
	input.Split(bufio.ScanWords)
	for input.Scan() {
		freq[input.Text()]++
	}

	if err := input.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "wordfreq: %v", err)
	}
	for word, count := range freq {
		fmt.Println(word, count)
	}
}
