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
    'Log cleanup resumed.': 'Log cleanup resumed.',
    'Resume failed cleanup': 'Resume failed cleanup',
    'The previous cleanup stopped before its final status was saved. Resume it to finish safely without applying usage adjustments twice.':
      'The previous cleanup stopped before its final status was saved. Resume it to finish safely without applying usage adjustments twice.',
    'Any aspect ratio': 'Any aspect ratio',
    'Automatic detection': 'Automatic detection',
    'Controls whether non-square image requests can use this channel.':
      'Controls whether non-square image requests can use this channel.',
    'Image dimension support': 'Image dimension support',
    'Pending verification': 'Pending verification',
    'Square only': 'Square only',
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
    'Availability uses up to 300 recent GPT or Claude text requests':
      'Availability uses up to 300 recent GPT or Claude text requests',
    Observing: 'Observing',
    'Recent {{count}} of 300 GPT or Claude text requests':
      'Recent {{count}} of 300 GPT or Claude text requests',
    'Success {{success}}%, failed {{failure}}%':
      'Success {{success}}%, failed {{failure}}%',
    Stable: 'Stable',
    Degraded: 'Degraded',
    Unavailable: 'Unavailable',
    'All Categories': 'All Categories',
    'All Priorities': 'All Priorities',
    'All Statuses': 'All Statuses',
    'Attachment upload failed': 'Attachment upload failed',
    Attachments: 'Attachments',
    'Close Ticket': 'Close Ticket',
    Closed: 'Closed',
    'Describe an issue or request a manual refund review. Refunds are handled offline by an administrator.':
      'Describe an issue or request a manual refund review. Refunds are handled offline by an administrator.',
    'Describe the problem in detail': 'Describe the problem in detail',
    'High Priority': 'High Priority',
    'Manual Refund': 'Manual Refund',
    'My Tickets': 'My Tickets',
    'New Ticket': 'New Ticket',
    'No tickets found': 'No tickets found',
    'Normal Priority': 'Normal Priority',
    Reopen: 'Reopen',
    'Reply could not be sent': 'Reply could not be sent',
    'Reply sent': 'Reply sent',
    'Reply to Ticket': 'Reply to Ticket',
    'Search tickets': 'Search tickets',
    'Send Reply': 'Send Reply',
    Subject: 'Subject',
    'Subject and description are required':
      'Subject and description are required',
    'Summarize the issue in one sentence':
      'Summarize the issue in one sentence',
    Support: 'Support',
    'Ticket Center': 'Ticket Center',
    'Support Tickets': 'Support Tickets',
    'This ticket is closed. Contact support to reopen it.':
      'This ticket is closed. Contact support to reopen it.',
    'Ticket could not be created': 'Ticket could not be created',
    'Ticket created': 'Ticket created',
    'Ticket not found': 'Ticket not found',
    'Up to 5 files, 50 MB each': 'Up to 5 files, 50 MB each',
    Urgent: 'Urgent',
    'Waiting for Support': 'Waiting for Support',
    'Waiting for User': 'Waiting for User',
    'Write a reply': 'Write a reply',
    '{{count}} files selected': '{{count}} files selected',
  },
  zh: {
    'Log cleanup resumed.': '日志清理已恢复。',
    'Resume failed cleanup': '恢复失败的清理',
    'The previous cleanup stopped before its final status was saved. Resume it to finish safely without applying usage adjustments twice.':
      '上次清理在保存最终状态前意外中断。恢复该任务可以安全完成清理，不会重复调整用量统计。',
    'Any aspect ratio': '任意宽高比',
    'Automatic detection': '自动检测',
    'Controls whether non-square image requests can use this channel.':
      '控制非正方形图像请求是否可使用此渠道。',
    'Image dimension support': '图像尺寸能力',
    'Pending verification': '待验证',
    'Square only': '仅正方形',
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
    'Availability uses up to 300 recent GPT or Claude text requests':
      '可用性仅统计最近最多 300 个 GPT 或 Claude 文本请求',
    Observing: '观察中',
    'Recent {{count}} of 300 GPT or Claude text requests':
      '最近 {{count}} / 300 个 GPT 或 Claude 文本请求',
    'Success {{success}}%, failed {{failure}}%':
      '成功 {{success}}%，失败 {{failure}}%',
    Stable: '稳定',
    Degraded: '降级',
    Unavailable: '不可用',
    'All Categories': '全部类型',
    'All Priorities': '全部优先级',
    'All Statuses': '全部状态',
    'Attachment upload failed': '附件上传失败',
    Attachments: '附件',
    'Close Ticket': '关闭工单',
    Closed: '已关闭',
    'Describe an issue or request a manual refund review. Refunds are handled offline by an administrator.':
      '描述问题，或提交人工退款审核申请。退款由管理员线下处理。',
    'Describe the problem in detail': '请详细描述问题',
    'High Priority': '高优先级',
    'Manual Refund': '人工退款',
    'My Tickets': '我的工单',
    'New Ticket': '新建工单',
    'No tickets found': '暂无工单',
    'Normal Priority': '普通优先级',
    Reopen: '重新打开',
    'Reply could not be sent': '回复发送失败',
    'Reply sent': '回复已发送',
    'Reply to Ticket': '回复工单',
    'Search tickets': '搜索工单',
    'Send Reply': '发送回复',
    Subject: '主题',
    'Subject and description are required': '请填写主题和说明',
    'Summarize the issue in one sentence': '用一句话概括问题',
    Support: '客服',
    'Ticket Center': '工单中心',
    'Support Tickets': '工单中心',
    'This ticket is closed. Contact support to reopen it.':
      '此工单已关闭，请联系管理员重新打开。',
    'Ticket could not be created': '工单创建失败',
    'Ticket created': '工单已创建',
    'Ticket not found': '找不到工单',
    'Up to 5 files, 50 MB each': '最多 5 个文件，每个不超过 50 MB',
    Urgent: '紧急',
    'Waiting for Support': '等待客服处理',
    'Waiting for User': '等待用户回复',
    'Write a reply': '输入回复内容',
    '{{count}} files selected': '已选择 {{count}} 个文件',
  },
  fr: {
    'Log cleanup resumed.': 'Nettoyage des journaux repris.',
    'Resume failed cleanup': 'Reprendre le nettoyage échoué',
    'The previous cleanup stopped before its final status was saved. Resume it to finish safely without applying usage adjustments twice.':
      'Le nettoyage précédent s’est interrompu avant l’enregistrement de son état final. Reprenez-le pour le terminer sans appliquer deux fois les ajustements d’utilisation.',
    'Any aspect ratio': "Tout rapport d'aspect",
    'Automatic detection': 'Détection automatique',
    'Controls whether non-square image requests can use this channel.':
      "Détermine si les requêtes d'images non carrées peuvent utiliser ce canal.",
    'Image dimension support': "Prise en charge des dimensions d'image",
    'Pending verification': 'Vérification en attente',
    'Square only': 'Carré uniquement',
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
    'Group availability monitoring':
      'Surveillance de la disponibilité des groupes',
    'Show request success availability only; latency and upstream details are never exposed.':
      'Affiche uniquement la réussite récente des requêtes ; la latence et les détails amont ne sont jamais exposés.',
    'JSON map of group identifiers to availability monitoring switches.':
      'Carte JSON des groupes vers leurs interrupteurs de surveillance de disponibilité.',
    'Nested JSON: user id → target group → ratio.':
      'JSON imbriqué : identifiant utilisateur → groupe cible → ratio.',
    'No groups configured.': 'Aucun groupe configuré.',
    'Recent request success only': 'Succès récents des requêtes uniquement',
    'Availability uses up to 300 recent GPT or Claude text requests':
      "La disponibilité utilise jusqu'à 300 requêtes texte GPT ou Claude récentes",
    Observing: 'Observation',
    'Recent {{count}} of 300 GPT or Claude text requests':
      '{{count}} requêtes texte GPT ou Claude récentes sur 300',
    'Success {{success}}%, failed {{failure}}%':
      'Réussite {{success}} %, échec {{failure}} %',
    Stable: 'Stable',
    Degraded: 'Dégradé',
    Unavailable: 'Indisponible',
    'All Categories': 'Toutes les catégories',
    'All Priorities': 'Toutes les priorités',
    'All Statuses': 'Tous les statuts',
    'Attachment upload failed': "Échec de l'envoi de la pièce jointe",
    Attachments: 'Pièces jointes',
    'Close Ticket': 'Fermer le ticket',
    Closed: 'Fermé',
    'Describe an issue or request a manual refund review. Refunds are handled offline by an administrator.':
      'Décrivez un problème ou demandez un examen de remboursement manuel. Les remboursements sont traités hors ligne par un administrateur.',
    'Describe the problem in detail': 'Décrivez le problème en détail',
    'High Priority': 'Priorité élevée',
    'Manual Refund': 'Remboursement manuel',
    'My Tickets': 'Mes tickets',
    'New Ticket': 'Nouveau ticket',
    'No tickets found': 'Aucun ticket trouvé',
    'Normal Priority': 'Priorité normale',
    Reopen: 'Rouvrir',
    'Reply could not be sent': "La réponse n'a pas pu être envoyée",
    'Reply sent': 'Réponse envoyée',
    'Reply to Ticket': 'Répondre au ticket',
    'Search tickets': 'Rechercher des tickets',
    'Send Reply': 'Envoyer la réponse',
    Subject: 'Sujet',
    'Subject and description are required':
      'Le sujet et la description sont requis',
    'Summarize the issue in one sentence': 'Résumez le problème en une phrase',
    Support: 'Support',
    'Ticket Center': 'Centre de tickets',
    'Support Tickets': 'Tickets de support',
    'This ticket is closed. Contact support to reopen it.':
      'Ce ticket est fermé. Contactez le support pour le rouvrir.',
    'Ticket could not be created': 'Impossible de créer le ticket',
    'Ticket created': 'Ticket créé',
    'Ticket not found': 'Ticket introuvable',
    'Up to 5 files, 50 MB each': 'Jusqu’à 5 fichiers, 50 Mo chacun',
    Urgent: 'Urgent',
    'Waiting for Support': 'En attente du support',
    'Waiting for User': "En attente de l'utilisateur",
    'Write a reply': 'Écrire une réponse',
    '{{count}} files selected': '{{count}} fichiers sélectionnés',
  },
  ja: {
    'Log cleanup resumed.': 'ログのクリーンアップを再開しました。',
    'Resume failed cleanup': '失敗したクリーンアップを再開',
    'The previous cleanup stopped before its final status was saved. Resume it to finish safely without applying usage adjustments twice.':
      '前回のクリーンアップは最終状態を保存する前に停止しました。再開しても使用量の調整は重複せず、安全に完了できます。',
    'Any aspect ratio': '任意のアスペクト比',
    'Automatic detection': '自動検出',
    'Controls whether non-square image requests can use this channel.':
      '正方形以外の画像リクエストでこのチャネルを使用できるかを制御します。',
    'Image dimension support': '画像サイズ対応',
    'Pending verification': '確認待ち',
    'Square only': '正方形のみ',
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
    'Availability uses up to 300 recent GPT or Claude text requests':
      '可用性は直近最大300件のGPTまたはClaudeテキストリクエストで判定します',
    Observing: '観測中',
    'Recent {{count}} of 300 GPT or Claude text requests':
      '直近のGPTまたはClaudeテキストリクエスト {{count}} / 300 件',
    'Success {{success}}%, failed {{failure}}%':
      '成功 {{success}}%、失敗 {{failure}}%',
    Stable: '安定',
    Degraded: '低下',
    Unavailable: '利用不可',
    'All Categories': 'すべてのカテゴリ',
    'All Priorities': 'すべての優先度',
    'All Statuses': 'すべてのステータス',
    'Attachment upload failed': '添付ファイルのアップロードに失敗しました',
    Attachments: '添付ファイル',
    'Close Ticket': 'チケットを閉じる',
    Closed: '終了',
    'Describe an issue or request a manual refund review. Refunds are handled offline by an administrator.':
      '問題を報告するか、手動返金の確認を依頼してください。返金は管理者がオフラインで処理します。',
    'Describe the problem in detail': '問題を詳しく説明してください',
    'High Priority': '高優先度',
    'Manual Refund': '手動返金',
    'My Tickets': '自分のチケット',
    'New Ticket': '新しいチケット',
    'No tickets found': 'チケットが見つかりません',
    'Normal Priority': '通常優先度',
    Reopen: '再開',
    'Reply could not be sent': '返信を送信できませんでした',
    'Reply sent': '返信を送信しました',
    'Reply to Ticket': 'チケットに返信',
    'Search tickets': 'チケットを検索',
    'Send Reply': '返信を送信',
    Subject: '件名',
    'Subject and description are required': '件名と説明を入力してください',
    'Summarize the issue in one sentence': '問題を一文で要約してください',
    Support: 'サポート',
    'Ticket Center': 'チケットセンター',
    'Support Tickets': 'サポートチケット',
    'This ticket is closed. Contact support to reopen it.':
      'このチケットは終了しています。再開するにはサポートに連絡してください。',
    'Ticket could not be created': 'チケットを作成できませんでした',
    'Ticket created': 'チケットを作成しました',
    'Ticket not found': 'チケットが見つかりません',
    'Up to 5 files, 50 MB each': '最大5ファイル、1ファイル50MBまで',
    Urgent: '緊急',
    'Waiting for Support': 'サポート待ち',
    'Waiting for User': 'ユーザー待ち',
    'Write a reply': '返信を入力',
    '{{count}} files selected': '{{count}}件のファイルを選択',
  },
  ru: {
    'Log cleanup resumed.': 'Очистка журналов возобновлена.',
    'Resume failed cleanup': 'Возобновить неудачную очистку',
    'The previous cleanup stopped before its final status was saved. Resume it to finish safely without applying usage adjustments twice.':
      'Предыдущая очистка остановилась до сохранения итогового состояния. Возобновление безопасно завершит ее без повторной корректировки статистики использования.',
    'Any aspect ratio': 'Любое соотношение сторон',
    'Automatic detection': 'Автоматическое определение',
    'Controls whether non-square image requests can use this channel.':
      'Определяет, можно ли использовать этот канал для запросов неквадратных изображений.',
    'Image dimension support': 'Поддержка размеров изображений',
    'Pending verification': 'Ожидает проверки',
    'Square only': 'Только квадратные',
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
    'Availability uses up to 300 recent GPT or Claude text requests':
      'Доступность рассчитывается по последним 300 текстовым запросам GPT или Claude',
    Observing: 'Наблюдение',
    'Recent {{count}} of 300 GPT or Claude text requests':
      'Последние текстовые запросы GPT или Claude: {{count}} из 300',
    'Success {{success}}%, failed {{failure}}%':
      'Успешно {{success}} %, с ошибкой {{failure}} %',
    Stable: 'Стабильно',
    Degraded: 'Снижено',
    Unavailable: 'Недоступно',
    'All Categories': 'Все категории',
    'All Priorities': 'Все приоритеты',
    'All Statuses': 'Все статусы',
    'Attachment upload failed': 'Не удалось загрузить вложение',
    Attachments: 'Вложения',
    'Close Ticket': 'Закрыть тикет',
    Closed: 'Закрыт',
    'Describe an issue or request a manual refund review. Refunds are handled offline by an administrator.':
      'Опишите проблему или запросите проверку ручного возврата. Возвраты обрабатываются администратором вне системы.',
    'Describe the problem in detail': 'Подробно опишите проблему',
    'High Priority': 'Высокий приоритет',
    'Manual Refund': 'Ручной возврат',
    'My Tickets': 'Мои тикеты',
    'New Ticket': 'Новый тикет',
    'No tickets found': 'Тикеты не найдены',
    'Normal Priority': 'Обычный приоритет',
    Reopen: 'Открыть снова',
    'Reply could not be sent': 'Не удалось отправить ответ',
    'Reply sent': 'Ответ отправлен',
    'Reply to Ticket': 'Ответить на тикет',
    'Search tickets': 'Поиск тикетов',
    'Send Reply': 'Отправить ответ',
    Subject: 'Тема',
    'Subject and description are required': 'Укажите тему и описание',
    'Summarize the issue in one sentence':
      'Кратко опишите проблему одним предложением',
    Support: 'Поддержка',
    'Ticket Center': 'Центр тикетов',
    'Support Tickets': 'Тикеты поддержки',
    'This ticket is closed. Contact support to reopen it.':
      'Тикет закрыт. Обратитесь в поддержку для повторного открытия.',
    'Ticket could not be created': 'Не удалось создать тикет',
    'Ticket created': 'Тикет создан',
    'Ticket not found': 'Тикет не найден',
    'Up to 5 files, 50 MB each': 'До 5 файлов, до 50 МБ каждый',
    Urgent: 'Срочно',
    'Waiting for Support': 'Ожидает поддержки',
    'Waiting for User': 'Ожидает пользователя',
    'Write a reply': 'Напишите ответ',
    '{{count}} files selected': 'Выбрано файлов: {{count}}',
  },
  vi: {
    'Log cleanup resumed.': 'Đã tiếp tục dọn dẹp nhật ký.',
    'Resume failed cleanup': 'Tiếp tục dọn dẹp bị gián đoạn',
    'The previous cleanup stopped before its final status was saved. Resume it to finish safely without applying usage adjustments twice.':
      'Lần dọn dẹp trước đã dừng trước khi lưu trạng thái cuối. Tiếp tục tác vụ để hoàn tất an toàn mà không điều chỉnh số liệu sử dụng hai lần.',
    'Any aspect ratio': 'Mọi tỷ lệ khung hình',
    'Automatic detection': 'Tự động phát hiện',
    'Controls whether non-square image requests can use this channel.':
      'Kiểm soát việc yêu cầu ảnh không vuông có thể sử dụng kênh này hay không.',
    'Image dimension support': 'Hỗ trợ kích thước ảnh',
    'Pending verification': 'Đang chờ xác minh',
    'Square only': 'Chỉ hình vuông',
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
    'Availability uses up to 300 recent GPT or Claude text requests':
      'Mức khả dụng dựa trên tối đa 300 yêu cầu văn bản GPT hoặc Claude gần nhất',
    Observing: 'Đang theo dõi',
    'Recent {{count}} of 300 GPT or Claude text requests':
      '{{count}}/300 yêu cầu văn bản GPT hoặc Claude gần nhất',
    'Success {{success}}%, failed {{failure}}%':
      'Thành công {{success}}%, lỗi {{failure}}%',
    Stable: 'Ổn định',
    Degraded: 'Suy giảm',
    Unavailable: 'Không khả dụng',
    'All Categories': 'Tất cả danh mục',
    'All Priorities': 'Tất cả mức ưu tiên',
    'All Statuses': 'Tất cả trạng thái',
    'Attachment upload failed': 'Tải tệp đính kèm thất bại',
    Attachments: 'Tệp đính kèm',
    'Close Ticket': 'Đóng yêu cầu',
    Closed: 'Đã đóng',
    'Describe an issue or request a manual refund review. Refunds are handled offline by an administrator.':
      'Mô tả vấn đề hoặc yêu cầu quản trị viên xem xét hoàn tiền thủ công. Việc hoàn tiền được xử lý ngoại tuyến.',
    'Describe the problem in detail': 'Mô tả chi tiết vấn đề',
    'High Priority': 'Ưu tiên cao',
    'Manual Refund': 'Hoàn tiền thủ công',
    'My Tickets': 'Yêu cầu của tôi',
    'New Ticket': 'Tạo yêu cầu',
    'No tickets found': 'Không tìm thấy yêu cầu',
    'Normal Priority': 'Ưu tiên thường',
    Reopen: 'Mở lại',
    'Reply could not be sent': 'Không thể gửi phản hồi',
    'Reply sent': 'Đã gửi phản hồi',
    'Reply to Ticket': 'Phản hồi yêu cầu',
    'Search tickets': 'Tìm kiếm yêu cầu',
    'Send Reply': 'Gửi phản hồi',
    Subject: 'Chủ đề',
    'Subject and description are required': 'Cần nhập chủ đề và mô tả',
    'Summarize the issue in one sentence': 'Tóm tắt vấn đề trong một câu',
    Support: 'Hỗ trợ',
    'Ticket Center': 'Trung tâm hỗ trợ',
    'Support Tickets': 'Yêu cầu hỗ trợ',
    'This ticket is closed. Contact support to reopen it.':
      'Yêu cầu này đã đóng. Hãy liên hệ hỗ trợ để mở lại.',
    'Ticket could not be created': 'Không thể tạo yêu cầu',
    'Ticket created': 'Đã tạo yêu cầu',
    'Ticket not found': 'Không tìm thấy yêu cầu',
    'Up to 5 files, 50 MB each': 'Tối đa 5 tệp, mỗi tệp 50 MB',
    Urgent: 'Khẩn cấp',
    'Waiting for Support': 'Đang chờ hỗ trợ',
    'Waiting for User': 'Đang chờ người dùng',
    'Write a reply': 'Viết phản hồi',
    '{{count}} files selected': 'Đã chọn {{count}} tệp',
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
