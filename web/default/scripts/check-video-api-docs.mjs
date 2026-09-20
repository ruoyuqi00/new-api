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
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

export const EXPECTED_VIDEO_CAPABILITIES = [
  ['grok-video', 'per_successful_task', 'model_default'],
  ['grok-video-1.5', 'per_successful_task', 'model_default'],
  ['happyhouse-1.0', 'per_successful_task', 'model_default'],
  ['happyhouse-1.1', 'per_successful_task', 'model_default'],
  ['minimax-h3-2k', 'per_successful_task', '2K'],
  ['omni-fast', 'per_successful_task', 'model_default'],
  ['omni-fast-no-water', 'per_successful_task', 'model_default'],
  ['omni-v2v', 'per_successful_task', 'model_default'],
  ['omni-v2v-no-water', 'per_successful_task', 'model_default'],
  ['sd7-seedance-2.0-1080p', 'per_successful_task', '1080p'],
  ['sd7-seedance-2.0-720p', 'per_successful_task', '720p'],
  ['sd8-seedance-2.0', 'per_successful_task', 'model_default'],
  ['seedance-2.0', 'per_successful_task', 'model_default'],
]

export const EXPECTED_EXPANDED_VIDEO_CAPABILITIES = [
  ['seedance-2-0-mini-official', 'per_1m_video_tokens', '480p/720p'],
  ['seedance-2-0-fast-official', 'per_1m_video_tokens', '480p/720p'],
  ['seedance-2-0-official', 'per_1m_video_tokens', '480p/720p/1080p/4K'],
  ['seedance-2-5-official', 'per_1m_video_tokens', '720p/1080p'],
  ['minimax-h3', 'per_second', '480p/768p/1080p/2K/4K'],
  ['wan3.0-video', 'per_second', '480p/720p/1080p'],
  ['wan3.0-video-prime', 'per_second', '480p/720p/1080p'],
  ['seedance2.0-9-3-3-PT', 'per_second', '480p/720p'],
  ['seedance2.5-30-10-10-PT', 'per_second', '480p/720p'],
  ['seedance2.0-fast-PT', 'per_second', '480p/720p'],
  ['grok-v1.5-video', 'per_successful_task', '720p/1080p'],
]

const DOC_PATHS = [
  'public/developer-docs/yucore-api.md',
  'public/developer-docs/yucore-api.zh-TW.md',
  'public/developer-docs/yucore-api.en.md',
]

const REQUIRED_PATHS = [
  '/v1/models',
  '/v1/videos',
  '/v1/videos/{task_id}',
  '/v1/videos/{task_id}/content',
  '/v1/images/generations',
  '/v1/images/edits',
]

const REQUIRED_STATUSES = [
  'queued',
  'processing',
  'completed',
  'succeeded',
  'success',
  'failed',
  'canceled',
  'cancelled',
]

const CATALOG_START = '<!-- video-model-catalog:start -->'
const CATALOG_END = '<!-- video-model-catalog:end -->'
const EXPANDED_CATALOG_START = '<!-- expanded-video-model-catalog:start -->'
const EXPANDED_CATALOG_END = '<!-- expanded-video-model-catalog:end -->'

const GENERIC_PRIVATE_PATTERNS = [
  /上游/iu,
  /供(?:应|應)源/iu,
  /基(?:础|礎)成本/iu,
  /加(?:价|價)/iu,
  /利(?:润|潤)/iu,
  /渠道\s*ID/iu,
  /adapter/iu,
  /provider account/iu,
  /source cost/iu,
  /markup/iu,
  /capacity observation/iu,
  /internal routing/iu,
  /\bsk-[A-Za-z0-9_-]{12,}\b/u,
  /\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b/iu,
  /\b(?:\d{1,3}\.){3}\d{1,3}\b/u,
]

function fail(code, fileName, detail) {
  throw new Error(`${code}: ${fileName}${detail ? ` (${detail})` : ''}`)
}

