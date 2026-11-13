package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePurchase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCost float64
		wantDesc string
		wantErr  bool
	}{
		{name: "integer price and description", input: "100 Coffee", wantCost: 100, wantDesc: "Coffee"},
		{name: "decimal dot and multiword description", input: "150.50 Taxi to hotel", wantCost: 150.50, wantDesc: "Taxi to hotel"},
		{name: "comma decimal", input: "42,75 Lunch", wantCost: 42.75, wantDesc: "Lunch"},
		{name: "extra whitespace collapsed", input: "  99.99   Pizza   and soda  ", wantCost: 99.99, wantDesc: "Pizza and soda"},
		{name: "price only defaults description", input: "250", wantCost: 250, wantDesc: "No description"},
		{name: "empty input", input: "   ", wantErr: true},
		{name: "zero price", input: "0 Coffee", wantErr: true},
		{name: "negative price", input: "-10 Coffee", wantErr: true},
		{name: "NaN rejected", input: "NaN Coffee", wantErr: true},
		{name: "Inf rejected", input: "Inf Coffee", wantErr: true},
		{name: "non-numeric", input: "abc Coffee", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cost, desc, err := ParsePurchase(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tc.wantCost, cost, 1e-9)
			assert.Equal(t, tc.wantDesc, desc)
		})
	}
}

func TestGenerateInviteCode(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantLen int
	}{
		{name: "standard length 6", length: 6, wantLen: 6},
		{name: "length 12", length: 12, wantLen: 12},
		{name: "zero length", length: 0, wantLen: 0},
		{name: "negative length", length: -3, wantLen: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code := GenerateInviteCode(tc.length)
			assert.Len(t, code, tc.wantLen)
		})
	}

	assert.Equal(t, "t.me/mybot?start=invite_abc123", GenerateInviteCodeURL("abc123", "mybot"))
	assert.Equal(t, "t.me/mybot?start=auth_uuid1", GenerateLoginCodeURL("uuid1", "mybot"))
}

func TestEncodeDecodeJSON(t *testing.T) {
	type sample struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	in := sample{ID: 7, Name: "SplitCore"}
	encoded, err := EncodeJSON(in)
	require.NoError(t, err)

	decoded, err := DecodeJSON[sample](encoded)
	require.NoError(t, err)
	assert.Equal(t, in, decoded)

	_, err = DecodeJSON[sample]([]byte("{invalid"))
	require.Error(t, err)
}
