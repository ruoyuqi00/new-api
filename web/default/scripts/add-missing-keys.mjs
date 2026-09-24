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
    'Ignore client max_output_tokens': 'Ignore client max_output_tokens',
    'Remove max_output_tokens from Responses requests before forwarding them upstream':
      'Remove max_output_tokens from Responses requests before forwarding them upstream',
    '1K price': '1K price',
    '2K price': '2K price',
    '4K price': '4K price',
    'Automatic resolution billing': 'Automatic resolution billing',
    'Base price': 'Base price',
    'Clear the search or enable a supported image model first.':
      'Clear the search or enable a supported image model first.',
    'Configure 1K, 2K, and 4K prices under one official image model. The requested size selects the billing tier automatically.':
      'Configure 1K, 2K, and 4K prices under one official image model. The requested size selects the billing tier automatically.',
    'Default tier': 'Default tier',
    'Edit image resolution prices': 'Edit image resolution prices',
    'Higher tiers cannot cost less than lower tiers':
      'Higher tiers cannot cost less than lower tiers',
    'Image resolution price policies': 'Image resolution price policies',
    'Image resolution prices': 'Image resolution prices',
    'JSON map of complete per-model 1K, 2K, and 4K price policies. Use the image resolution prices tab for guided editing.':
      'JSON map of complete per-model 1K, 2K, and 4K price policies. Use the image resolution prices tab for guided editing.',
    'No image pricing models found': 'No image pricing models found',
    'Per image': 'Per image',
    'Price per image': 'Price per image',
    Resolution: 'Resolution',
    'Save resolution prices': 'Save resolution prices',
    'Search image models': 'Search image models',
    'Set the base per-image price for each resolution tier.':
      'Set the base per-image price for each resolution tier.',
    'per image': 'per image',
    'Thinking Tokens': 'Thinking Tokens',
    'Text Output Tokens': 'Text Output Tokens',
    'Thinking Billing': 'Thinking Billing',
    'Thinking tokens are included in output tokens and are not billed twice.':
      'Thinking tokens are included in output tokens and are not billed twice.',
    'Thinking Type': 'Thinking Type',
    'Web Search Calls': 'Web Search Calls',
    'Web Search Unit Price': 'Web Search Unit Price',
    'Web Search Fee': 'Web Search Fee',
    'File Search Calls': 'File Search Calls',
    'File Search Unit Price': 'File Search Unit Price',
    'File Search Fee': 'File Search Fee',
    'View response': 'View response',
    'Test response': 'Test response',
    'Response content': 'Response content',
    'Response content was truncated to 8 KB.':
      'Response content was truncated to 8 KB.',
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
    '{{count}} available groups': '{{count}} available groups',
    'All protocol routes': 'All protocol routes',
    'Claude messages routes': 'Claude messages routes',
    'Configure API key access groups and routing capabilities.':
      'Configure API key access groups and routing capabilities.',
    'Filter available routes by protocol':
      'Filter available routes by protocol',
    'Gemini native routes': 'Gemini native routes',
    'Group capabilities are derived from active channels':
      'Group capabilities are derived from active channels',
    'Image generation routes': 'Image generation routes',
    'OpenAI compatible routes': 'OpenAI compatible routes',
    'Protocol routing': 'Protocol routing',
    'Search groups, descriptions, or API endpoints':
      'Search groups, descriptions, or API endpoints',
    'Set the key name and accessible model groups':
      'Set the key name and accessible model groups',
    'Video generation routes': 'Video generation routes',
    'Video tier price overrides': 'Video tier price overrides',
    'JSON map of complete per-model video tier overrides. Use the video tier prices tab for guided editing.':
      'JSON map of complete per-model video tier overrides. Use the video tier prices tab for guided editing.',
    'Video tier prices': 'Video tier prices',
    'Configure exact prices by resolution. Models without an override continue to follow their base price.':
      'Configure exact prices by resolution. Models without an override continue to follow their base price.',
    'Search video models': 'Search video models',
    'No video pricing models found': 'No video pricing models found',
    'Clear the search or enable a supported video model first.':
      'Clear the search or enable a supported video model first.',
    'Billing unit': 'Billing unit',
    'Price source': 'Price source',
    'Resolution tiers': 'Resolution tiers',
    'Per second': 'Per second',
    'Per successful task': 'Per successful task',
    'Per 1M video tokens': 'Per 1M video tokens',
    Explicit: 'Explicit',
    Inherited: 'Inherited',
    'Edit video tier prices': 'Edit video tier prices',
    'Set an independent price for every supported video tier.':
      'Set an independent price for every supported video tier.',
    'Explicit tier prices': 'Explicit tier prices',
    From: 'From',
    'With reference video': 'With reference video',
    'per second': 'per second',
    'per successful task': 'per successful task',
    'per 1M video tokens': 'per 1M video tokens',
    'Not applicable': 'Not applicable',
    'Use inherited prices': 'Use inherited prices',
    'Save tier prices': 'Save tier prices',
    'Use inherited video prices?': 'Use inherited video prices?',
    'This removes the independent tier prices for this model and returns every resolution to base-price scaling.':
      'This removes the independent tier prices for this model and returns every resolution to base-price scaling.',
    'Must be greater than zero': 'Must be greater than zero',
  },
  zh: {
    'Ignore client max_output_tokens': '忽略客户端 max_output_tokens',
    'Remove max_output_tokens from Responses requests before forwarding them upstream':
      '转发 Responses 请求到上游前移除 max_output_tokens',
    '1K price': '1K 价格',
    '2K price': '2K 价格',
    '4K price': '4K 价格',
    'Automatic resolution billing': '按分辨率自动计费',
    'Base price': '基础价格',
    'Clear the search or enable a supported image model first.':
      '清除搜索条件，或先启用支持的图片模型。',
    'Configure 1K, 2K, and 4K prices under one official image model. The requested size selects the billing tier automatically.':
      '在一个官方图片模型下配置 1K、2K 和 4K 价格。系统会根据请求尺寸自动选择计费档位。',
    'Default tier': '默认档位',
    'Edit image resolution prices': '编辑图片分辨率价格',
    'Higher tiers cannot cost less than lower tiers':
      '高分辨率档价格不能低于低分辨率档',
    'Image resolution price policies': '图片分辨率价格策略',
    'Image resolution prices': '图片分辨率价格',
    'JSON map of complete per-model 1K, 2K, and 4K price policies. Use the image resolution prices tab for guided editing.':
      '按模型配置完整 1K、2K 和 4K 价格策略的 JSON 映射。请使用图片分辨率价格标签页进行可视化编辑。',
    'No image pricing models found': '未找到图片计价模型',
    'Per image': '按张',
    'Price per image': '每张价格',
    Resolution: '分辨率',
    'Save resolution prices': '保存分辨率价格',
    'Search image models': '搜索图片模型',
    'Set the base per-image price for each resolution tier.':
      '为每个分辨率档设置每张图片的基础价格。',
    'per image': '每张',
    'Thinking Tokens': '思考 Token',
    'Text Output Tokens': '正文输出 Token',
    'Thinking Billing': '思考计费',
    'Thinking tokens are included in output tokens and are not billed twice.':
      '思考 Token 已包含在输出 Token 中，不会重复计费。',
    'Thinking Type': '思考类型',
    'Web Search Calls': '网页搜索调用次数',
    'Web Search Unit Price': '网页搜索单价',
    'Web Search Fee': '网页搜索费用',
    'File Search Calls': '文件搜索调用次数',
    'File Search Unit Price': '文件搜索单价',
    'File Search Fee': '文件搜索费用',
    'View response': '查看回复',
    'Test response': '测试回复',
    'Response content': '回复内容',
    'Response content was truncated to 8 KB.': '回复内容已截断至 8 KB。',
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
    '{{count}} available groups': '{{count}} 个可用分组',
    'All protocol routes': '全部协议路由',
    'Claude messages routes': 'Claude 消息路由',
    'Configure API key access groups and routing capabilities.':
      '配置密钥的访问分组与路由能力。',
    'Filter available routes by protocol': '按请求协议筛选可用路由',
    'Gemini native routes': 'Gemini 原生路由',
    'Group capabilities are derived from active channels':
      '分组能力来自启用中的渠道',
    'Image generation routes': '图片生成路由',
    'OpenAI compatible routes': 'OpenAI 兼容路由',
    'Protocol routing': '协议路由',
    'Search groups, descriptions, or API endpoints':
      '搜索分组、说明或 API 端点',
    'Set the key name and accessible model groups':
      '设置密钥名称与可访问的模型分组',
    'Video generation routes': '视频生成路由',
    'Video tier price overrides': '视频档位价格覆盖',
    'JSON map of complete per-model video tier overrides. Use the video tier prices tab for guided editing.':
      '完整的模型视频档位价格 JSON 映射。建议在“视频档位价格”页签中编辑。',
    'Video tier prices': '视频档位价格',
    'Configure exact prices by resolution. Models without an override continue to follow their base price.':
      '按分辨率配置精确价格；未覆盖的模型继续按基础价换算。',
    'Search video models': '搜索视频模型',
    'No video pricing models found': '未找到视频计价模型',
    'Clear the search or enable a supported video model first.':
      '清除搜索条件，或先启用受支持的视频模型。',
    'Billing unit': '计费单位',
    'Price source': '价格来源',
    'Resolution tiers': '分辨率档位',
    'Per second': '按秒',
    'Per successful task': '按成功任务',
    'Per 1M video tokens': '按百万视频 Token',
    Explicit: '独立配置',
    Inherited: '继承基础价',
    'Edit video tier prices': '编辑视频档位价格',
    'Set an independent price for every supported video tier.':
      '为每个支持的视频档位设置独立价格。',
    'Explicit tier prices': '独立档位价格',
    From: '起价',
    'With reference video': '带参考视频',
    'per second': '每秒',
    'per successful task': '每个成功任务',
    'per 1M video tokens': '每百万视频 Token',
    'Not applicable': '不适用',
    'Use inherited prices': '使用继承价格',
    'Save tier prices': '保存档位价格',
    'Use inherited video prices?': '使用继承的视频价格？',
    'This removes the independent tier prices for this model and returns every resolution to base-price scaling.':
      '这会删除该模型的独立档位价格，并让所有分辨率恢复按基础价换算。',
    'Must be greater than zero': '必须大于 0',
  },
  fr: {
    'Ignore client max_output_tokens': 'Ignorer max_output_tokens du client',
    'Remove max_output_tokens from Responses requests before forwarding them upstream':
      'Supprimer max_output_tokens des requêtes Responses avant leur transfert en amont',
    '1K price': 'Prix 1K',
    '2K price': 'Prix 2K',
    '4K price': 'Prix 4K',
    'Automatic resolution billing': 'Facturation automatique par résolution',
    'Base price': 'Prix de base',
    'Clear the search or enable a supported image model first.':
      "Effacez la recherche ou activez d'abord un modèle d'image pris en charge.",
    'Configure 1K, 2K, and 4K prices under one official image model. The requested size selects the billing tier automatically.':
      "Configurez les prix 1K, 2K et 4K sous un modèle d'image officiel. La taille demandée sélectionne automatiquement le palier.",
    'Default tier': 'Palier par défaut',
    'Edit image resolution prices': "Modifier les prix de résolution d'image",
    'Higher tiers cannot cost less than lower tiers':
      'Un palier supérieur ne peut pas coûter moins cher',
    'Image resolution price policies': "Tarifs par résolution d'image",
    'Image resolution prices': "Prix par résolution d'image",
    'JSON map of complete per-model 1K, 2K, and 4K price policies. Use the image resolution prices tab for guided editing.':
      "Objet JSON des tarifs 1K, 2K et 4K complets par modèle. Utilisez l'onglet des prix par résolution pour l'édition guidée.",
    'No image pricing models found': "Aucun modèle d'image tarifé trouvé",
    'Per image': 'Par image',
    'Price per image': 'Prix par image',
    Resolution: 'Résolution',
    'Save resolution prices': 'Enregistrer les prix',
    'Search image models': "Rechercher des modèles d'image",
    'Set the base per-image price for each resolution tier.':
      'Définissez le prix de base par image pour chaque résolution.',
    'per image': 'par image',
    'Thinking Tokens': 'Jetons de raisonnement',
    'Text Output Tokens': 'Jetons de sortie texte',
    'Thinking Billing': 'Facturation du raisonnement',
    'Thinking tokens are included in output tokens and are not billed twice.':
      'Les jetons de raisonnement sont inclus dans les jetons de sortie et ne sont pas facturés deux fois.',
    'Thinking Type': 'Type de raisonnement',
    'Web Search Calls': 'Appels de recherche web',
    'Web Search Unit Price': 'Tarif de recherche web',
    'Web Search Fee': 'Frais de recherche web',
    'File Search Calls': 'Appels de recherche de fichiers',
    'File Search Unit Price': 'Tarif de recherche de fichiers',
    'File Search Fee': 'Frais de recherche de fichiers',
    'View response': 'Voir la réponse',
    'Test response': 'Réponse du test',
    'Response content': 'Contenu de la réponse',
    'Response content was truncated to 8 KB.':
      'Le contenu de la réponse a été tronqué à 8 Ko.',
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
    '{{count}} available groups': '{{count}} groupes disponibles',
    'All protocol routes': 'Toutes les routes de protocole',
    'Claude messages routes': 'Routes de messages Claude',
    'Configure API key access groups and routing capabilities.':
      "Configurez les groupes d'accès et les capacités de routage de la clé API.",
    'Filter available routes by protocol':
      'Filtrer les routes disponibles par protocole',
    'Gemini native routes': 'Routes natives Gemini',
    'Group capabilities are derived from active channels':
      'Les capacités du groupe proviennent des canaux actifs',
    'Image generation routes': "Routes de génération d'images",
    'OpenAI compatible routes': 'Routes compatibles OpenAI',
    'Protocol routing': 'Routage par protocole',
    'Search groups, descriptions, or API endpoints':
      'Rechercher des groupes, descriptions ou points API',
    'Set the key name and accessible model groups':
      'Définissez le nom de la clé et les groupes de modèles accessibles',
    'Video generation routes': 'Routes de génération vidéo',
    'Video tier price overrides': 'Remplacements des tarifs vidéo',
    'JSON map of complete per-model video tier overrides. Use the video tier prices tab for guided editing.':
      'Carte JSON complète des tarifs vidéo par modèle. Utilisez l’onglet des tarifs vidéo pour une modification guidée.',
    'Video tier prices': 'Tarifs vidéo par niveau',
    'Configure exact prices by resolution. Models without an override continue to follow their base price.':
      'Définissez les tarifs par résolution. Sans remplacement, le tarif de base reste appliqué.',
    'Search video models': 'Rechercher des modèles vidéo',
    'No video pricing models found': 'Aucun modèle vidéo tarifé trouvé',
    'Clear the search or enable a supported video model first.':
      'Effacez la recherche ou activez d’abord un modèle vidéo compatible.',
    'Billing unit': 'Unité de facturation',
    'Price source': 'Source du tarif',
    'Resolution tiers': 'Niveaux de résolution',
    'Per second': 'Par seconde',
    'Per successful task': 'Par tâche réussie',
    'Per 1M video tokens': 'Par million de jetons vidéo',
    Explicit: 'Explicite',
    Inherited: 'Hérité',
    'Edit video tier prices': 'Modifier les tarifs vidéo',
    'Set an independent price for every supported video tier.':
      'Définissez un tarif indépendant pour chaque niveau vidéo pris en charge.',
    'Explicit tier prices': 'Tarifs explicites',
    From: 'À partir de',
    'With reference video': 'Avec vidéo de référence',
    'per second': 'par seconde',
    'per successful task': 'par tâche réussie',
    'per 1M video tokens': 'par million de jetons vidéo',
    'Not applicable': 'Non applicable',
    'Use inherited prices': 'Utiliser les tarifs hérités',
    'Save tier prices': 'Enregistrer les tarifs',
    'Use inherited video prices?': 'Utiliser les tarifs vidéo hérités ?',
    'This removes the independent tier prices for this model and returns every resolution to base-price scaling.':
      'Cela supprime les tarifs indépendants de ce modèle et rétablit le calcul depuis le tarif de base.',
    'Must be greater than zero': 'Doit être supérieur à zéro',
  },
  ja: {
    'Ignore client max_output_tokens':
      'クライアントの max_output_tokens を無視',
    'Remove max_output_tokens from Responses requests before forwarding them upstream':
      'Responses リクエストをアップストリームへ転送する前に max_output_tokens を削除します',
    '1K price': '1K 価格',
    '2K price': '2K 価格',
    '4K price': '4K 価格',
    'Automatic resolution billing': '解像度別の自動課金',
    'Base price': '基本価格',
    'Clear the search or enable a supported image model first.':
      '検索をクリアするか、対応する画像モデルを有効にしてください。',
    'Configure 1K, 2K, and 4K prices under one official image model. The requested size selects the billing tier automatically.':
      '1つの公式画像モデルに 1K、2K、4K の価格を設定します。要求サイズに応じて課金区分が自動選択されます。',
    'Default tier': 'デフォルト区分',
    'Edit image resolution prices': '画像解像度価格を編集',
    'Higher tiers cannot cost less than lower tiers':
      '上位区分の価格を下位区分より低くできません',
    'Image resolution price policies': '画像解像度価格ポリシー',
    'Image resolution prices': '画像解像度価格',
    'JSON map of complete per-model 1K, 2K, and 4K price policies. Use the image resolution prices tab for guided editing.':
      'モデルごとの完全な 1K、2K、4K 価格ポリシーの JSON マップです。画像解像度価格タブで編集してください。',
    'No image pricing models found': '画像価格モデルが見つかりません',
    'Per image': '画像ごと',
    'Price per image': '画像単価',
    Resolution: '解像度',
    'Save resolution prices': '解像度価格を保存',
    'Search image models': '画像モデルを検索',
    'Set the base per-image price for each resolution tier.':
      '各解像度区分の画像単価を設定します。',
    'per image': '画像ごと',
    'Thinking Tokens': '思考トークン',
    'Text Output Tokens': 'テキスト出力トークン',
    'Thinking Billing': '思考の課金',
    'Thinking tokens are included in output tokens and are not billed twice.':
      '思考トークンは出力トークンに含まれており、二重課金されません。',
    'Thinking Type': '思考タイプ',
    'Web Search Calls': 'Web 検索回数',
    'Web Search Unit Price': 'Web 検索単価',
    'Web Search Fee': 'Web 検索料金',
    'File Search Calls': 'ファイル検索回数',
    'File Search Unit Price': 'ファイル検索単価',
    'File Search Fee': 'ファイル検索料金',
    'View response': '応答を表示',
    'Test response': 'テスト応答',
    'Response content': '応答内容',
    'Response content was truncated to 8 KB.':
      '応答内容は8KBに切り詰められています。',
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
    '{{count}} available groups': '利用可能なグループ: {{count}}',
    'All protocol routes': 'すべてのプロトコルルート',
    'Claude messages routes': 'Claude メッセージルート',
    'Configure API key access groups and routing capabilities.':
      'API キーのアクセスグループとルーティング機能を設定します。',
    'Filter available routes by protocol':
      'プロトコルで利用可能なルートを絞り込む',
    'Gemini native routes': 'Gemini ネイティブルート',
    'Group capabilities are derived from active channels':
      'グループ機能は有効なチャネルから取得されます',
    'Image generation routes': '画像生成ルート',
    'OpenAI compatible routes': 'OpenAI 互換ルート',
    'Protocol routing': 'プロトコルルーティング',
    'Search groups, descriptions, or API endpoints':
      'グループ、説明、API エンドポイントを検索',
    'Set the key name and accessible model groups':
      'キー名とアクセス可能なモデルグループを設定します',
    'Video generation routes': '動画生成ルート',
    'Video tier price overrides': '動画ティア価格の上書き',
    'JSON map of complete per-model video tier overrides. Use the video tier prices tab for guided editing.':
      'モデル別の完全な動画ティア価格 JSON です。動画ティア価格タブで編集してください。',
    'Video tier prices': '動画ティア価格',
    'Configure exact prices by resolution. Models without an override continue to follow their base price.':
      '解像度ごとの価格を設定します。上書きしないモデルは基本価格に従います。',
    'Search video models': '動画モデルを検索',
    'No video pricing models found': '動画価格モデルが見つかりません',
    'Clear the search or enable a supported video model first.':
      '検索を解除するか、対応する動画モデルを有効にしてください。',
    'Billing unit': '課金単位',
    'Price source': '価格の適用元',
    'Resolution tiers': '解像度ティア',
    'Per second': '秒単位',
    'Per successful task': '成功タスク単位',
    'Per 1M video tokens': '動画100万トークン単位',
    Explicit: '個別設定',
    Inherited: '基本価格を継承',
    'Edit video tier prices': '動画ティア価格を編集',
    'Set an independent price for every supported video tier.':
      '対応する動画ティアごとに個別価格を設定します。',
    'Explicit tier prices': '個別ティア価格',
    From: '最低',
    'With reference video': '参照動画あり',
    'per second': '1秒あたり',
    'per successful task': '成功タスクあたり',
    'per 1M video tokens': '動画100万トークンあたり',
    'Not applicable': '対象外',
    'Use inherited prices': '継承価格を使用',
    'Save tier prices': 'ティア価格を保存',
    'Use inherited video prices?': '継承された動画価格を使用しますか？',
    'This removes the independent tier prices for this model and returns every resolution to base-price scaling.':
      'このモデルの個別ティア価格を削除し、すべての解像度を基本価格からの換算に戻します。',
    'Must be greater than zero': '0 より大きい値が必要です',
  },
  ru: {
    'Ignore client max_output_tokens': 'Игнорировать max_output_tokens клиента',
    'Remove max_output_tokens from Responses requests before forwarding them upstream':
      'Удалять max_output_tokens из запросов Responses перед отправкой провайдеру',
    '1K price': 'Цена 1K',
    '2K price': 'Цена 2K',
    '4K price': 'Цена 4K',
    'Automatic resolution billing': 'Автоматическая тарификация по разрешению',
    'Base price': 'Базовая цена',
    'Clear the search or enable a supported image model first.':
      'Очистите поиск или сначала включите поддерживаемую модель изображений.',
    'Configure 1K, 2K, and 4K prices under one official image model. The requested size selects the billing tier automatically.':
      'Настройте цены 1K, 2K и 4K для одной официальной модели изображений. Тариф выбирается автоматически по размеру.',
    'Default tier': 'Уровень по умолчанию',
    'Edit image resolution prices': 'Изменить цены по разрешению',
    'Higher tiers cannot cost less than lower tiers':
      'Более высокий уровень не может стоить дешевле',
    'Image resolution price policies': 'Тарифы по разрешению изображений',
    'Image resolution prices': 'Цены по разрешению изображений',
    'JSON map of complete per-model 1K, 2K, and 4K price policies. Use the image resolution prices tab for guided editing.':
      'JSON-карта полных тарифов 1K, 2K и 4K для каждой модели. Используйте вкладку цен по разрешению.',
    'No image pricing models found': 'Модели с тарифами изображений не найдены',
    'Per image': 'За изображение',
    'Price per image': 'Цена за изображение',
    Resolution: 'Разрешение',
    'Save resolution prices': 'Сохранить цены',
    'Search image models': 'Поиск моделей изображений',
    'Set the base per-image price for each resolution tier.':
      'Задайте базовую цену за изображение для каждого разрешения.',
    'per image': 'за изображение',
    'Thinking Tokens': 'Токены рассуждений',
    'Text Output Tokens': 'Токены текстового вывода',
    'Thinking Billing': 'Тарификация рассуждений',
    'Thinking tokens are included in output tokens and are not billed twice.':
      'Токены рассуждений включены в выходные токены и не оплачиваются дважды.',
    'Thinking Type': 'Тип рассуждений',
    'Web Search Calls': 'Вызовы веб-поиска',
    'Web Search Unit Price': 'Тариф веб-поиска',
    'Web Search Fee': 'Плата за веб-поиск',
    'File Search Calls': 'Вызовы поиска по файлам',
    'File Search Unit Price': 'Тариф поиска по файлам',
    'File Search Fee': 'Плата за поиск по файлам',
    'View response': 'Показать ответ',
    'Test response': 'Ответ теста',
    'Response content': 'Содержимое ответа',
    'Response content was truncated to 8 KB.':
      'Содержимое ответа сокращено до 8 КБ.',
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
    '{{count}} available groups': 'Доступно групп: {{count}}',
    'All protocol routes': 'Все маршруты протоколов',
    'Claude messages routes': 'Маршруты сообщений Claude',
    'Configure API key access groups and routing capabilities.':
      'Настройте группы доступа и возможности маршрутизации API-ключа.',
    'Filter available routes by protocol':
      'Фильтровать доступные маршруты по протоколу',
    'Gemini native routes': 'Нативные маршруты Gemini',
    'Group capabilities are derived from active channels':
      'Возможности группы определяются активными каналами',
    'Image generation routes': 'Маршруты генерации изображений',
    'OpenAI compatible routes': 'Маршруты, совместимые с OpenAI',
    'Protocol routing': 'Маршрутизация протоколов',
    'Search groups, descriptions, or API endpoints':
      'Поиск групп, описаний или конечных точек API',
    'Set the key name and accessible model groups':
      'Задайте имя ключа и доступные группы моделей',
    'Video generation routes': 'Маршруты генерации видео',
    'Video tier price overrides': 'Переопределения тарифов видео',
    'JSON map of complete per-model video tier overrides. Use the video tier prices tab for guided editing.':
      'Полная JSON-карта тарифов видео по моделям. Для настройки используйте вкладку тарифов видео.',
    'Video tier prices': 'Тарифы видео по уровням',
    'Configure exact prices by resolution. Models without an override continue to follow their base price.':
      'Задайте цены по разрешениям. Модели без переопределения используют базовую цену.',
    'Search video models': 'Поиск видеомоделей',
    'No video pricing models found': 'Видеомодели с тарифами не найдены',
    'Clear the search or enable a supported video model first.':
      'Очистите поиск или сначала включите поддерживаемую видеомодель.',
    'Billing unit': 'Единица тарификации',
    'Price source': 'Источник цены',
    'Resolution tiers': 'Уровни разрешения',
    'Per second': 'За секунду',
    'Per successful task': 'За успешную задачу',
    'Per 1M video tokens': 'За 1 млн видеотокенов',
    Explicit: 'Задано вручную',
    Inherited: 'Унаследовано',
    'Edit video tier prices': 'Изменить тарифы видео',
    'Set an independent price for every supported video tier.':
      'Задайте отдельную цену для каждого поддерживаемого уровня видео.',
    'Explicit tier prices': 'Отдельные цены уровней',
    From: 'От',
    'With reference video': 'С референсным видео',
    'per second': 'за секунду',
    'per successful task': 'за успешную задачу',
    'per 1M video tokens': 'за 1 млн видеотокенов',
    'Not applicable': 'Не применяется',
    'Use inherited prices': 'Использовать унаследованные цены',
    'Save tier prices': 'Сохранить цены уровней',
    'Use inherited video prices?': 'Использовать унаследованные цены видео?',
    'This removes the independent tier prices for this model and returns every resolution to base-price scaling.':
      'Отдельные цены этой модели будут удалены, а все разрешения снова будут рассчитываться от базовой цены.',
    'Must be greater than zero': 'Значение должно быть больше нуля',
  },
  vi: {
    'Ignore client max_output_tokens': 'Bỏ qua max_output_tokens của máy khách',
    'Remove max_output_tokens from Responses requests before forwarding them upstream':
      'Xóa max_output_tokens khỏi yêu cầu Responses trước khi chuyển tiếp lên nhà cung cấp',
    '1K price': 'Giá 1K',
    '2K price': 'Giá 2K',
    '4K price': 'Giá 4K',
    'Automatic resolution billing': 'Tự động tính phí theo độ phân giải',
    'Base price': 'Giá cơ sở',
    'Clear the search or enable a supported image model first.':
      'Xóa tìm kiếm hoặc bật một mô hình hình ảnh được hỗ trợ trước.',
    'Configure 1K, 2K, and 4K prices under one official image model. The requested size selects the billing tier automatically.':
      'Cấu hình giá 1K, 2K và 4K trong một mô hình hình ảnh chính thức. Kích thước yêu cầu tự động chọn bậc tính phí.',
    'Default tier': 'Bậc mặc định',
    'Edit image resolution prices': 'Sửa giá độ phân giải hình ảnh',
    'Higher tiers cannot cost less than lower tiers':
      'Bậc cao hơn không được có giá thấp hơn bậc thấp',
    'Image resolution price policies': 'Chính sách giá độ phân giải hình ảnh',
    'Image resolution prices': 'Giá theo độ phân giải hình ảnh',
    'JSON map of complete per-model 1K, 2K, and 4K price policies. Use the image resolution prices tab for guided editing.':
      'Ánh xạ JSON của chính sách giá 1K, 2K và 4K đầy đủ theo mô hình. Dùng thẻ giá độ phân giải để chỉnh sửa trực quan.',
    'No image pricing models found': 'Không tìm thấy mô hình định giá hình ảnh',
    'Per image': 'Theo hình ảnh',
    'Price per image': 'Giá mỗi hình ảnh',
    Resolution: 'Độ phân giải',
    'Save resolution prices': 'Lưu giá độ phân giải',
    'Search image models': 'Tìm mô hình hình ảnh',
    'Set the base per-image price for each resolution tier.':
      'Đặt giá cơ sở mỗi hình ảnh cho từng bậc độ phân giải.',
    'per image': 'mỗi hình ảnh',
    'Thinking Tokens': 'Token suy luận',
    'Text Output Tokens': 'Token đầu ra văn bản',
    'Thinking Billing': 'Tính phí suy luận',
    'Thinking tokens are included in output tokens and are not billed twice.':
      'Token suy luận đã nằm trong token đầu ra và không bị tính phí hai lần.',
    'Thinking Type': 'Kiểu suy luận',
    'Web Search Calls': 'Số lần tìm kiếm web',
    'Web Search Unit Price': 'Đơn giá tìm kiếm web',
    'Web Search Fee': 'Phí tìm kiếm web',
    'File Search Calls': 'Số lần tìm kiếm tệp',
    'File Search Unit Price': 'Đơn giá tìm kiếm tệp',
    'File Search Fee': 'Phí tìm kiếm tệp',
    'View response': 'Xem phản hồi',
    'Test response': 'Phản hồi kiểm thử',
    'Response content': 'Nội dung phản hồi',
    'Response content was truncated to 8 KB.':
      'Nội dung phản hồi đã được rút gọn còn 8 KB.',
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
    '{{count}} available groups': '{{count}} nhóm khả dụng',
    'All protocol routes': 'Tất cả tuyến giao thức',
    'Claude messages routes': 'Tuyến tin nhắn Claude',
    'Configure API key access groups and routing capabilities.':
      'Cấu hình nhóm truy cập và khả năng định tuyến của khóa API.',
    'Filter available routes by protocol': 'Lọc tuyến khả dụng theo giao thức',
    'Gemini native routes': 'Tuyến gốc Gemini',
    'Group capabilities are derived from active channels':
      'Khả năng nhóm được lấy từ các kênh đang hoạt động',
    'Image generation routes': 'Tuyến tạo ảnh',
    'OpenAI compatible routes': 'Tuyến tương thích OpenAI',
    'Protocol routing': 'Định tuyến giao thức',
    'Search groups, descriptions, or API endpoints':
      'Tìm nhóm, mô tả hoặc điểm cuối API',
    'Set the key name and accessible model groups':
      'Đặt tên khóa và các nhóm mô hình có thể truy cập',
    'Video generation routes': 'Tuyến tạo video',
    'Video tier price overrides': 'Giá video ghi đè theo mức',
    'JSON map of complete per-model video tier overrides. Use the video tier prices tab for guided editing.':
      'Bản đồ JSON đầy đủ của giá video theo từng mô hình. Hãy chỉnh sửa trong thẻ giá video theo mức.',
    'Video tier prices': 'Giá video theo mức',
    'Configure exact prices by resolution. Models without an override continue to follow their base price.':
      'Đặt giá theo độ phân giải. Mô hình không ghi đè tiếp tục dùng giá cơ sở.',
    'Search video models': 'Tìm mô hình video',
    'No video pricing models found': 'Không tìm thấy mô hình video có giá',
    'Clear the search or enable a supported video model first.':
      'Xóa tìm kiếm hoặc bật một mô hình video được hỗ trợ trước.',
    'Billing unit': 'Đơn vị tính phí',
    'Price source': 'Nguồn giá',
    'Resolution tiers': 'Mức độ phân giải',
    'Per second': 'Theo giây',
    'Per successful task': 'Theo tác vụ thành công',
    'Per 1M video tokens': 'Theo 1 triệu token video',
    Explicit: 'Thiết lập riêng',
    Inherited: 'Kế thừa',
    'Edit video tier prices': 'Sửa giá video theo mức',
    'Set an independent price for every supported video tier.':
      'Đặt giá riêng cho từng mức video được hỗ trợ.',
    'Explicit tier prices': 'Giá riêng theo mức',
    From: 'Từ',
    'With reference video': 'Có video tham chiếu',
    'per second': 'mỗi giây',
    'per successful task': 'mỗi tác vụ thành công',
    'per 1M video tokens': 'mỗi 1 triệu token video',
    'Not applicable': 'Không áp dụng',
    'Use inherited prices': 'Dùng giá kế thừa',
    'Save tier prices': 'Lưu giá theo mức',
    'Use inherited video prices?': 'Dùng giá video kế thừa?',
    'This removes the independent tier prices for this model and returns every resolution to base-price scaling.':
      'Thao tác này xóa giá riêng của mô hình và đưa mọi độ phân giải về cách tính theo giá cơ sở.',
    'Must be greater than zero': 'Phải lớn hơn 0',
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