function parseCapabilityCatalog(
  fileName,
  content,
  startMarker,
  endMarker,
  expectedRows,
  modelSetMismatchCode,
  capabilityMismatchCode
) {
  const start = content.indexOf(startMarker)
  const end = content.indexOf(endMarker)
  if (
    start < 0 ||
    end <= start ||
    content.includes(startMarker, start + startMarker.length) ||
    content.includes(endMarker, end + endMarker.length)
  ) {
    fail(modelSetMismatchCode, fileName, 'catalog markers')
  }

  const section = content.slice(start + startMarker.length, end)
  if (/^\|[^\n]*(?:\bprices?\b|价格|價格)[^\n]*\|$/imu.test(section)) {
    fail('PUBLIC_PRICE_COLUMN', fileName)
  }

  const rows = new Map()
  const rowPattern = /^\|\s*`([^`]+)`\s*\|\s*`([^`]+)`\s*\|\s*`([^`]+)`\s*\|$/gm
  for (const match of section.matchAll(rowPattern)) {
    if (rows.has(match[1])) {
      fail(modelSetMismatchCode, fileName, 'duplicate model')
    }
    rows.set(match[1], [match[1], match[2], match[3]])
  }

  const actualModels = [...rows.keys()].sort()
  const expectedModels = expectedRows.map(([model]) => model).sort()
  if (JSON.stringify(actualModels) !== JSON.stringify(expectedModels)) {
    fail(modelSetMismatchCode, fileName)
  }
  for (const expectedRow of expectedRows) {
    if (
      JSON.stringify(rows.get(expectedRow[0])) !== JSON.stringify(expectedRow)
    ) {
      fail(capabilityMismatchCode, fileName, expectedRow[0])
    }
  }
  return expectedRows.map((row) => [...row])
}

function parseCatalog(fileName, content) {
  return parseCapabilityCatalog(
    fileName,
    content,
    CATALOG_START,
    CATALOG_END,
    EXPECTED_VIDEO_CAPABILITIES,
    'MODEL_SET_MISMATCH',
    'VIDEO_CAPABILITY_MISMATCH'
  )
}

function parseExpandedCatalog(fileName, content) {
  return parseCapabilityCatalog(
    fileName,
    content,
    EXPANDED_CATALOG_START,
    EXPANDED_CATALOG_END,
    EXPECTED_EXPANDED_VIDEO_CAPABILITIES,
    'EXPANDED_VIDEO_MODEL_SET_MISMATCH',
    'EXPANDED_VIDEO_CAPABILITY_MISMATCH'
  )
}

function checkJsonExamples(fileName, content) {
  const fencePattern = /```json\s*([\s\S]*?)```/g
  let count = 0
  for (const match of content.matchAll(fencePattern)) {
    count++
    try {
      JSON.parse(match[1])
    } catch {
      fail('INVALID_JSON', fileName, `block ${count}`)
    }
  }
  if (count === 0) fail('INVALID_JSON', fileName, 'no JSON examples')
}

function checkGrokImagineContract(fileName, content) {
  const requiredModels = [
    'grok-imagine-image',
    'grok-imagine-image-quality',
    'grok-imagine-video',
    'grok-imagine-video-1.5',
    'grok-imagine-video-1.5-preview',
  ]
  for (const model of requiredModels) {
    if (!content.includes(`\`${model}\``)) {
      fail('GROK_IMAGINE_CONTRACT_MISMATCH', fileName, model)
    }
  }
  if (content.includes('grok-imagine-edit')) {
    fail('GROK_IMAGINE_CONTRACT_MISMATCH', fileName, 'unsupported edit model')
  }
}

function checkPrivateContent(fileName, content, privatePatterns) {
  const patterns = [...GENERIC_PRIVATE_PATTERNS, ...privatePatterns]
  for (const [index, pattern] of patterns.entries()) {
    pattern.lastIndex = 0
    if (pattern.test(content)) {
      fail('PRIVATE_CONTENT', fileName, `rule ${index + 1}`)
    }
  }
}

export function checkDocument(fileName, content, privatePatterns = []) {
  checkPrivateContent(fileName, content, privatePatterns)
  const videoCapabilities = parseCatalog(fileName, content)
  const expandedVideoCapabilities = parseExpandedCatalog(fileName, content)
  checkJsonExamples(fileName, content)
  checkGrokImagineContract(fileName, content)

  if (!content.includes('](/pricing)')) {
    fail('PRICING_LINK_MISMATCH', fileName)
  }

  const paths = REQUIRED_PATHS.filter((requiredPath) =>
    content.includes(requiredPath)
  )
  if (paths.length !== REQUIRED_PATHS.length) {
    fail('PATH_MISMATCH', fileName)
  }

  const statuses = REQUIRED_STATUSES.filter((status) =>
    content.includes(status)
  )
  if (statuses.length !== REQUIRED_STATUSES.length) {
    fail('STATUS_MISMATCH', fileName)
  }

  return { videoCapabilities, expandedVideoCapabilities, paths, statuses }
}

export function checkParity(contracts) {
  if (contracts.length !== DOC_PATHS.length) {
    throw new Error('DOCUMENT_PARITY_MISMATCH: expected three documents')
  }
  const expected = JSON.stringify(contracts[0])
  if (contracts.some((contract) => JSON.stringify(contract) !== expected)) {
    throw new Error('DOCUMENT_PARITY_MISMATCH')
  }
}

async function readPrivatePatterns() {
  const patternFile = process.env.YUAPI_PRIVATE_PATTERN_FILE
  if (!patternFile) return []

  const lines = (await fs.readFile(patternFile, 'utf8')).split(/\r?\n/)
  const patterns = []
  for (const [index, line] of lines.entries()) {
    const value = line.trim()
    if (!value || value.startsWith('#')) continue
    try {
      patterns.push(new RegExp(value, 'iu'))
    } catch {
      throw new Error(`PRIVATE_PATTERN_INVALID: rule ${index + 1}`)
    }
  }
  return patterns
}

export async function main() {
  const privatePatterns = await readPrivatePatterns()
  const contracts = []
  for (const fileName of DOC_PATHS) {
    const content = await fs.readFile(fileName, 'utf8')
    contracts.push(checkDocument(fileName, content, privatePatterns))
    console.log(`DOCS_OK ${path.basename(fileName)}`)
  }
  checkParity(contracts)
  console.log('DOCUMENT_PARITY_OK')
}

if (path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : 'DOCS_CHECK_FAILED')
    process.exitCode = 1
  })
}
