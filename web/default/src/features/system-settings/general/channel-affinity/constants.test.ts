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

import { RULE_TEMPLATES, mergeAffinityRuleForSave } from './constants'
import type { AffinityRule } from './types'

describe('channel affinity rule persistence', () => {
  test('keeps GPT text cache injection enabled when a rule is edited', () => {
    const template = RULE_TEMPLATES.codexCli as AffinityRule & {
      inject_prompt_cache_key?: boolean
    }
    assert.equal(template.inject_prompt_cache_key, true)
    assert.ok(template.path_regex.includes('/v1/responses'))
    assert.ok(template.path_regex.includes('/v1/chat/completions'))

    const edited: AffinityRule = {
      ...template,
      name: 'edited gpt text affinity',
    }
    delete (edited as AffinityRule & { inject_prompt_cache_key?: boolean })
      .inject_prompt_cache_key

    const saved = mergeAffinityRuleForSave(template, edited) as AffinityRule & {
      inject_prompt_cache_key?: boolean
    }
    assert.equal(saved.name, 'edited gpt text affinity')
    assert.equal(saved.inject_prompt_cache_key, true)
  })
})
