package utils

import (
	"testing"
)

func TestGenerateRandomString(t *testing.T) {
	type args struct {
		length int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateRandomString(tt.args.length); got != tt.want {
				t.Errorf("GenerateRandomString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIn(t *testing.T) {
	type args struct {
		a []string
		s string
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
			if got := In(tt.args.a, tt.args.s); got != tt.want {
				t.Errorf("In() = %v, want %v", got, tt.want)
			}
		})
	}
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

//func TestPtr(t *testing.T) {
//	type args[T comparable] struct {
//		v T
//	}
//	type testCase[T comparable] struct {
//		name string
//		args args[T]
//		want *T
//	}
//	tests := []testCase[ /* TODO: Insert concrete types here */ ]{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if got := Ptr(tt.args.v); !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("Ptr() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}

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
