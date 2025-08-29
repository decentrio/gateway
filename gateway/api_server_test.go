package gateway_test

import (
	"testing"

	"github.com/decentrio/gateway/gateway"
	"github.com/stretchr/testify/require"
)

func TestGetHeightFromURL(t *testing.T) {
	testcases := []struct {
		name      string
		url       string
		expHeight string
	}{
		{
			name:      "suc",
			url:       "/cosmos/tx/v1beta1/txs?query=tx.height=112",
			expHeight: "112",
		},
		{
			name:      "suc",
			url:       "/cosmos/tx/v1beta1/txs?query=tx.height=112",
			expHeight: "112",
		},
	}
	for _, t := range testcases {
		height, err := gateway.GetHeightFromURL(t.url)
		require.NoError(&testing.T{}, err)
		require.Equal(&testing.T{}, t.expHeight, height)
	}

}
