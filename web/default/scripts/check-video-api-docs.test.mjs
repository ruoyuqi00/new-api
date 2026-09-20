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
  EXPECTED_EXPANDED_VIDEO_CAPABILITIES,
  EXPECTED_VIDEO_CAPABILITIES,
  checkDocument,
  checkParity,
} from './check-video-api-docs.mjs'

function renderRows(rows) {
  return rows
    .map(
      ([model, billing, resolution]) =>
        `| \`${model}\` | \`${billing}\` | \`${resolution}\` |`
    )
    .join('\n')
}

function buildDocument() {
  return `# Test

<!-- video-model-catalog:start -->
| Model | Billing | Resolution |
| --- | --- | --- |
${renderRows(EXPECTED_VIDEO_CAPABILITIES)}
<!-- video-model-catalog:end -->

<!-- expanded-video-model-catalog:start -->
| Model | Billing | Resolution |
| --- | --- | --- |
${renderRows(EXPECTED_EXPANDED_VIDEO_CAPABILITIES)}
<!-- expanded-video-model-catalog:end -->

Current prices: [/pricing](/pricing)

GET /v1/models
POST /v1/videos
GET /v1/videos/{task_id}
GET /v1/videos/{task_id}/content
POST /v1/images/generations
POST /v1/images/edits

queued processing completed succeeded success failed canceled cancelled

\`grok-imagine-image\` \`grok-imagine-image-quality\` \`grok-imagine-video\` \`grok-imagine-video-1.5\` \`grok-imagine-video-1.5-preview\`

\`\`\`json
{"model":"seedance-2.0","duration":4,"generate_audio":false}
\`\`\`
`
}

describe('video API documentation checker', () => {
  test('accepts a complete price-free public contract', () => {
    const contract = checkDocument('valid.md', buildDocument())
    expect(contract.videoCapabilities).toEqual(EXPECTED_VIDEO_CAPABILITIES)
    expect(contract.expandedVideoCapabilities).toEqual(
      EXPECTED_EXPANDED_VIDEO_CAPABILITIES
    )
  })

  test('rejects a missing expanded video capability row', () => {
    const changed = buildDocument().replace(
      '| `minimax-h3` | `per_second` | `480p/768p/1080p/2K/4K` |\n',
      ''
    )
    expect(() => checkDocument('missing-expanded.md', changed)).toThrow(
      'EXPANDED_VIDEO_MODEL_SET_MISMATCH'
    )
  })

  test('rejects a changed video capability', () => {
    const changed = buildDocument().replace(
      '| `grok-v1.5-video` | `per_successful_task` | `720p/1080p` |',
      '| `grok-v1.5-video` | `per_second` | `720p/1080p` |'
    )
    expect(() => checkDocument('changed-expanded.md', changed)).toThrow(
      'EXPANDED_VIDEO_CAPABILITY_MISMATCH'
    )
  })

  test('rejects a price column in a public model catalog', () => {
    const changed = buildDocument().replace(
      '| Model | Billing | Resolution |',
      '| Model | Billing | Resolution | Price |'
    )
    expect(() => checkDocument('priced.md', changed)).toThrow(
      'PUBLIC_PRICE_COLUMN'
    )
  })

  test('requires the live pricing page link', () => {
    const changed = buildDocument().replace(
      'Current prices: [/pricing](/pricing)',
      'Current prices are available elsewhere.'
    )
    expect(() => checkDocument('no-pricing-link.md', changed)).toThrow(
      'PRICING_LINK_MISMATCH'
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
