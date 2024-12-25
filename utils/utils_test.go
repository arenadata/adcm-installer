package utils

import (
	"testing"
)

func TestGenerateRandomString(t *testing.T) {
	const length = 42
	t.Run("RandomStringLength", func(t *testing.T) {
		if got := GenerateRandomString(length); len(got) != length {
			t.Errorf("GenerateRandomString() length = %v, want %v", got, length)
		}
	})
}
