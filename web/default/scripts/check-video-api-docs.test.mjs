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
  EXPECTED_VIDEO_PRICES,
  checkDocument,
  checkParity,
} from './check-video-api-docs.mjs'

function buildDocument() {
  const rows = [...EXPECTED_VIDEO_PRICES]
    .map(([model, price]) => `| \`${model}\` | ${price} |`)
    .join('\n')

  return `# Test

<!-- video-model-catalog:start -->
| Model | Price |
| --- | ---: |
${rows}
<!-- video-model-catalog:end -->

GET /v1/models
POST /v1/videos
GET /v1/videos/{task_id}
GET /v1/videos/{task_id}/content
POST /v1/images/generations
POST /v1/images/edits

queued processing completed succeeded success failed canceled cancelled

\`\`\`json
{"model":"seedance-2.0","duration":4,"generate_audio":false}
\`\`\`
`
}

describe('video API documentation checker', () => {
  test('accepts a complete public contract', () => {
    expect(checkDocument('valid.md', buildDocument()).modelPrices).toEqual(
      [...EXPECTED_VIDEO_PRICES]
    )
  })

  test('rejects a missing model', () => {
    const changed = buildDocument().replace('| `seedance-2.0` | 5.616 |\n', '')
    expect(() => checkDocument('missing.md', changed)).toThrow(
      'MODEL_SET_MISMATCH'
    )
  })

  test('rejects a changed public price', () => {
    const changed = buildDocument().replace(
      '| `seedance-2.0` | 5.616 |',
      '| `seedance-2.0` | 5.5 |'
    )
    expect(() => checkDocument('changed.md', changed)).toThrow(
      'PRICE_MISMATCH'
    )
  })

  test('rejects invalid JSON examples', () => {
    const changed = buildDocument().replace('"duration":4', '"duration":')
    expect(() => checkDocument('invalid-json.md', changed)).toThrow(
      'INVALID_JSON'
    )
  })

  test('rejects internal dependency wording', () => {
    expect(() =>
      checkDocument('private.md', `${buildDocument()}\n上游账号`)
    ).toThrow('PRIVATE_CONTENT')
  })

  test('requires normalized contracts to match', () => {
    const validContract = checkDocument('valid.md', buildDocument())
    const changedContract = {
      ...validContract,
      statuses: [...validContract.statuses, 'unexpected'],
    }
    expect(() =>
      checkParity([validContract, validContract, changedContract])
    ).toThrow('DOCUMENT_PARITY_MISMATCH')
  })
})
