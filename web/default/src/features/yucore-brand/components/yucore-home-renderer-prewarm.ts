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
export type YucoreHomePrewarmHost = {
  clearTimeout(handle: number): void
  setTimeout(callback: () => void, delay: number): number
}

export function scheduleYucoreHomeRendererPrewarm(
  host: YucoreHomePrewarmHost,
  durationMs: number,
  prepareSignal: () => void,
  prepareAll: () => void
): () => void {
  const handles = [
    host.setTimeout(prepareSignal, durationMs * 0.8),
    host.setTimeout(prepareAll, durationMs * 0.9),
  ]

  return () => handles.forEach((handle) => host.clearTimeout(handle))
}
