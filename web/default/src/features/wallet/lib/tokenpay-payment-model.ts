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
  TokenPayNetwork,
  TokenPayOrderStatus,
  TokenPayPackage,
} from '../types'

export function sortTokenPayPackages(
  packages: readonly TokenPayPackage[]
): TokenPayPackage[] {
  return [...packages].sort((left, right) => left.usdt - right.usdt)
}

export function tokenPayNetworkLabel(network: TokenPayNetwork): string {
  switch (network) {
    case 'USDT_TRC20':
      return 'TRON (TRC20)'
    case 'EVM_BSC_USDT_BEP20':
      return 'BSC (BEP20)'
    case 'EVM_Polygon_USDT_ERC20':
      return 'Polygon'
  }
}

export function isTokenPayTerminal(status: TokenPayOrderStatus): boolean {
  return status === 'paid' || status === 'failed'
}

export function claimTokenPayCredit(
  tradeNo: string,
  status: TokenPayOrderStatus,
  refreshed: Set<string>
): boolean {
  if (status !== 'paid' || refreshed.has(tradeNo)) return false
  refreshed.add(tradeNo)
  return true
}
