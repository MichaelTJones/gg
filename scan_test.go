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
