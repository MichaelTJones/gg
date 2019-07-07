package main

import (
	"reflect"
	"runtime"
	"testing"
)

func Test_getMaxCPU(t *testing.T) {
	actualProcs := runtime.NumCPU()
	tests := []struct {
		name    string
		rcvdVal int
		want1   int
	}{
		{
			name:    "0 should use all CPUs",
			rcvdVal: 0,
			want1:   actualProcs,
		},

		{
			name:    "negative number should use all but x CPUs",
			rcvdVal: -2,
			want1:   actualProcs - 2,
		},

		{
			name:    "should use at least 1 CPU",
			rcvdVal: -1 * (actualProcs + 2),
			want1:   2,
		},

		{
			name:    "should use 2 workers even if only 1 CPU requested",
			rcvdVal: 1,
			want1:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*flagCPUs = tt.rcvdVal
			got1 := getMaxCPU()

			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("getMaxCPU got1 = %v, want1: %v", got1, tt.want1)
			}
		})
	}
}
