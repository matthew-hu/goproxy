package popcount

import (
	`crypto/sha256`
)


var count [256]byte
// count[i] stores the counts of 1 bit in value i
func init() {
	for i := range count {
		count[i] = count[i/2] + byte(i&1)
	}
}


func Count(s1, s2 string) int {
	b1 := []byte(s1)
	b2 := []byte(s2)

	sh1 := sha256.Sum256(b1)
	sh2 := sha256.Sum256(b2)

	n := 0
	for i := range sh1 {
		n += int(count[sh1[i]^sh2[i]])
	}
	return n
}

