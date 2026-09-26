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
export class SHKeeperSettingsLifecycle {
  private generation = 0
  private inFlightSave: Promise<unknown> | null = null

  beginStatusRequest() {
    return this.generation
  }

  canApplyStatus(generation: number) {
    return generation === this.generation
  }

  runSave<T>(
    cancelPendingStatus: () => Promise<unknown>,
    operation: (generation: number) => Promise<T>
  ): Promise<T> {
    if (this.inFlightSave) {
      return this.inFlightSave as Promise<T>
    }

    const generation = ++this.generation
    const promise = (async () => {
      await cancelPendingStatus()
      return operation(generation)
    })()
    this.inFlightSave = promise
    const clear = () => {
      if (this.inFlightSave === promise) {
        this.inFlightSave = null
      }
    }
    void promise.then(clear, clear)
    return promise
  }
}
