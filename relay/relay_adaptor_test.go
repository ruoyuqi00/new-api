package relay

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/task/sora"

	"github.com/stretchr/testify/require"
)

func TestGetTaskAdaptorUsesSoraAdaptorForOpenAIAndSoraChannels(t *testing.T) {
	require.IsType(t, &sora.TaskAdaptor{}, GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeOpenAI))))
	require.IsType(t, &sora.TaskAdaptor{}, GetTaskAdaptor(constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeSora))))
}
