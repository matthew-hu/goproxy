package hash

import (
	`crypto/sha256`
	`crypto/sha512`
	`fmt`
	"strings"
)

func Hashcode(b []byte, algo string) string {
	switch {
		case strings.Index(algo, "384") != -1:
			return fmt.Sprintf("%x", sha512.Sum384(b))
		case strings.Index(algo, "512") != -1:
			return fmt.Sprintf("%x", sha512.Sum512(b))
		default:
			return fmt.Sprintf("%x", sha256.Sum256(b))
	}
}
