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
import { test } from 'node:test'

import { renderToStaticMarkup } from 'react-dom/server'

import i18n from '@/i18n/config'

import { PlanForm } from './plan-form'

test('new plan starts paused and unpublished while exposing explicit targets and bounded controls', async () => {
  await i18n.changeLanguage('en')
  const markup = renderToStaticMarkup(
    <PlanForm
      options={{
        models: ['gpt-6-astra', 'gpt-6.1-sol'],
        groups: [
          { name: 'standard', models: ['gpt-6-astra'] },
          { name: 'premium', models: ['gpt-6-astra', 'gpt-6.1-sol'] },
        ],
      }}
      isSaving={false}
      onSave={() => {}}
      onCancel={() => {}}
    />
  )
  assert.match(markup, /role="combobox"/)
  assert.match(markup, /Select groups/)
  assert.match(markup, /gpt-6-astra/)
  assert.match(markup, /gpt-6\.1-sol/)
  assert.match(markup, /Custom interval/)
  assert.match(markup, /Add question/)
  assert.match(
    markup,
    /<input(?=[^>]*name="interval_minutes")(?=[^>]*min="1")(?=[^>]*max="10080")[^>]*>/
  )
  assert.equal(
    (
      markup.match(
        /<[^ >]+(?=[^>]*role="switch")(?=[^>]*aria-checked="false")[^>]*>/g
      ) ?? []
    ).length,
    2
  )
  assert.doesNotMatch(markup, /aria-checked="true"/)
})
