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
import type { BillingSettings } from '../types'

export function getGroupDefaults(settings: BillingSettings) {
  return {
    TopupGroupRatio: settings.TopupGroupRatio,
    GroupRatio: settings.GroupRatio,
    UserUsableGroups: settings.UserUsableGroups,
    SensitiveInputCheckGroups: settings.SensitiveInputCheckGroups,
    GroupGroupRatio: settings.GroupGroupRatio,
    UserGroupRatio: settings['group_ratio_setting.user_group_ratio'],
    AvailabilityMonitoring:
      settings['group_ratio_setting.availability_monitoring'],
    AutoGroups: settings.AutoGroups,
    DefaultUseAutoGroup: settings.DefaultUseAutoGroup,
    GroupSpecialUsableGroup:
      settings['group_ratio_setting.group_special_usable_group'],
  }
}
