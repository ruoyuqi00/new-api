import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  filterApiKeyGroupOptions,
  getApiKeyGroupProtocolNode,
  getProtocolRouteLabel,
} from './api-key-group-protocols.ts'

const options = [
  {
    value: 'claude',
    label: 'Claude MAX',
    desc: 'Native messages',
    protocols: ['claude'] as const,
    endpointPaths: ['/v1/messages'],
  },
  {
    value: 'multi',
    label: '国模按量',
    desc: 'Token billing',
    ratio: 0.3,
    protocols: ['openai', 'claude', 'gemini', 'image', 'video'] as const,
    endpointPaths: ['/v1/chat/completions', '/v1/messages'],
  },
  {
    value: 'legacy',
    label: 'Legacy group',
    desc: 'No capability metadata',
  },
]

describe('API key group protocol filtering', () => {
  test('keeps every group in the all view, including legacy metadata', () => {
    assert.deepEqual(
      filterApiKeyGroupOptions(options, 'all', '').map(
        (option) => option.value
      ),
      ['claude', 'multi', 'legacy']
    )
  })

  test('shows multi-protocol groups in every supported protocol view', () => {
    assert.deepEqual(
      filterApiKeyGroupOptions(options, 'claude', '').map(
        (option) => option.value
      ),
      ['claude', 'multi']
    )
    for (const protocol of ['openai', 'gemini', 'image', 'video'] as const) {
      assert.deepEqual(
        filterApiKeyGroupOptions(options, protocol, '').map(
          (option) => option.value
        ),
        ['multi']
      )
    }
  })

  test('combines protocol filtering with the existing search behavior', () => {
    assert.deepEqual(
      filterApiKeyGroupOptions(options, 'claude', 'CHAT/COMPLETIONS').map(
        (option) => option.value
      ),
      ['multi']
    )
    assert.deepEqual(
      filterApiKeyGroupOptions(options, 'openai', 'native messages'),
      []
    )
    assert.deepEqual(
      filterApiKeyGroupOptions(options, 'all', '0.3').map(
        (option) => option.value
      ),
      ['multi']
    )
  })

  test('uses stable route labels and recognizable protocol nodes', () => {
    assert.equal(getProtocolRouteLabel('all'), 'All protocol routes')
    assert.equal(getProtocolRouteLabel('claude'), 'Claude messages routes')
    assert.equal(getProtocolRouteLabel('video'), 'Video generation routes')

    assert.equal(getApiKeyGroupProtocolNode(options[0]), 'CL')
    assert.equal(getApiKeyGroupProtocolNode(options[1]), 'CN')
    assert.equal(getApiKeyGroupProtocolNode(options[2]), 'RT')
    assert.equal(
      getApiKeyGroupProtocolNode({
        value: 'media',
        label: 'Image generation',
        protocols: ['image'],
      }),
      'IM'
    )
    assert.equal(
      getApiKeyGroupProtocolNode({
        value: 'claude-max',
        label: 'Claude-MAX',
        protocols: ['openai', 'claude'],
      }),
      'CL'
    )
    assert.equal(
      getApiKeyGroupProtocolNode({
        value: 'gemini',
        label: 'Gemini 按量',
        protocols: ['openai', 'gemini'],
      }),
      'GM'
    )
  })
})
