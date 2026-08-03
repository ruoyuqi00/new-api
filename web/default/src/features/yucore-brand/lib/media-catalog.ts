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
import type { YucoreMediaCatalog, YucoreMediaModel } from '../api/studio'

export type YucoreMediaKind = 'image' | 'video'

export function modelsForKind(
  catalog: YucoreMediaCatalog,
  groupId: string,
  kind: YucoreMediaKind
): YucoreMediaModel[] {
  const group = catalog.groups.find((item) => item.id === groupId)
  return group?.models.filter((model) => model.kind === kind) ?? []
}

export function resolveMediaSelection(
  catalog: YucoreMediaCatalog,
  groupId: string,
  modelId: string,
  kind: YucoreMediaKind
): { group: string; modelId: string } {
  let group = catalog.groups.find((item) => item.id === groupId)
  if (!group) {
    group = catalog.groups.find((item) => item.id === catalog.default_group)
  }
  if (!group) {
    group = catalog.groups[0]
  }
  if (!group) {
    return { group: '', modelId: '' }
  }

  const models = group.models.filter((model) => model.kind === kind)
  const selected = models.find((model) => model.id === modelId) ?? models[0]
  return { group: group.id, modelId: selected?.id ?? '' }
}
