package word

import `unicode`
import `bufio`
import `io`
import `net`
import `net/http`
import `net/url`

func IsPalindrome(s string) bool {
	var letters = make([]rune, 0, len(s))
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters = append(letters, unicode.ToLower(r))
		}
	}

	for i, j := 0, len(letters)-1; i < j; i, j = i+1, j-1 {
		if letters[i] != letters[j] {
			return false
		}
	}
	return true
}
