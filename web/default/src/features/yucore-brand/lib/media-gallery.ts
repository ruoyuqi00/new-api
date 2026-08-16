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
import type { YucoreMediaTask } from '../api/studio'

const SAMPLE_TASK_PREFIX = 'yu_sample_'
const SAMPLE_TASK_MODE = 'admin-sample-import'
const SAMPLE_COLLECTION_ID = 'video-model-examples'

export function splitMediaGalleryTasks(tasks: readonly YucoreMediaTask[]): {
  samples: YucoreMediaTask[]
  personal: YucoreMediaTask[]
} {
  const samples: YucoreMediaTask[] = []
  const personal: YucoreMediaTask[] = []

  for (const task of tasks) {
    const isSample =
      task.task_id.startsWith(SAMPLE_TASK_PREFIX) &&
      task.mode === SAMPLE_TASK_MODE &&
      task.metadata.imported_sample === true &&
      task.metadata.collection_id === SAMPLE_COLLECTION_ID
    if (isSample) {
      samples.push(task)
    } else {
      personal.push(task)
    }
  }

  return { samples, personal }
}
