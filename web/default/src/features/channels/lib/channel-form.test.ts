/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { Channel } from '../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
} from './channel-form'

describe('channel image dimension support', () => {
  test('persists and restores the selected image dimension capability', () => {
    const formData = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'image channel',
      key: 'test-key',
      models: 'gpt-image-2-1k',
      image_dimension_support: 'square' as const,
    }

    const payload = transformFormDataToCreatePayload(formData)
    const settings = JSON.parse(String(payload.channel.settings)) as Record<
      string,
      unknown
    >
    assert.equal(settings.image_dimension_support, 'square')

    const channel = {
      ...payload.channel,
      id: 42,
      status: 1,
      settings: payload.channel.settings,
      channel_info: { multi_key_mode: 'random' },
    } as unknown as Channel
    assert.equal(
      transformChannelToFormDefaults(channel).image_dimension_support,
      'square'
    )
  })

  test('normalizes the legacy ratio capability to the any-ratio option', () => {
    const channel = {
      id: 43,
      name: 'legacy image channel',
      type: 1,
      status: 1,
      settings: '{"image_dimension_support":"ratio"}',
      channel_info: { multi_key_mode: 'random' },
    } as unknown as Channel

    assert.equal(
      transformChannelToFormDefaults(channel).image_dimension_support,
      'any'
    )
  })
})

describe('channel client output limit control', () => {
  test('persists and restores the OpenAI output-limit override', () => {
    const formData = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'openai channel',
      type: 1,
      key: 'test-key',
      models: 'gpt-5.1',
      ignore_client_max_output_tokens: true,
    }

    const payload = transformFormDataToCreatePayload(formData)
    const settings = JSON.parse(String(payload.channel.settings)) as Record<
      string,
      unknown
    >
    assert.equal(settings.ignore_client_max_output_tokens, true)

    const channel = {
      ...payload.channel,
      id: 44,
      status: 1,
      settings: payload.channel.settings,
      channel_info: { multi_key_mode: 'random' },
    } as unknown as Channel
    assert.equal(
      transformChannelToFormDefaults(channel).ignore_client_max_output_tokens,
      true
    )
  })

  test('does not persist the OpenAI-only option for other channel types', () => {
    const formData = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'anthropic channel',
      type: 14,
      key: 'test-key',
      models: 'claude-sonnet-4-5',
      ignore_client_max_output_tokens: true,
    }

    const payload = transformFormDataToCreatePayload(formData)
    const settings = JSON.parse(String(payload.channel.settings)) as Record<
      string,
      unknown
    >
    assert.equal('ignore_client_max_output_tokens' in settings, false)
  })
})
