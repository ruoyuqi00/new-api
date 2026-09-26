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
import * as z from 'zod'

import type {
  SHKeeperNetwork,
  SHKeeperSettingsRequest,
  SHKeeperSettingsStatus,
} from '../types'

type Translate = (message: string) => string

type ArrayError = {
  message?: string
  root?: { message?: string }
}

const shkeeperNetworks = [
  'BNB-USDT',
  'USDT',
  'POLYGON-USDT',
] as const satisfies readonly SHKeeperNetwork[]

export function createSHKeeperSettingsSchema(t: Translate) {
  const packageSchema = z.object({
    usdt: z.coerce
      .number()
      .int(t('USDT amount must be a whole number'))
      .positive(t('USDT amount must be positive')),
    balance: z
      .string()
      .trim()
      .refine(
        (value) => /^\d+(?:\.\d{1,6})?$/.test(value) && Number(value) > 0,
        t('Credited balance must be a positive number with up to 6 decimals')
      ),
    label: z
      .string()
      .trim()
      .optional()
      .transform((value) => value ?? ''),
  })

  return z
    .object({
      enabled: z.boolean(),
      base_url: z
        .string()
        .trim()
        .transform((value) => value.replace(/\/+$/, '')),
      api_key: z.string().trim(),
      backend_key: z.string().trim(),
      api_key_configured: z.boolean(),
      backend_key_configured: z.boolean(),
      packages: z.array(packageSchema),
      enabled_networks: z.array(z.enum(shkeeperNetworks)),
      invoice_expiry_minutes: z.coerce
        .number()
        .int(t('Invoice expiry must be a whole number'))
        .positive(t('Invoice expiry must be positive')),
      reconcile_interval_seconds: z.coerce
        .number()
        .int(t('Reconciliation interval must be a whole number'))
        .min(60, t('Reconciliation interval must be at least 60 seconds')),
      allow_private_url: z.boolean(),
    })
    .superRefine((values, context) => {
      const seenAmounts = new Set<number>()
      for (const item of values.packages) {
        if (seenAmounts.has(item.usdt)) {
          context.addIssue({
            code: 'custom',
            path: ['packages'],
            message: t('Each package must use a unique USDT amount'),
          })
          break
        }
        seenAmounts.add(item.usdt)
      }

      if (!values.enabled) return

      if (!values.base_url) {
        context.addIssue({
          code: 'custom',
          path: ['base_url'],
          message: t('SHKeeper base URL is required'),
        })
      }
      if (!values.api_key && !values.api_key_configured) {
        context.addIssue({
          code: 'custom',
          path: ['api_key'],
          message: t('SHKeeper API key is required'),
        })
      }
      if (values.enabled_networks.length === 0) {
        context.addIssue({
          code: 'custom',
          path: ['enabled_networks'],
          message: t('Select at least one SHKeeper network'),
        })
      }
      if (values.packages.length === 0) {
        context.addIssue({
          code: 'custom',
          path: ['packages'],
          message: t('Add at least one SHKeeper package'),
        })
      }
    })
}

export type SHKeeperSettingsFormValues = z.infer<
  ReturnType<typeof createSHKeeperSettingsSchema>
>

export function buildSHKeeperFormDefaults(
  status: SHKeeperSettingsStatus
): SHKeeperSettingsFormValues {
  return {
    enabled: status.enabled,
    base_url: status.base_url,
    api_key: '',
    backend_key: '',
    api_key_configured: status.api_key_configured,
    backend_key_configured: status.backend_key_configured,
    packages: (status.packages ?? []).map((item) => ({
      usdt: item.usdt,
      balance: item.balance,
      label: item.label ?? '',
    })),
    enabled_networks: [...(status.enabled_networks ?? [])],
    invoice_expiry_minutes: status.invoice_expiry_minutes,
    reconcile_interval_seconds: status.reconcile_interval_seconds,
    allow_private_url: status.allow_private_url,
  }
}

export function getSHKeeperArrayErrorMessage(error: ArrayError | undefined) {
  return error?.message ?? error?.root?.message
}

export function buildSHKeeperSettingsRequest(
  values: SHKeeperSettingsFormValues
): SHKeeperSettingsRequest {
  return {
    enabled: values.enabled,
    base_url: values.base_url,
    api_key: values.api_key,
    backend_key: values.backend_key,
    packages: values.packages.map((item) => ({ ...item })),
    enabled_networks: [...values.enabled_networks],
    invoice_expiry_minutes: values.invoice_expiry_minutes,
    reconcile_interval_seconds: values.reconcile_interval_seconds,
    allow_private_url: values.allow_private_url,
  }
}
