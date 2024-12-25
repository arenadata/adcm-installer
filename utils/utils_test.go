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

//func TestPtrIsEmpty(t *testing.T) {
//	type args struct {
//		ptr any
//	}
//
//	tests := []struct {
//		name string
//		args args
//		want bool
//	}{
//		{"", args{nil}, true},
//		{"", args{Ptr(0)}, true},
//		{"", args{Ptr(-1)}, false},
//		{"", args{Ptr(1)}, false},
//		{"", args{Ptr(uint16(0))}, true},
//		{"", args{Ptr(uint16(1))}, false},
//		{"", args{Ptr("")}, true},
//		{"", args{Ptr("true")}, false},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if got := PtrIsEmpty(tt.args.ptr); got != tt.want {
//				t.Errorf("PtrIsEmpty() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}

func Test_ptrIsEmpty(t *testing.T) {
	type args[T comparable] struct {
		v *T
	}
	type testCase[T comparable] struct {
		name string
		args args[T]
		want bool
	}

	for _, tt := range []testCase[uint16]{
		{"Uint16Zero", args[uint16]{Ptr(uint16(0))}, true},
		{"Uint16One", args[uint16]{Ptr(uint16(1))}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := PtrIsEmpty(tt.args.v); got != tt.want {
				t.Errorf("ptrIsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}

	for _, tt := range []testCase[int]{
		{"IntZero", args[int]{Ptr(0)}, true},
		{"IntMinusOne", args[int]{Ptr(-1)}, false},
		{"IntOne", args[int]{Ptr(1)}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := PtrIsEmpty(tt.args.v); got != tt.want {
				t.Errorf("ptrIsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}

	for _, tt := range []testCase[string]{
		{"StringEmpty", args[string]{Ptr("")}, true},
		{"StringNonEmpty", args[string]{Ptr("true")}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := PtrIsEmpty(tt.args.v); got != tt.want {
				t.Errorf("ptrIsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}

	type str struct {
		field string
	}

	var emptyStr *str
	var nonEmptyStruct = &str{field: "abc"}
	for _, tt := range []testCase[str]{
		{"StructEmpty-1", args[str]{emptyStr}, true},
		{"StructEmpty-2", args[str]{&str{}}, true},
		{"StructNonEmpty", args[str]{nonEmptyStruct}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := PtrIsEmpty(tt.args.v); got != tt.want {
				t.Errorf("ptrIsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
