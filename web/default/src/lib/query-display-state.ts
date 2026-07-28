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
export type QueryDisplayState =
  | 'initial-loading'
  | 'refreshing'
  | 'stale-error'
  | 'terminal-error'
  | 'ready'

export interface RetainedQueryData<T> {
  scope: string
  data: T
}

export function getRetainedQueryData<T>(input: {
  data: T | undefined
  scope: string
  retainedData: RetainedQueryData<T> | undefined
}): T | undefined {
  if (input.data !== undefined) return input.data
  if (input.retainedData?.scope === input.scope) return input.retainedData.data
  return undefined
}

export function getQueryDisplayState(input: {
  hasData: boolean
  isPending: boolean
  isFetching: boolean
  isError: boolean
}): QueryDisplayState {
  if (input.isError) return input.hasData ? 'stale-error' : 'terminal-error'
  if (!input.hasData && input.isPending) return 'initial-loading'
  if (input.hasData && input.isFetching) return 'refreshing'
  return 'ready'
}
