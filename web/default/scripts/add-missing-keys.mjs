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
    'Per-call expression': 'Per-call expression',
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
    'User-specific group ratios': 'User-specific group ratios',
    'Configure a ratio for one user without changing their group permissions.':
      'Configure a ratio for one user without changing their group permissions.',
    'Group availability monitoring': 'Group availability monitoring',
    'Show request success availability only; latency and upstream details are never exposed.':
      'Show request success availability only; latency and upstream details are never exposed.',
    'JSON map of group identifiers to availability monitoring switches.':
      'JSON map of group identifiers to availability monitoring switches.',
    'Nested JSON: user id → target group → ratio.':
      'Nested JSON: user id → target group → ratio.',
    'No groups configured.': 'No groups configured.',
    'Recent request success only': 'Recent request success only',
    'Availability uses up to 300 recent GPT text requests':
      'Availability uses up to 300 recent GPT text requests',
    Observing: 'Observing',
    'Recent {{count}} of 300 GPT text requests':
      'Recent {{count}} of 300 GPT text requests',
    'Success {{success}}%, failed {{failure}}%':
      'Success {{success}}%, failed {{failure}}%',
    Stable: 'Stable',
    Degraded: 'Degraded',
    Unavailable: 'Unavailable',
  },
  zh: {
    'Affiliate Credit Rebate': '邀请充值返利',
    'Affiliate Rebate Percentage': '邀请返利比例',
    'Earn {{percentage}} on eligible referral credits.':
      '被邀请人获得符合条件的额度时，您可获得 {{percentage}} 返利。',
    'Per-call expression': '按次表达式',
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
    'User-specific group ratios': '用户专属分组倍率',
    'Configure a ratio for one user without changing their group permissions.':
      '无需改变用户分组权限，为单个用户配置专属倍率。',
    'Group availability monitoring': '分组可用性监控',
    'Show request success availability only; latency and upstream details are never exposed.':
      '仅显示请求成功可用性，不暴露延迟或上游详情。',
    'JSON map of group identifiers to availability monitoring switches.':
      '使用 JSON 映射配置分组标识与可用性监控开关。',
    'Nested JSON: user id → target group → ratio.':
      '嵌套 JSON：用户 ID → 目标分组 → 倍率。',
    'No groups configured.': '暂无已配置分组。',
    'Recent request success only': '仅统计近期请求成功情况',
    'Availability uses up to 300 recent GPT text requests':
      '可用性仅统计最近最多 300 个 GPT 文本请求',
    Observing: '观察中',
    'Recent {{count}} of 300 GPT text requests':
      '最近 {{count}} / 300 个 GPT 文本请求',
    'Success {{success}}%, failed {{failure}}%':
      '成功 {{success}}%，失败 {{failure}}%',
    Stable: '稳定',
    Degraded: '降级',
    Unavailable: '不可用',
  },
  fr: {
    'Affiliate Credit Rebate': "Remise d'affiliation sur les crédits",
    'Affiliate Rebate Percentage': "Pourcentage de remise d'affiliation",
    'Earn {{percentage}} on eligible referral credits.':
      'Gagnez {{percentage}} sur les crédits éligibles de vos filleuls.',
    'Per-call expression': 'Expression par appel',
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
    'User-specific group ratios': 'Ratios de groupe par utilisateur',
    'Configure a ratio for one user without changing their group permissions.':
      'Configurez un ratio pour un utilisateur sans modifier ses permissions de groupe.',
    'Group availability monitoring': 'Surveillance de la disponibilité des groupes',
    'Show request success availability only; latency and upstream details are never exposed.':
      "Affiche uniquement la réussite récente des requêtes ; la latence et les détails amont ne sont jamais exposés.",
    'JSON map of group identifiers to availability monitoring switches.':
      'Carte JSON des groupes vers leurs interrupteurs de surveillance de disponibilité.',
    'Nested JSON: user id → target group → ratio.':
      'JSON imbriqué : identifiant utilisateur → groupe cible → ratio.',
    'No groups configured.': 'Aucun groupe configuré.',
    'Recent request success only': 'Succès récents des requêtes uniquement',
    'Availability uses up to 300 recent GPT text requests':
      "La disponibilité utilise jusqu'à 300 requêtes texte GPT récentes",
    Observing: 'Observation',
    'Recent {{count}} of 300 GPT text requests':
      '{{count}} requêtes texte GPT récentes sur 300',
    'Success {{success}}%, failed {{failure}}%':
      'Réussite {{success}} %, échec {{failure}} %',
    Stable: 'Stable',
    Degraded: 'Dégradé',
    Unavailable: 'Indisponible',
  },
  ja: {
    'Affiliate Credit Rebate': '紹介クレジット還元',
    'Affiliate Rebate Percentage': '紹介還元率',
    'Earn {{percentage}} on eligible referral credits.':
      '紹介したユーザーの対象クレジットから {{percentage}} を獲得できます。',
    'Per-call expression': '呼び出し単位の式',
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
    'User-specific group ratios': 'ユーザー別グループ倍率',
    'Configure a ratio for one user without changing their group permissions.':
      'グループ権限を変更せず、ユーザーごとの倍率を設定します。',
    'Group availability monitoring': 'グループ可用性モニタリング',
    'Show request success availability only; latency and upstream details are never exposed.':
      'リクエスト成功の可用性のみを表示し、遅延や上流の詳細は公開しません。',
    'JSON map of group identifiers to availability monitoring switches.':
      'グループ識別子と可用性監視スイッチの JSON マップです。',
    'Nested JSON: user id → target group → ratio.':
      'ネストされた JSON：ユーザー ID → 対象グループ → 倍率。',
    'No groups configured.': '設定されたグループはありません。',
    'Recent request success only': '最近のリクエスト成功のみ',
    'Availability uses up to 300 recent GPT text requests':
      '可用性は直近最大300件のGPTテキストリクエストで判定します',
    Observing: '観測中',
    'Recent {{count}} of 300 GPT text requests':
      '直近のGPTテキストリクエスト {{count}} / 300 件',
    'Success {{success}}%, failed {{failure}}%':
      '成功 {{success}}%、失敗 {{failure}}%',
    Stable: '安定',
    Degraded: '低下',
    Unavailable: '利用不可',
  },
  ru: {
    'Affiliate Credit Rebate': 'Партнёрское вознаграждение за пополнение',
    'Affiliate Rebate Percentage': 'Процент партнёрского вознаграждения',
    'Earn {{percentage}} on eligible referral credits.':
      'Получайте {{percentage}} от подходящих пополнений приглашённых пользователей.',
    'Per-call expression': 'Выражение за вызов',
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
    'User-specific group ratios': 'Персональные коэффициенты групп',
    'Configure a ratio for one user without changing their group permissions.':
      'Настройте коэффициент для пользователя без изменения его прав группы.',
    'Group availability monitoring': 'Мониторинг доступности групп',
    'Show request success availability only; latency and upstream details are never exposed.':
      'Показывается только успешность запросов; задержка и сведения об upstream не раскрываются.',
    'JSON map of group identifiers to availability monitoring switches.':
      'JSON-карта идентификаторов групп и переключателей мониторинга доступности.',
    'Nested JSON: user id → target group → ratio.':
      'Вложенный JSON: ID пользователя → целевая группа → коэффициент.',
    'No groups configured.': 'Группы не настроены.',
    'Recent request success only': 'Только успешность последних запросов',
    'Availability uses up to 300 recent GPT text requests':
      'Доступность рассчитывается по последним 300 текстовым запросам GPT',
    Observing: 'Наблюдение',
    'Recent {{count}} of 300 GPT text requests':
      'Последние текстовые запросы GPT: {{count}} из 300',
    'Success {{success}}%, failed {{failure}}%':
      'Успешно {{success}} %, с ошибкой {{failure}} %',
    Stable: 'Стабильно',
    Degraded: 'Снижено',
    Unavailable: 'Недоступно',
  },
  vi: {
    'Affiliate Credit Rebate': 'Hoàn thưởng tín dụng giới thiệu',
    'Affiliate Rebate Percentage': 'Tỷ lệ hoàn thưởng giới thiệu',
    'Earn {{percentage}} on eligible referral credits.':
      'Nhận {{percentage}} từ các khoản tín dụng giới thiệu đủ điều kiện.',
    'Per-call expression': 'Biểu thức theo lượt gọi',
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
    'User-specific group ratios': 'Tỷ lệ nhóm theo người dùng',
    'Configure a ratio for one user without changing their group permissions.':
      'Thiết lập tỷ lệ cho một người dùng mà không thay đổi quyền nhóm của họ.',
    'Group availability monitoring': 'Giám sát khả dụng nhóm',
    'Show request success availability only; latency and upstream details are never exposed.':
      'Chỉ hiển thị khả dụng theo lượt gọi thành công; không hiển thị độ trễ hoặc chi tiết upstream.',
    'JSON map of group identifiers to availability monitoring switches.':
      'Bản đồ JSON của mã nhóm và công tắc giám sát khả dụng.',
    'Nested JSON: user id → target group → ratio.':
      'JSON lồng nhau: ID người dùng → nhóm đích → tỷ lệ.',
    'No groups configured.': 'Chưa cấu hình nhóm nào.',
    'Recent request success only': 'Chỉ thành công của các lượt gọi gần đây',
    'Availability uses up to 300 recent GPT text requests':
      'Mức khả dụng dựa trên tối đa 300 yêu cầu văn bản GPT gần nhất',
    Observing: 'Đang theo dõi',
    'Recent {{count}} of 300 GPT text requests':
      '{{count}}/300 yêu cầu văn bản GPT gần nhất',
    'Success {{success}}%, failed {{failure}}%':
      'Thành công {{success}}%, lỗi {{failure}}%',
    Stable: 'Ổn định',
    Degraded: 'Suy giảm',
    Unavailable: 'Không khả dụng',
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
