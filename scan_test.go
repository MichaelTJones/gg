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
