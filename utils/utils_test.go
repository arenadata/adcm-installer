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

func TestIsPath(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Empty", args{""}, true},
		{"CurrentPath", args{"."}, true},
		{"CurrentPathWithLeadingPoint", args{"./"}, true},
		{"RelativePathWithLeadingPoint", args{"./path/to/volume"}, true},
		{"RelativePathWithTrailingSlash", args{"path/to/volume/"}, true},
		{"RootPath", args{"/"}, true},
		{"AbsolutePath", args{"/path/to/volume"}, true},
		{"NamedVolume", args{"adcm"}, false},
		{"NamedVolumeWithPoint", args{"adcm.pg"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPath(tt.args.s); got != tt.want {
				t.Errorf("IsPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPtrIsEmpty(t *testing.T) {
	type args struct {
		v any
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PtrIsEmpty(tt.args.v); got != tt.want {
				t.Errorf("PtrIsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
