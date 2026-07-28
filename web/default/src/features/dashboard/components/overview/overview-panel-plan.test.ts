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

import { getOverviewPanelPlan } from './overview-panel-plan'

describe('overview panel plan', () => {
  test('uses only enabled panels for ordinary users', () => {
    assert.deepEqual(
      getOverviewPanelPlan({
        isAdmin: false,
        apiInfo: true,
        announcements: false,
        faq: true,
        uptime: true,
      }),
      { left: ['api-info', 'faq'], uptime: true }
    )
  })

  test('adds performance health for admins', () => {
    assert.deepEqual(
      getOverviewPanelPlan({
        isAdmin: true,
        apiInfo: false,
        announcements: true,
        faq: false,
        uptime: false,
      }),
      { left: ['performance', 'announcements'], uptime: false }
    )
  })

  test('does no secondary panel work when everything is disabled', () => {
    assert.deepEqual(
      getOverviewPanelPlan({
        isAdmin: false,
        apiInfo: false,
        announcements: false,
        faq: false,
        uptime: false,
      }),
      { left: [], uptime: false }
    )
  })
})
