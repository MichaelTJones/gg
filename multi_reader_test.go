package main

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func sameMultiReader(a, b *multiReader) bool {
	if a.ext != b.ext {
		return false
	}
	if a.zipIndex != b.zipIndex {
		return false
	}
	return true
}

func Test_newMultiReader(t *testing.T) {
	type args struct {
		r    io.Reader
		ext  string
		name string
	}
	tests := []struct {
		name string
		args func(t *testing.T) args

		want1 *multiReader
	}{
		{
			name: "wrong extension should yeld empty multiReader",
			args: func(*testing.T) args {
				var r *bytes.Buffer
				return args{
					r:    r,
					ext:  "asd",
					name: "",
				}
			},
			want1: &multiReader{},
		},

		{
			name: "cpio extension should create a cpio multiReader",
			args: func(*testing.T) args {
				var r *bytes.Buffer
				return args{
					r:    r,
					ext:  ".cpio",
					name: "",
				}
			},
			want1: &multiReader{ext: eCPIO},
		},

		{
			name: "tar extension should create a tar multiReader",
			args: func(*testing.T) args {
				var r *bytes.Buffer
				return args{
					r:    r,
					ext:  ".tar",
					name: "",
				}
			},
			want1: &multiReader{ext: eTAR},
		},

		{
			name: "zip extension should create a zip multiReader",
			args: func(*testing.T) args {
				var r *bytes.Buffer
				return args{
					r:    r,
					ext:  ".zip",
					name: "testdata/source.zip",
				}
			},
			want1: &multiReader{ext: eZIP, zipIndex: -1},
		},

		{
			name: "zip should return empty mutiReader if file doesn't exists",
			args: func(*testing.T) args {
				var r *bytes.Buffer
				return args{
					r:    r,
					ext:  ".zip",
					name: "invalid.zip",
				}
			},
			want1: &multiReader{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := newMultiReader(tArgs.r, tArgs.ext, tArgs.name)

			if !sameMultiReader(got1, tt.want1) {
				t.Errorf("newMultiReader got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}

func Test_multiReader_Next(t *testing.T) {
	zipMR := newMultiReader(&bytes.Buffer{}, ".zip", "testdata/source.zip")
	tests := []struct {
		name    string
		init    func(t *testing.T) *multiReader
		inspect func(r *multiReader, t *testing.T)

		want1      string
		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}{
		{
			name:    "we should find our files in the zip",
			init:    func(*testing.T) *multiReader { return zipMR },
			want1:   "main.go",
			wantErr: false,
		},

		{
			name:    "we should find our files in the zip",
			init:    func(*testing.T) *multiReader { return zipMR },
			want1:   "main_test.go",
			wantErr: false,
		},

		{
			name:    "we should find our files in the zip",
			init:    func(*testing.T) *multiReader { return zipMR },
			want1:   "scan.go",
			wantErr: false,
		},

		{
			name:    "we should find our files in the zip",
			init:    func(*testing.T) *multiReader { return zipMR },
			want1:   "scan_test.go",
			wantErr: false,
		},

		{
			name:    "at the end we should get an io.EOF",
			init:    func(*testing.T) *multiReader { return zipMR },
			want1:   "",
			wantErr: true,
			inspectErr: func(err error, t *testing.T) {
				if err != io.EOF {
					t.Errorf("expected io.EOF err, got: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receiver := tt.init(t)
			got1, err := receiver.Next()

			if tt.inspect != nil {
				tt.inspect(receiver, t)
			}

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("multiReader.Next got1 = %v, want1: %v", got1, tt.want1)
			}

			if (err != nil) != tt.wantErr {
				t.Fatalf("multiReader.Next error = %v, wantErr: %t", err, tt.wantErr)
			}

			if tt.inspectErr != nil {
				tt.inspectErr(err, t)
			}
		})
	}
}
