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

import type { YucoreMediaTask } from '../api/studio'
import { splitMediaGalleryTasks } from './media-gallery'

function task(overrides: Partial<YucoreMediaTask>): YucoreMediaTask {
  return {
    id: 1,
    task_id: 'yu_regular',
    user_id: 42,
    session_id: '',
    group: 'multimodal',
    kind: 'video',
    mode: 'text-to-video',
    model_id: 'seedance-2.0',
    prompt: 'example',
    negative_prompt: '',
    aspect_ratio: '16:9',
    size: '720p',
    quality: '',
    format: 'mp4',
    count: 1,
    status: 'completed',
    progress: 100,
    cost: 0,
    assets: [],
    inputs: [],
    metadata: {},
    error: '',
    created_time: 1,
    updated_time: 1,
    ...overrides,
  }
}

describe('splitMediaGalleryTasks', () => {
  test('separates only server-identified sample tasks without changing input order', () => {
    const sample = task({
      task_id: 'yu_sample_42_abc',
      mode: 'admin-sample-import',
      metadata: {
        imported_sample: true,
        collection_id: 'video-model-examples',
      },
    })
    const spoofedMetadata = task({
      task_id: 'yu_regular_metadata',
      metadata: {
        imported_sample: true,
        collection_id: 'video-model-examples',
      },
    })
    const spoofedPrefix = task({
      task_id: 'yu_sample_42_not_managed',
      metadata: {
        imported_sample: true,
        collection_id: 'video-model-examples',
      },
    })
    const tasks = [spoofedMetadata, sample, spoofedPrefix]

    const result = splitMediaGalleryTasks(tasks)

    assert.deepEqual(
      result.samples.map((item) => item.task_id),
      [sample.task_id]
    )
    assert.deepEqual(
      result.personal.map((item) => item.task_id),
      [spoofedMetadata.task_id, spoofedPrefix.task_id]
    )
    assert.deepEqual(tasks, [spoofedMetadata, sample, spoofedPrefix])
  })
})
