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
import { describe, expect, it } from 'bun:test'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const docs = readFileSync(
  resolve(import.meta.dir, '../public/developer-docs/yucore-api.md'),
  'utf8'
)
const normalMediaEndpointPattern = /\$YUAPI_BASE_URL\/(?:images|videos)\b/

describe('VIP direct API documentation', () => {
  it('documents the public and direct endpoints without automatic switching', () => {
    expect(docs).toContain('https://api.yuaiapi.com/v1')
    expect(docs).toContain('https://vip.yuaiapi.com/v1')
    expect(docs).toContain(
      '两个地址共用同一套 API Key、模型列表、账户余额和价格'
    )
    expect(docs).toContain('用户需要在客户端中明确选择要使用的 Base URL')
    expect(docs).toContain('系统不会自动切换、重定向或替换这两个地址')
  })

  it('routes image and video examples through the direct endpoint', () => {
    expect(docs).toContain('YUAPI_MEDIA_BASE_URL="https://vip.yuaiapi.com/v1"')
    expect(docs).toContain('base_url = "https://vip.yuaiapi.com/v1"')
    expect(docs).toContain('$YUAPI_MEDIA_BASE_URL/images/generations')
    expect(docs).toContain('$YUAPI_MEDIA_BASE_URL/images/edits')
    expect(docs).toContain('$YUAPI_MEDIA_BASE_URL/videos')
    expect(docs).toContain('curl "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID"')
    expect(docs).toContain(
      'curl -L "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID/content"'
    )
  })

  it('rejects media endpoint references that use the normal Base URL', () => {
    expect('$YUAPI_BASE_URL/images/generations').toMatch(
      normalMediaEndpointPattern
    )
    expect('$YUAPI_BASE_URL/videos/task_123').toMatch(
      normalMediaEndpointPattern
    )
    expect(docs).not.toMatch(normalMediaEndpointPattern)
  })

  it('derives relative Python video results from the selected direct origin', () => {
    expect(docs).toContain('api_origin = base_url.removesuffix("/v1")')
    expect(docs).toContain('urljoin(api_origin, video_url)')
    expect(docs).not.toContain('urljoin("https://api.yuaiapi.com", video_url)')
  })

  it('keeps normal model discovery on the public endpoint', () => {
    expect(docs).toContain('$YUAPI_BASE_URL/models')
    expect(docs).toContain('YUAPI_BASE_URL="https://api.yuaiapi.com/v1"')
  })

  it('documents current text, image, and video boundaries', () => {
    expect(docs).toContain('`grok-4.5`')
    expect(docs).toContain('`grok-imagine-image`')
    expect(docs).toContain('`grok-imagine-image-quality`')
    expect(docs).toContain('`grok-video`')
    expect(docs).toContain('`grok-video-1.5`')
    expect(docs).toContain('`grok-imagine-video`')
    expect(docs).toContain('`grok-imagine-video-1.5`')
    expect(docs).toContain('`grok-imagine-video-1.5-preview`')
    expect(docs).toContain('0.02619')
    expect(docs).toContain('0.0414')
    expect(docs).toContain('0.0594')
    expect(docs).toContain('0.0774')
    expect(docs).toContain('0.9936')
    expect(docs).toContain('2.0016')
    expect(docs).toContain('"duration": 4')
    expect(docs).toContain('"reference_image_urls": [')
    expect(docs).toContain('"aspect_ratio": "16:9"')
    expect(docs).not.toContain('grok-imagine-edit')
    expect(docs).not.toContain('上游')
    expect(docs).not.toContain('加价')
  })
})
