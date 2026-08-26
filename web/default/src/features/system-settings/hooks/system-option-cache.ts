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
import type {
  SystemOption,
  SystemOptionsResponse,
  UpdateOptionRequest,
} from '../types'

export function applySystemOptionUpdates(
  current: SystemOptionsResponse | undefined,
  updates: ReadonlyArray<Pick<UpdateOptionRequest, 'key' | 'value'>>
): SystemOptionsResponse | undefined {
  if (!current || updates.length === 0) return current

  const options = new Map<string, SystemOption>(
    current.data.map((option) => [option.key, option])
  )
  for (const update of updates) {
    options.set(update.key, {
      key: update.key,
      value: String(update.value),
    })
  }

  return {
    ...current,
    data: [...options.values()],
  }
}
