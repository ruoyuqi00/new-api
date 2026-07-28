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
  buildGroupPricingRows,
  getGroupCoverageState,
  getSpecialRuleSources,
  serializeGroupPricingRows,
  type GroupPricingRow,
} from '../src/features/system-settings/models/group-config-utils'

const catalog = [
  {
    name: 'ready',
    ratio: 1,
    public: true,
    description: 'Ready group',
    active_channel_count: 2,
    active_model_count: 3,
    active_models: ['a', 'b', 'c'],
  },
  {
    name: 'missing',
    ratio: 0.2,
    public: false,
    description: '',
    active_channel_count: 0,
    active_model_count: 0,
    active_models: [],
  },
]

describe('group pricing configuration', () => {
  test('builds public and private rows from existing JSON settings', () => {
    const rows = buildGroupPricingRows(
      '{"public":1,"private":0.2}',
      '{"public":"Public description"}'
    )

    expect(rows).toHaveLength(2)
    expect(rows.find((row) => row.name === 'public')).toMatchObject({
      ratio: 1,
      public: true,
      description: 'Public description',
    })
    expect(rows.find((row) => row.name === 'private')).toMatchObject({
      ratio: 0.2,
      public: false,
      description: '',
    })
  })

  test('serializes private groups out of UserUsableGroups', () => {
    const rows: GroupPricingRow[] = [
      {
        _id: 'private-row',
        name: 'private',
        ratio: 0.2,
        public: false,
        description: 'Must not leak',
      },
      {
        _id: 'public-row',
        name: 'public',
        ratio: 1,
        public: true,
        description: 'Public description',
      },
    ]

    const result = serializeGroupPricingRows(rows)

    expect(JSON.parse(result.GroupRatio)).toEqual({ private: 0.2, public: 1 })
    expect(JSON.parse(result.UserUsableGroups)).toEqual({
      public: 'Public description',
    })
  })

  test('reports saved groups with active models as ready', () => {
    expect(getGroupCoverageState('ready', catalog)).toBe('ready')
  })

  test('reports saved groups without active models as missing', () => {
    expect(getGroupCoverageState('missing', catalog)).toBe('missing')
  })

  test('reports new unsaved groups separately', () => {
    expect(getGroupCoverageState('new-group', catalog)).toBe('unsaved')
  })

  test('finds user groups that reference a pricing group', () => {
    const sources = getSpecialRuleSources(
      'private',
      '{"partner":{"+:private":"Allowed"},"blocked":{"-:private":"remove"},"other":{"public":""}}'
    )

    expect(sources).toEqual(['blocked', 'partner'])
  })
})
