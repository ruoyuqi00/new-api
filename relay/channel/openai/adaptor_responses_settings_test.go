package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestPreservesClientOutputLimitByDefault(t *testing.T) {
	maxOutputTokens := uint(4096)
	request := dto.OpenAIResponsesRequest{
		Model:           "gpt-5.1",
		MaxOutputTokens: &maxOutputTokens,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}, request)

	require.NoError(t, err)
	convertedRequest := converted.(dto.OpenAIResponsesRequest)
	require.NotNil(t, convertedRequest.MaxOutputTokens)
	require.Equal(t, maxOutputTokens, *convertedRequest.MaxOutputTokens)
}

func TestConvertOpenAIResponsesRequestIgnoresClientOutputLimitWhenEnabled(t *testing.T) {
	maxOutputTokens := uint(4096)
	request := dto.OpenAIResponsesRequest{
		Model:           "gpt-5.1",
		MaxOutputTokens: &maxOutputTokens,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeOpenAI,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				IgnoreClientMaxOutputTokens: true,
			},
		},
	}, request)

	require.NoError(t, err)
	convertedRequest := converted.(dto.OpenAIResponsesRequest)
	require.Nil(t, convertedRequest.MaxOutputTokens)
}

func TestConvertOpenAIResponsesRequestPreservesClientOutputLimitForNonOpenAIChannel(t *testing.T) {
	maxOutputTokens := uint(4096)
	request := dto.OpenAIResponsesRequest{
		Model:           "grok-4",
		MaxOutputTokens: &maxOutputTokens,
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: constant.ChannelTypeXai,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				IgnoreClientMaxOutputTokens: true,
			},
		},
	}, request)

	require.NoError(t, err)
	convertedRequest := converted.(dto.OpenAIResponsesRequest)
	require.NotNil(t, convertedRequest.MaxOutputTokens)
	require.Equal(t, maxOutputTokens, *convertedRequest.MaxOutputTokens)
}
