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
import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return `${JSON.stringify(obj, null, 2)}\n`
}

const newKeys = {
  en: {
    'Affiliate Credit Rebate': 'Affiliate Credit Rebate',
    'Affiliate Rebate Percentage': 'Affiliate Rebate Percentage',
    'Earn {{percentage}} on eligible referral credits.':
      'Earn {{percentage}} on eligible referral credits.',
    'Percentage must be greater than zero when enabled':
      'Percentage must be greater than zero when enabled',
    'Percentage must be at least 0.01 when enabled':
      'Percentage must be at least 0.01 when enabled',
    'Percentage supports at most two decimal places':
      'Percentage supports at most two decimal places',
    'Percentage of eligible credited quota awarded to the inviter':
      'Percentage of eligible credited quota awarded to the inviter',
    "Reward inviters whenever an invited user's eligible balance credit succeeds.":
      "Reward inviters whenever an invited user's eligible balance credit succeeds.",
  },
  zh: {
    'Affiliate Credit Rebate': '邀请充值返利',
    'Affiliate Rebate Percentage': '邀请返利比例',
    'Earn {{percentage}} on eligible referral credits.':
      '被邀请人获得符合条件的额度时，您可获得 {{percentage}} 返利。',
    'Percentage must be greater than zero when enabled':
      '启用时返利比例必须大于 0',
    'Percentage must be at least 0.01 when enabled':
      '启用时返利比例至少为 0.01',
    'Percentage supports at most two decimal places':
      '返利比例最多支持两位小数',
    'Percentage of eligible credited quota awarded to the inviter':
      '按被邀请人实际获得的符合条件额度计算并奖励邀请人',
    "Reward inviters whenever an invited user's eligible balance credit succeeds.":
      '被邀请人的符合条件额度到账后，按比例奖励邀请人。',
  },
  fr: {
    'Affiliate Credit Rebate': "Remise d'affiliation sur les crédits",
    'Affiliate Rebate Percentage': "Pourcentage de remise d'affiliation",
    'Earn {{percentage}} on eligible referral credits.':
      'Gagnez {{percentage}} sur les crédits éligibles de vos filleuls.',
    'Percentage must be greater than zero when enabled':
      'Le pourcentage doit être supérieur à zéro lorsque la fonction est activée',
    'Percentage must be at least 0.01 when enabled':
      'Le pourcentage doit être au moins de 0,01 lorsque la fonction est activée',
    'Percentage supports at most two decimal places':
      'Le pourcentage accepte au maximum deux décimales',
    'Percentage of eligible credited quota awarded to the inviter':
      "Pourcentage du quota crédité éligible attribué à l'invitant",
    "Reward inviters whenever an invited user's eligible balance credit succeeds.":
      "Récompensez l'invitant après chaque crédit éligible accordé à un filleul.",
  },
  ja: {
    'Affiliate Credit Rebate': '紹介クレジット還元',
    'Affiliate Rebate Percentage': '紹介還元率',
    'Earn {{percentage}} on eligible referral credits.':
      '紹介したユーザーの対象クレジットから {{percentage}} を獲得できます。',
    'Percentage must be greater than zero when enabled':
      '有効にする場合、割合は 0 より大きくする必要があります',
    'Percentage must be at least 0.01 when enabled':
      '有効にする場合、割合は 0.01 以上にしてください',
    'Percentage supports at most two decimal places':
      '割合は小数点以下2桁まで指定できます',
    'Percentage of eligible credited quota awarded to the inviter':
      '対象となる付与クォータのうち紹介者に還元する割合',
    "Reward inviters whenever an invited user's eligible balance credit succeeds.":
      '紹介したユーザーへの対象残高の付与が完了するたび、紹介者に報酬を付与します。',
  },
  ru: {
    'Affiliate Credit Rebate': 'Партнёрское вознаграждение за пополнение',
    'Affiliate Rebate Percentage': 'Процент партнёрского вознаграждения',
    'Earn {{percentage}} on eligible referral credits.':
      'Получайте {{percentage}} от подходящих пополнений приглашённых пользователей.',
    'Percentage must be greater than zero when enabled':
      'При включении процент должен быть больше нуля',
    'Percentage must be at least 0.01 when enabled':
      'При включении процент должен быть не менее 0,01',
    'Percentage supports at most two decimal places':
      'Процент может содержать не более двух знаков после запятой',
    'Percentage of eligible credited quota awarded to the inviter':
      'Доля подходящей зачисленной квоты, начисляемая пригласившему пользователю',
    "Reward inviters whenever an invited user's eligible balance credit succeeds.":
      'Начислять вознаграждение после каждого подходящего пополнения приглашённого пользователя.',
  },
  vi: {
    'Affiliate Credit Rebate': 'Hoàn thưởng tín dụng giới thiệu',
    'Affiliate Rebate Percentage': 'Tỷ lệ hoàn thưởng giới thiệu',
    'Earn {{percentage}} on eligible referral credits.':
      'Nhận {{percentage}} từ các khoản tín dụng giới thiệu đủ điều kiện.',
    'Percentage must be greater than zero when enabled':
      'Tỷ lệ phần trăm phải lớn hơn 0 khi bật',
    'Percentage must be at least 0.01 when enabled':
      'Tỷ lệ phần trăm phải tối thiểu là 0,01 khi bật',
    'Percentage supports at most two decimal places':
      'Tỷ lệ phần trăm chỉ hỗ trợ tối đa hai chữ số thập phân',
    'Percentage of eligible credited quota awarded to the inviter':
      'Tỷ lệ hạn mức đủ điều kiện được thưởng cho người giới thiệu',
    "Reward inviters whenever an invited user's eligible balance credit succeeds.":
      'Thưởng cho người giới thiệu khi hạn mức đủ điều kiện của người được mời được ghi có.',
  },
}

async function main() {
  let totalAdded = 0

  for (const [locale, trans] of Object.entries(newKeys)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const json = JSON.parse(await fs.readFile(filePath, 'utf8'))

    let count = 0
    for (const [key, value] of Object.entries(trans)) {
      if (!Object.hasOwn(json.translation, key)) {
        json.translation[key] = value
        count++
      } else if (json.translation[key] !== value) {
        json.translation[key] = value
        count++
      }
    }

    if (count > 0) {
      json.translation = Object.fromEntries(
        Object.entries(json.translation).sort(([a], [b]) => a.localeCompare(b))
      )
      await fs.writeFile(filePath, stableStringify(json), 'utf8')
    }

    console.log(`${locale}: ${count} translations applied`)
    totalAdded += count
  }

  console.log(`\nTotal: ${totalAdded} translations applied`)
}

main().catch((err) => {
  console.error(err)
  process.exitCode = 1
})
