package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

func TestUserGroupInfoPreservesLegacyFieldsWithProtocolMetadata(t *testing.T) {
	info := UserGroupInfo{
		Ratio: 0.3,
		Desc:  "国产模型多协议路由",
		GroupProtocolMetadata: service.GroupProtocolMetadata{
			Protocols:     []string{"openai", "claude"},
			EndpointPaths: []string{"/v1/chat/completions", "/v1/messages"},
		},
	}

	data, err := common.Marshal(info)
	require.NoError(t, err)

	decoded := make(map[string]interface{})
	require.NoError(t, common.Unmarshal(data, &decoded))
	require.Equal(t, 0.3, decoded["ratio"])
	require.Equal(t, "国产模型多协议路由", decoded["desc"])
	require.Equal(t, []interface{}{"openai", "claude"}, decoded["protocols"])
	require.Equal(t,
		[]interface{}{"/v1/chat/completions", "/v1/messages"},
		decoded["endpoint_paths"],
	)
}
