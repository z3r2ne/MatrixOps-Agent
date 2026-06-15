package token

import "math"

func Estimate(text string) int {
	if text == "" {
		return 0
	}
	runes := []rune(text)
	return int(math.Ceil(float64(len(runes)) / 4.0))
}
