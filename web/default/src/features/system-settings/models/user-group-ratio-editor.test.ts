import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  flattenUserGroupRatios,
  serializeUserGroupRatios,
} from './user-group-ratio-utils'

describe('user-specific group ratio editor data', () => {
  test('flattens and serializes user id overrides without changing group rules', () => {
    const rows = flattenUserGroupRatios(
      '{"81":{"china":0.42,"default":0.8},"82":{"china":0.7}}'
    )

    assert.deepEqual(rows, [
      { userId: '81', group: 'china', ratio: 0.42 },
      { userId: '81', group: 'default', ratio: 0.8 },
      { userId: '82', group: 'china', ratio: 0.7 },
    ])
    assert.equal(
      serializeUserGroupRatios(rows),
      '{\n  "81": {\n    "china": 0.42,\n    "default": 0.8\n  },\n  "82": {\n    "china": 0.7\n  }\n}'
    )
  })
})
