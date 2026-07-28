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

const KEEP_CURRENT_PAGE_KEY = 'keepCurrentPageOnError'
const HANDLE_ERROR_LOCALLY_KEY = 'handleErrorLocally'
const NAVIGATE_TO_SERVER_ERROR_KEY = 'navigateToServerError'

export const KEEP_CURRENT_PAGE_ON_QUERY_ERROR = {
  [KEEP_CURRENT_PAGE_KEY]: true,
  [HANDLE_ERROR_LOCALLY_KEY]: true,
} as const

export const NAVIGATE_TO_SERVER_ERROR_ON_QUERY_ERROR = {
  [NAVIGATE_TO_SERVER_ERROR_KEY]: true,
} as const

export function shouldShowGlobalQueryErrorToast(
  meta: Record<string, unknown> | undefined
): boolean {
  return meta?.[HANDLE_ERROR_LOCALLY_KEY] !== true
}

export function shouldNavigateToServerError(
  status: number | undefined,
  meta: Record<string, unknown> | undefined
): boolean {
  return (
    status === 500 &&
    meta?.[NAVIGATE_TO_SERVER_ERROR_KEY] === true &&
    meta?.[KEEP_CURRENT_PAGE_KEY] !== true
  )
}
