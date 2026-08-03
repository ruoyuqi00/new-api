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
import { describe, expect, test } from 'bun:test'

import {
  modelsForKind,
  resolveMediaSelection,
} from '../src/features/yucore-brand/lib/media-catalog'
import type { YucoreMediaCatalog } from '../src/features/yucore-brand/api/studio'

const catalog: YucoreMediaCatalog = {
  default_group: 'multimodal',
  groups: [
    {
      id: 'multimodal',
      description: 'Media',
      ratio: 1,
      models: [
        {
          id: 'image-live',
          name: 'image-live',
          kind: 'image',
          modes: ['text-to-image'],
          input_limits: {},
          pricing: {},
          async: false,
        },
        {
          id: 'video-live',
          name: 'video-live',
          kind: 'video',
          modes: ['text-to-video'],
          input_limits: {},
          pricing: {},
          async: true,
        },
      ],
    },
    {
      id: 'images-only',
      description: 'Images',
      ratio: 1.5,
      models: [
        {
          id: 'image-premium',
          name: 'image-premium',
          kind: 'image',
          modes: ['text-to-image'],
          input_limits: {},
          pricing: {},
          async: false,
        },
      ],
    },
  ],
}

describe('YuCore media catalog selection', () => {
  test('preserves a valid model and replaces a missing model deterministically', () => {
    expect(
      resolveMediaSelection(catalog, 'multimodal', 'image-live', 'image')
    ).toEqual({ group: 'multimodal', modelId: 'image-live' })
    expect(
      resolveMediaSelection(catalog, 'multimodal', 'missing', 'image')
    ).toEqual({ group: 'multimodal', modelId: 'image-live' })
  })

  test('filters models by selected group and media kind', () => {
    expect(
      modelsForKind(catalog, 'multimodal', 'video').map((model) => model.id)
    ).toEqual(['video-live'])
    expect(modelsForKind(catalog, 'images-only', 'video')).toEqual([])
  })

  test('keeps a valid empty group selection instead of using a fictional fallback', () => {
    expect(
      resolveMediaSelection(catalog, 'images-only', 'video-live', 'video')
    ).toEqual({ group: 'images-only', modelId: '' })
  })

  test('falls back to the server default group when the group is unavailable', () => {
    expect(
      resolveMediaSelection(catalog, 'removed', 'image-premium', 'image')
    ).toEqual({ group: 'multimodal', modelId: 'image-live' })
  })
})
