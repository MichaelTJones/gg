package main

import (
	"reflect"
	"testing"
)

func Test_visibleWithFlagSet(t *testing.T) {
	*flagVisible = true
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 bool
	}{
		{
			name: "hidden file",
			args: func(*testing.T) args {
				return args{name: ".test"}
			},
			want1: false,
		},

		{
			name: "normal file in hidden folder should not be visible",
			args: func(*testing.T) args {
				return args{name: "/home/user/.config/test.go"}
			},
			want1: false,
		},

		{
			name: "normal file",
			args: func(*testing.T) args {
				return args{name: "test"}
			},
			want1: true,
		},

		{
			name: "go source file",
			args: func(*testing.T) args {
				return args{name: "test.go"}
			},
			want1: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := visible(tArgs.name)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("visible got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}

func Test_visibleWithoutFlagSet(t *testing.T) {
	// flagVisible = false means that we will show results for hidden files
	*flagVisible = false
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 bool
	}{
		{
			name: "hidden file",
			args: func(*testing.T) args {
				return args{name: ".test"}
			},
			want1: true,
		},

		{
			name: "normal file in hidden folder should be visible",
			args: func(*testing.T) args {
				return args{name: "/home/user/.config/test.go"}
			},
			want1: true,
		},

		{
			name: "normal file",
			args: func(*testing.T) args {
				return args{name: "test"}
			},
			want1: true,
		},

		{
			name: "go source file",
			args: func(*testing.T) args {
				return args{name: "test.go"}
			},
			want1: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := visible(tArgs.name)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("visible got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}

func Test_isCompressed(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 bool
	}{
		{
			name: ".bz2 is a valid compression",
			args: func(*testing.T) args {
				return args{name: "test.bz2"}
			},
			want1: true,
		},

		{
			name: ".gz is a valid compression",
			args: func(*testing.T) args {
				return args{name: "test.gz"}
			},
			want1: true,
		},

		{
			name: ".zst is a valid compression",
			args: func(*testing.T) args {
				return args{name: "test.zst"}
			},
			want1: true,
		},

		{
			name: ".go isn't a valid compression",
			args: func(*testing.T) args {
				return args{name: "test.go"}
			},
			want1: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := isCompressed(tArgs.name)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("isCompressed got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}

func Test_isGoWithFlagSet(t *testing.T) {
	*flagGo = true
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 bool
	}{
		{
			name: "go files should pass",
			args: func(*testing.T) args {
				return args{name: "test.go"}
			},
			want1: true,
		},

		{
			name: "zip files should not pass",
			args: func(*testing.T) args {
				return args{name: "test.go.zip"}
			},
			// is this assertion right ?
			want1: false,
		},

		{
			name: "gz files should pass",
			args: func(*testing.T) args {
				return args{name: "test.go.gz"}
			},
			want1: true,
		},

		{
			name: "bz2 files should pass",
			args: func(*testing.T) args {
				return args{name: "test.go.bz2"}
			},
			want1: true,
		},

		{
			name: "zst files should pass",
			args: func(*testing.T) args {
				return args{name: "test.go.zst"}
			},
			want1: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := isGo(tArgs.name)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("isGo got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}

func Test_isGoWithoutFlagSet(t *testing.T) {
	// with this flag set to false our search isn't limited to .go files
	*flagGo = false
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 bool
	}{
		{
			name: "go files should pass",
			args: func(*testing.T) args {
				return args{name: "test.go"}
			},
			want1: true,
		},

		{
			name: "zipped go files should pass",
			args: func(*testing.T) args {
				return args{name: "test.go.zip"}
			},
			want1: true,
		},

		{
			name: "anything should pass when flagGo = false",
			args: func(*testing.T) args {
				return args{name: "test.zip.exe"}
			},
			want1: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := isGo(tArgs.name)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("isGo got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}

func Test_isArchive(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 bool
	}{
		{
			name: "tar is a valid archive format",
			args: func(*testing.T) args {
				return args{name: "test.tar"}
			},
			want1: true,
		},

		{
			name: "zip is a valid archive format",
			args: func(*testing.T) args {
				return args{name: "test.zip"}
			},
			want1: true,
		},

		{
			name: "cpio is a valid archive format",
			args: func(*testing.T) args {
				return args{name: "test.cpio"}
			},
			want1: true,
		},

		{
			name: "cpio.bz2 is a valid archive format",
			args: func(*testing.T) args {
				return args{name: "test.cpio.bz2"}
			},
			want1: true,
		},

		{
			name: "cpio.exe isn't a valid archive format",
			args: func(*testing.T) args {
				return args{name: "test.cpio.exe"}
			},
			want1: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := isArchive(tArgs.name)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("isArchive got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}

func Test_parseFirstArg(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 searchMode
	}{
		{
			name: "'a' should include all",
			args: func(*testing.T) args {
				return args{input: "a"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: true,
				N: true,
				O: true,
				P: true,
				R: true,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'c' should include only comments",
			args: func(*testing.T) args {
				return args{input: "c"}
			},
			want1: searchMode{
				C: true,
			},
		},

		{
			name: "'aC' should only exclude comments",
			args: func(*testing.T) args {
				return args{input: "aC"}
			},
			want1: searchMode{
				C: false,
				D: true,
				I: true,
				K: true,
				N: true,
				O: true,
				P: true,
				R: true,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'d' should include only defined non-types",
			args: func(*testing.T) args {
				return args{input: "d"}
			},
			want1: searchMode{
				D: true,
			},
		},

		{
			name: "'aD' should only exclude defined non-types",
			args: func(*testing.T) args {
				return args{input: "aD"}
			},
			want1: searchMode{
				C: true,
				D: false,
				I: true,
				K: true,
				N: true,
				O: true,
				P: true,
				R: true,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'i' should include only identifiers",
			args: func(*testing.T) args {
				return args{input: "i"}
			},
			want1: searchMode{
				I: true,
			},
		},

		{
			name: "'aI' should only exclude identifiers",
			args: func(*testing.T) args {
				return args{input: "aI"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: false,
				K: true,
				N: true,
				O: true,
				P: true,
				R: true,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'k' should include only keywords",
			args: func(*testing.T) args {
				return args{input: "k"}
			},
			want1: searchMode{
				K: true,
			},
		},

		{
			name: "'aK' should only exclude keywords",
			args: func(*testing.T) args {
				return args{input: "aK"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: false,
				N: true,
				O: true,
				P: true,
				R: true,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'n' should include only numbers",
			args: func(*testing.T) args {
				return args{input: "n"}
			},
			want1: searchMode{
				N: true,
			},
		},

		{
			name: "'aN' should only exclude numbers",
			args: func(*testing.T) args {
				return args{input: "aN"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: true,
				N: false,
				O: true,
				P: true,
				R: true,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'o' should include only operators",
			args: func(*testing.T) args {
				return args{input: "o"}
			},
			want1: searchMode{
				O: true,
			},
		},

		{
			name: "'aO' should only exclude operators",
			args: func(*testing.T) args {
				return args{input: "aO"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: true,
				N: true,
				O: false,
				P: true,
				R: true,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'p' should include only package names",
			args: func(*testing.T) args {
				return args{input: "p"}
			},
			want1: searchMode{
				P: true,
			},
		},

		{
			name: "'aP' should only exclude package names",
			args: func(*testing.T) args {
				return args{input: "aP"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: true,
				N: true,
				O: true,
				P: false,
				R: true,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'r' should include only rune literals",
			args: func(*testing.T) args {
				return args{input: "r"}
			},
			want1: searchMode{
				R: true,
			},
		},

		{
			name: "'aR' should only exclude rune literals",
			args: func(*testing.T) args {
				return args{input: "aR"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: true,
				N: true,
				O: true,
				P: true,
				R: false,
				S: true,
				T: true,
				V: true,
			},
		},

		{
			name: "'s' should include only strings",
			args: func(*testing.T) args {
				return args{input: "s"}
			},
			want1: searchMode{
				S: true,
			},
		},

		{
			name: "'aS' should only exclude strings",
			args: func(*testing.T) args {
				return args{input: "aS"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: true,
				N: true,
				O: true,
				P: true,
				R: true,
				S: false,
				T: true,
				V: true,
			},
		},

		{
			name: "'t' should include only types",
			args: func(*testing.T) args {
				return args{input: "t"}
			},
			want1: searchMode{
				T: true,
			},
		},

		{
			name: "'aT' should only exclude types",
			args: func(*testing.T) args {
				return args{input: "aT"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: true,
				N: true,
				O: true,
				P: true,
				R: true,
				S: true,
				T: false,
				V: true,
			},
		},

		{
			name: "'v' should include only numeric values",
			args: func(*testing.T) args {
				return args{input: "v"}
			},
			want1: searchMode{
				V: true,
			},
		},

		{
			name: "'aV' should only exclude numeric values",
			args: func(*testing.T) args {
				return args{input: "aV"}
			},
			want1: searchMode{
				C: true,
				D: true,
				I: true,
				K: true,
				N: true,
				O: true,
				P: true,
				R: true,
				S: true,
				T: true,
				V: false,
			},
		},

		{
			name: "'g' should be grep mode",
			args: func(*testing.T) args {
				return args{input: "g"}
			},
			want1: searchMode{
				G: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := parseFirstArg(tArgs.input)

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("parseFirstArg got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}
