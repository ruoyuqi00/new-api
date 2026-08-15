/*
Copyright (C) 2025 QuantumNous

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
import assert from 'node:assert/strict';
import { describe, test } from 'node:test';

import {
  CHANNEL_AFFINITY_RULE_TEMPLATES,
  mergeChannelAffinityRuleForSave,
} from './channel-affinity-template.constants.js';

describe('classic channel affinity rule persistence', () => {
  test('keeps GPT text cache injection enabled when a rule is edited', () => {
    const template = CHANNEL_AFFINITY_RULE_TEMPLATES.codexCli;
    assert.equal(template.inject_prompt_cache_key, true);
    assert.ok(template.path_regex.includes('/v1/responses'));
    assert.ok(template.path_regex.includes('/v1/chat/completions'));

    const edited = { ...template, name: 'edited gpt text affinity' };
    delete edited.inject_prompt_cache_key;
    const saved = mergeChannelAffinityRuleForSave(template, edited);

    assert.equal(saved.name, 'edited gpt text affinity');
    assert.equal(saved.inject_prompt_cache_key, true);
  });

  test('allows editable optional fields to be cleared', () => {
    const template = {
      ...CHANNEL_AFFINITY_RULE_TEMPLATES.codexCli,
      user_agent_include: ['legacy-client'],
    };
    const edited = { ...template };
    delete edited.user_agent_include;
    delete edited.param_override_template;

    const saved = mergeChannelAffinityRuleForSave(template, edited);
    assert.equal(saved.user_agent_include, undefined);
    assert.equal(saved.param_override_template, undefined);
  });
});
