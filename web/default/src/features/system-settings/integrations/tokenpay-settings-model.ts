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
import { z } from 'zod'

import type {
  TokenPayNetwork,
  TokenPaySettingsRequest,
  TokenPaySettingsStatus,
} from '../types'

const networkSchema = z.enum([
  'USDT_TRC20',
  'EVM_BSC_USDT_BEP20',
  'EVM_Polygon_USDT_ERC20',
])

export const tokenPayNetworks: TokenPayNetwork[] = [
  'USDT_TRC20',
  'EVM_BSC_USDT_BEP20',
  'EVM_Polygon_USDT_ERC20',
]

export function createTokenPaySettingsSchema(tokenConfigured: boolean) {
  return z
    .object({
      enabled: z.boolean(),
      base_url: z.string().trim(),
      api_token: z.string(),
      allow_private_url: z.boolean(),
      packages: z.array(
        z.object({
          usdt: z.number().int().positive(),
          balance: z
            .string()
            .trim()
            .regex(/^(?:0|[1-9]\d*)(?:\.\d{1,6})?$/)
            .refine((value) => Number(value) > 0),
          label: z.string(),
        })
      ),
      enabled_networks: z.array(networkSchema),
    })
    .superRefine((values, context) => {
      const seen = new Set<number>()
      values.packages.forEach((item, index) => {
        if (seen.has(item.usdt)) {
          context.addIssue({
            code: 'custom',
            path: ['packages', index, 'usdt'],
            message: 'Duplicate USDT package',
          })
        }
        seen.add(item.usdt)
      })
      if (!values.enabled) return
      if (!values.base_url) {
        context.addIssue({
          code: 'custom',
          path: ['base_url'],
          message: 'Base URL is required',
        })
      } else {
        try {
          const url = new URL(values.base_url)
          if (
            url.username ||
            url.password ||
            url.pathname !== '/' ||
            url.search ||
            url.hash ||
            (url.protocol !== 'https:' && !values.allow_private_url)
          ) {
            throw new Error('Invalid TokenPay URL')
          }
        } catch {
          context.addIssue({
            code: 'custom',
            path: ['base_url'],
            message: 'Enter a valid base URL',
          })
        }
      }
      if (!values.api_token.trim() && !tokenConfigured) {
        context.addIssue({
          code: 'custom',
          path: ['api_token'],
          message: 'API token is required',
        })
      }
      if (
        values.packages.length === 0 ||
        values.enabled_networks.length === 0
      ) {
        context.addIssue({
          code: 'custom',
          path: ['packages'],
          message: 'Select a package and network',
        })
      }
    })
}

export type TokenPaySettingsFormValues = z.infer<
  ReturnType<typeof createTokenPaySettingsSchema>
>

export function tokenPayFormDefaults(
  status?: TokenPaySettingsStatus
): TokenPaySettingsFormValues {
  return {
    enabled: status?.enabled ?? false,
    base_url: status?.base_url ?? '',
    api_token: '',
    allow_private_url: status?.allow_private_url ?? false,
    packages:
      status?.packages?.map((item) => ({
        usdt: item.usdt,
        balance: item.balance,
        label: item.label ?? '',
      })) ?? [],
    enabled_networks: status?.enabled_networks ?? [],
  }
}

export function buildTokenPaySettingsRequest(
  values: TokenPaySettingsFormValues
): TokenPaySettingsRequest {
  return {
    enabled: values.enabled,
    base_url: values.base_url.trim(),
    api_token: values.api_token.trim(),
    allow_private_url: values.allow_private_url,
    packages: values.packages.map((item) => ({
      usdt: item.usdt,
      balance: item.balance.trim(),
      label: item.label.trim(),
    })),
    enabled_networks: [...values.enabled_networks],
  }
}
