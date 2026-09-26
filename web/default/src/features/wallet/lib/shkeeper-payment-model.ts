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
import type {
  SHKeeperInvoice,
  SHKeeperOrderStatus,
  SHKeeperPackage,
} from '../types'

export function sortSHKeeperPackages(
  packages: readonly SHKeeperPackage[]
): SHKeeperPackage[] {
  return [...packages].sort((a, b) => a.usdt - b.usdt)
}

export function isSHKeeperTerminal(status: SHKeeperOrderStatus): boolean {
  return (
    status === 'paid' ||
    status === 'overpaid' ||
    status === 'late' ||
    status === 'failed'
  )
}

export function canRecoverSHKeeperTransaction(
  invoice: SHKeeperInvoice
): boolean {
  return (
    Boolean(invoice.address) &&
    (invoice.status === 'unpaid' ||
      invoice.status === 'partial' ||
      invoice.status === 'confirming')
  )
}

export function claimSHKeeperCredit(
  invoice: SHKeeperInvoice,
  refreshed: Set<string>
): boolean {
  if (
    !(Number(invoice.credited_balance) > 0) ||
    refreshed.has(invoice.trade_no)
  ) {
    return false
  }
  refreshed.add(invoice.trade_no)
  return true
}
