package pgd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInt32(t *testing.T) {
	tests := []struct {
		in      string
		want    int32
		wantErr bool
	}{
		{in: "0", want: 0},
		{in: "100", want: 100},
		{in: "-100", want: -100},
		{in: "2147483647", want: 2147483647},   // max int32
		{in: "-2147483648", want: -2147483648}, // min int32
		{in: "2147483648", wantErr: true},      // overflows int32
		{in: "-2147483649", wantErr: true},     // underflows int32
		{in: "abc", wantErr: true},
		{in: "", wantErr: true},
		{in: "1.5", wantErr: true},
		{in: " 5 ", wantErr: true}, // whitespace not stripped
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseInt32(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
