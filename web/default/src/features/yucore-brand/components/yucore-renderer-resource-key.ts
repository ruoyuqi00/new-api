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
import type { YucoreMotionProfile } from './yucore-motion-performance'

type SignalFieldResourceProps = {
  active?: boolean
  colorMode?: 'dark' | 'light'
  coreMode?: 'full' | 'ambient'
  corePlacement?: 'auth' | 'hero' | 'intro'
  intensity?: 'calm' | 'hero' | 'workbench'
  motionProfile?: YucoreMotionProfile
  renderProfile?: 'default' | 'console' | 'entrance'
}

type EarthResourceProps = {
  active?: boolean
  colorMode?: 'dark' | 'light'
  density?: 'loader' | 'persistent'
  motionProfile?: YucoreMotionProfile
  timeOffsetSeconds?: number
}

export function getSignalFieldResourceKey(
  props: SignalFieldResourceProps
): string {
  return [
    props.colorMode ?? 'dark',
    props.coreMode ?? 'ambient',
    props.corePlacement ?? 'auth',
    props.intensity ?? 'calm',
    props.motionProfile ?? 'balanced',
    props.renderProfile ?? 'default',
  ].join(':')
}

export function getEarthResourceKey(props: EarthResourceProps): string {
  return [
    props.colorMode ?? 'dark',
    props.density ?? 'persistent',
    props.motionProfile ?? 'balanced',
    props.timeOffsetSeconds ?? 0,
  ].join(':')
}
