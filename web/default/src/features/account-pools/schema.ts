import { z } from 'zod'

const providerAccountSchema = z
  .object({
    id: z.number().int().positive().optional(),
    name: z.string().trim().min(1, 'Account name is required').max(128),
    type: z.string().trim().min(1),
    credential: z.string().optional(),
    credential_set: z.boolean().optional(),
    base_url: z
      .string()
      .trim()
      .refine(
        (value) => !value || /^https?:\/\/[^\s]+$/i.test(value),
        'Base URL must use HTTP or HTTPS'
      ),
    model_mapping: z
      .string()
      .trim()
      .refine((value) => {
        if (!value) return true
        try {
          const parsed: unknown = JSON.parse(value)
          return (
            typeof parsed === 'object' &&
            parsed !== null &&
            !Array.isArray(parsed) &&
            Object.values(parsed).every((item) => typeof item === 'string')
          )
        } catch {
          return false
        }
      }, 'Model mapping must be a JSON object with string values'),
    status: z.number(),
    priority: z.number().int(),
    weight: z.number().int().min(0),
    concurrency_limit: z.number().int().min(0),
    cooldown_seconds: z.number().int().min(0),
    expires_at: z.number().int().min(0),
    metadata: z.string(),
  })
  .superRefine((account, context) => {
    if (!account.id && !account.credential?.trim()) {
      context.addIssue({
        code: 'custom',
        path: ['credential'],
        message: 'Credential is required',
      })
    }
    if (account.type !== 'oauth_json' || !account.credential?.trim()) return
    try {
      const parsed: unknown = JSON.parse(account.credential)
      if (
        typeof parsed === 'object' &&
        parsed !== null &&
        !Array.isArray(parsed)
      ) {
        return
      }
    } catch {
      // Report the same field-level error below.
    }
    context.addIssue({
      code: 'custom',
      path: ['credential'],
      message: 'OAuth credential must be a JSON object',
    })
  })

export const accountPoolFormSchema = z
  .object({
    name: z.string().trim().min(1, 'Account pool name is required').max(128),
    provider: z.string().trim().max(64),
    adapter_type: z.number().int().positive('Adapter type is required'),
    group: z.string().trim().min(1, 'Group is required').max(255),
    status: z.number(),
    priority: z.number().int(),
    weight: z.number().int().min(0),
    remark: z.string().max(255),
    channel_ids: z.array(z.number().int().positive()),
    accounts: z.array(providerAccountSchema),
  })
  .superRefine((pool, context) => {
    if (pool.status === 2 || pool.accounts.length > 0) return
    context.addIssue({
      code: 'custom',
      path: ['accounts'],
      message: 'Enabled account pool requires at least one account',
    })
  })

export type AccountPoolFormValues = z.infer<typeof accountPoolFormSchema>
