<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Download, Refresh, View } from '@element-plus/icons-vue'
import { buildBillingRows, entryDate } from '../billing/aggregate'
import { buildReportBlob as buildCanvasReportBlob, createReportCanvas as createCanvasReport } from '../billing/reportCanvas'
import { localDateKey } from '../utils/format'

const props = defineProps({
  entries: {
    type: Array,
    default: () => [],
  },
  dailyUsage: {
    type: Array,
    default: () => [],
  },
  availableDates: {
    type: Array,
    default: () => [],
  },
  selectedDate: {
    type: String,
    default: '',
  },
  formatNumber: {
    type: Function,
    required: true,
  },
})

const emit = defineEmits(['refresh', 'date-change'])

const selectedDate = ref(props.selectedDate || localDateKey())
const selectedTemplate = ref('poster')
const selectedLanguage = ref('zh')
const previewVisible = ref(false)
const previewBusy = ref(false)
const reportPreviewUrl = ref('')
const reportPreviewBlob = ref(null)

const billingLanguages = [
  { key: 'zh', label: '中文' },
  { key: 'en', label: 'English' },
]

const reportTemplates = [
  {
    key: 'poster',
    label: '费用海报',
    badge: 'POSTER',
    description: '大数字、强排版，适合保存当天用量截图。',
    previewTitle: '费用海报',
    previewSubtitle: '把一天的模型开销做成分享图',
    note: '偏视觉展示的模拟账单，不作为正式对账凭证。',
  },
  {
    key: 'neon',
    label: '午夜霓虹',
    badge: 'NEON',
    description: '深色背景和高亮数据，适合夜间主题。',
    previewTitle: '午夜霓虹',
    previewSubtitle: '更像一张模型消费战报',
    note: '深色霓虹模板突出总额、Token 与模型排行。',
  },
  {
    key: 'receipt',
    label: '复古小票',
    badge: 'RECEIPT',
    description: '像便利店小票一样轻松，有点生活感。',
    previewTitle: '复古小票',
    previewSubtitle: '把 Token 消耗打印成一张小票',
    note: '小票模板用于趣味记录，只展示本地估算结果。',
  },
  {
    key: 'standard',
    label: '清爽卡片',
    badge: 'CARD',
    description: '留白多、信息完整，适合普通保存。',
    previewTitle: '模拟账单卡',
    previewSubtitle: 'OmniProxy local usage snapshot',
    note: '卡片模板包含编号、日期、模型明细与合计金额。',
  },
  {
    key: 'summary',
    label: '能量仪表',
    badge: 'METER',
    description: '更像今日仪表盘，突出 Token 能量条。',
    previewTitle: '能量仪表',
    previewSubtitle: '当天模型调用的状态面板',
    note: '仪表模板突出总额、Token、请求数和主要模型占比。',
  },
  {
    key: 'ledger',
    label: '数据长图',
    badge: 'DATA',
    description: '保留表格秩序，但不做正式对账用途。',
    previewTitle: '数据长图',
    previewSubtitle: '模型、Token 和估算金额一屏看完',
    note: '数据模板更紧凑，优先展示更多模型明细和合计。',
  },
]

const englishTemplateText = {
  poster: {
    label: 'Cost Poster',
    description: 'Large numbers and bold layout for daily usage snapshots.',
    previewTitle: 'Cost Poster',
    previewSubtitle: 'Turn a day of model spend into a shareable card',
    note: 'A visual mock bill for sharing, not an official settlement document.',
  },
  neon: {
    label: 'Midnight Neon',
    description: 'Dark background and bright data highlights for night mode.',
    previewTitle: 'Midnight Neon',
    previewSubtitle: 'A battle-report style view of model spend',
    note: 'Neon template highlights total cost, tokens, and model ranking.',
  },
  receipt: {
    label: 'Retro Receipt',
    description: 'A casual receipt-style record with a bit of personality.',
    previewTitle: 'Retro Receipt',
    previewSubtitle: 'Print token usage like a tiny store receipt',
    note: 'Receipt template is for fun local records and estimated results only.',
  },
  standard: {
    label: 'Clean Card',
    description: 'More whitespace and complete information for regular saving.',
    previewTitle: 'Mock Bill Card',
    previewSubtitle: 'OmniProxy local usage snapshot',
    note: 'Card template includes ID, date, model details, and total estimate.',
  },
  summary: {
    label: 'Energy Meter',
    description: 'Dashboard-like view focused on token energy bars.',
    previewTitle: 'Energy Meter',
    previewSubtitle: 'A status panel for daily model calls',
    note: 'Meter template highlights total, tokens, requests, and top models.',
  },
  ledger: {
    label: 'Data Longform',
    description: 'Keeps table order without implying formal reconciliation.',
    previewTitle: 'Data Longform',
    previewSubtitle: 'Models, tokens, and estimated cost in one image',
    note: 'Data template is more compact and shows more model rows.',
  },
}

const activeTemplate = computed(
  () => reportTemplates.find((template) => template.key === selectedTemplate.value) || reportTemplates[0],
)
const activeBillTemplate = computed(
  () => ({
    ...activeTemplate.value,
    ...(selectedLanguage.value === 'en' ? englishTemplateText[activeTemplate.value.key] || {} : {}),
  }),
)

function billText(zh, en) {
  return selectedLanguage.value === 'en' ? en : zh
}

function formatBillText(template, values) {
  return String(template || '').replace(/\{(\w+)\}/g, (_, key) => values?.[key] ?? '')
}

const availableDates = computed(() => {
  const dates = new Set([localDateKey()])
  for (const date of props.availableDates || []) {
    if (date) dates.add(String(date))
  }
  for (const entry of props.entries || []) {
    const date = entryDate(entry)
    if (date) dates.add(date)
  }
  return [...dates].sort((a, b) => b.localeCompare(a)).slice(0, 30)
})

watch(
  () => props.selectedDate,
  (value) => {
    if (value && value !== selectedDate.value) {
      selectedDate.value = value
    }
  },
)

watch(selectedDate, (value) => {
  if (value && value !== props.selectedDate) {
    emit('date-change', value)
  }
})

watch([selectedDate, selectedTemplate, selectedLanguage], () => {
  if (previewVisible.value) {
    closeReportPreview()
  } else {
    resetReportPreview()
  }
})

const rawRows = computed(() => buildBillingRows(props.entries, selectedDate.value, props.dailyUsage))
const billingRows = computed(() => rawRows.value.filter((row) => row.price && row.totalTokens > 0))
const ignoredRows = computed(() => rawRows.value.filter((row) => !row.price || row.totalTokens <= 0))
const pricedRows = computed(() => billingRows.value)
const topRows = computed(() =>
  [...pricedRows.value]
    .sort((a, b) => b.cost - a.cost || b.totalTokens - a.totalTokens)
    .slice(0, 5),
)
const totals = computed(() => {
  const byCurrency = new Map()
  let inputTokens = 0
  let outputTokens = 0
  let totalTokens = 0
  let requestCount = 0
  for (const row of billingRows.value) {
    inputTokens += row.inputTokens
    outputTokens += row.outputTokens
    totalTokens += row.totalTokens
    requestCount += row.requestCount
    byCurrency.set(row.currency, (byCurrency.get(row.currency) || 0) + row.cost)
  }
  return {
    inputTokens,
    outputTokens,
    totalTokens,
    requestCount,
    byCurrency: [...byCurrency.entries()].map(([currency, value]) => ({ currency, value })),
  }
})
const totalCostText = computed(() => {
  if (!totals.value.byCurrency.length) return '暂无可计价用量'
  return totals.value.byCurrency.map((item) => formatMoney(item.value, item.currency)).join(' + ')
})
const billTotalCostText = computed(() => {
  if (!totals.value.byCurrency.length) return billText('暂无可计价用量', 'No billable usage')
  return totals.value.byCurrency.map((item) => formatMoney(item.value, item.currency)).join(' + ')
})
const ignoredTokenTotal = computed(() => ignoredRows.value.reduce((sum, row) => sum + row.totalTokens, 0))
const ignoredPreviewRows = computed(() => ignoredRows.value.slice(0, 3))
const statementId = computed(() => `OP-${selectedDate.value.replaceAll('-', '')}`)
const invoiceNumber = computed(() => `INV-${selectedDate.value}-${invoiceSuffix(selectedDate.value)}`)
const invoiceDateText = computed(() => formatDateLong(selectedDate.value))
const generatedAtText = computed(() =>
  new Intl.DateTimeFormat(selectedLanguage.value === 'en' ? 'en-US' : 'zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date()),
)

function formatMoney(value, currency) {
  const symbol = currency === 'CNY' ? '¥' : '$'
  return `${symbol}${Number(value || 0).toFixed(4)}`
}

function formatInvoiceMoney(value, currency) {
  const symbol = currency === 'CNY' ? '¥' : '$'
  const amount = Number(value || 0)
  const decimals = Math.abs(amount) >= 100 ? 2 : 4
  return `${symbol}${amount.toFixed(decimals)} ${currency || ''}`.trim()
}

function invoiceSuffix(value) {
  return String(value || '')
    .split('')
    .reduce((sum, char) => sum + char.charCodeAt(0), 0)
    .toString()
    .padStart(4, '0')
    .slice(-4)
}

function formatDateLong(dateValue) {
  const parsed = new Date(`${dateValue}T00:00:00`)
  if (Number.isNaN(parsed.getTime())) return dateValue
  return new Intl.DateTimeFormat(selectedLanguage.value === 'en' ? 'en-US' : 'zh-CN', {
    year: 'numeric',
    month: selectedLanguage.value === 'en' ? 'long' : '2-digit',
    day: 'numeric',
  }).format(parsed)
}

function totalInvoiceLines() {
  if (!totals.value.byCurrency.length) return [billText('暂无可计价用量', 'No billable usage')]
  return totals.value.byCurrency.map((item) => formatInvoiceMoney(item.value, item.currency))
}

function priceRateText(row) {
  return `${formatMoney(row.price.input, row.currency)} / ${formatMoney(row.price.output, row.currency)}`
}

function rowCostText(row) {
  return formatMoney(row.cost, row.currency)
}

function rowCostShare(row) {
  const maxCost = Math.max(...topRows.value.map((item) => item.cost), 0)
  if (!maxCost) return '8%'
  return `${Math.max(8, Math.round((row.cost / maxCost) * 100))}%`
}

async function exportReportImage() {
  await openReportPreview()
}

function createReportCanvas(templateKey = selectedTemplate.value) {
  return createCanvasReport(drawBillingReport, { templateKey })
}

async function buildReportBlob(templateKey = selectedTemplate.value) {
  return buildCanvasReportBlob(drawBillingReport, { templateKey })
}

async function openReportPreview() {
  previewBusy.value = true
  try {
    resetReportPreview()
    const blob = await buildReportBlob()
    if (!blob) return
    reportPreviewBlob.value = blob
    reportPreviewUrl.value = URL.createObjectURL(blob)
    previewVisible.value = true
  } finally {
    previewBusy.value = false
  }
}

function closeReportPreview() {
  previewVisible.value = false
  resetReportPreview()
}

function resetReportPreview() {
  if (reportPreviewUrl.value) {
    URL.revokeObjectURL(reportPreviewUrl.value)
  }
  reportPreviewUrl.value = ''
  reportPreviewBlob.value = null
}

async function savePreviewImage() {
  const blob = reportPreviewBlob.value
  if (!blob) return
  const filename = reportFileName()
  if (window.showSaveFilePicker) {
    try {
      const handle = await window.showSaveFilePicker({
        suggestedName: filename,
        types: [
          {
            description: 'PNG 图片',
            accept: { 'image/png': ['.png'] },
          },
        ],
      })
      const writable = await handle.createWritable()
      await writable.write(blob)
      await writable.close()
      closeReportPreview()
      return
    } catch (error) {
      if (error?.name === 'AbortError') return
      console.warn('保存账单图片失败，回退到浏览器下载', error)
    }
  }
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  closeReportPreview()
}

function reportFileName() {
  return `omniproxy-billing-${selectedDate.value}-${selectedTemplate.value}-${selectedLanguage.value}.png`
}

onBeforeUnmount(() => {
  previewVisible.value = false
  resetReportPreview()
})

function drawBillingReport(ctx, width, height, templateKey = selectedTemplate.value) {
  if (templateKey === 'poster') {
    drawPosterReport(ctx, width, height)
    return
  }
  if (templateKey === 'neon') {
    drawNeonReport(ctx, width, height)
    return
  }
  if (templateKey === 'receipt') {
    drawReceiptReport(ctx, width, height)
    return
  }
  if (templateKey === 'summary') {
    drawSummaryReport(ctx, width, height)
    return
  }
  if (templateKey === 'ledger') {
    drawLedgerReport(ctx, width, height)
    return
  }
  drawStandardReport(ctx, width, height)
}

function drawStandardReport(ctx, width, height) {
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, width, height)

  const pageX = 60
  const pageW = width - 120
  const muted = '#4b5563'
  const text = '#111111'
  const line = '#d8dee6'
  const font = '"Segoe UI", "Microsoft YaHei", Arial'

  drawInvoiceLogo(ctx, pageX, 78)
  ctx.fillStyle = text
  ctx.font = `700 44px ${font}`
  ctx.fillText('OmniProxy', pageX + 82, 104)
  ctx.font = `700 42px ${font}`
  ctx.textAlign = 'right'
  ctx.fillText(billText('模拟账单', 'Mock Bill'), pageX + pageW, 104)
  ctx.textAlign = 'left'

  ctx.font = `700 21px ${font}`
  ctx.fillText('OmniProxy Local', pageX, 208)
  ctx.font = `400 21px ${font}`
  ctx.fillStyle = text
  ctx.fillText(billText('本地模型用量快照', 'Local model usage snapshot'), pageX, 252)
  ctx.fillText(billText('来自请求历史', 'Generated from request history'), pageX, 294)
  ctx.fillText(billText('仅作模拟展示', 'Simulated bill only'), pageX, 336)

  const metaX = pageX + 680
  drawInvoicePair(ctx, metaX, 218, billText('模拟编号:', 'Mock Number:'), invoiceNumber.value)
  drawInvoicePair(ctx, metaX, 280, billText('用量日期:', 'Usage Date:'), invoiceDateText.value)
  drawInvoicePair(ctx, metaX, 342, billText('来源:', 'Source:'), billText('本地价格表', 'Local price table'))
  drawInvoicePair(ctx, metaX, 404, billText('类型:', 'Type:'), billText('模拟', 'Simulated'))

  drawLine(ctx, pageX, 470, pageX + pageW, 470, line)

  ctx.fillStyle = text
  ctx.font = `700 22px ${font}`
  ctx.fillText(billText('记录对象', 'Snapshot For'), pageX, 538)
  ctx.font = `400 21px ${font}`
  const clientNames = [...new Set(billingRows.value.flatMap((row) => row.clients || []))].filter(Boolean)
  ctx.fillText(clientNames[0] || billText('OmniProxy 用户', 'OmniProxy User'), pageX, 596)
  ctx.fillText(billText(`记录 ${statementId.value}`, `Statement ${statementId.value}`), pageX, 638)
  ctx.fillText(billText('本地工作区', 'Local workspace'), pageX, 680)

  const infoX = pageX + 530
  ctx.font = `700 22px ${font}`
  ctx.fillText(billText('用量信息', 'Usage Information'), infoX, 538)
  drawInvoicePair(ctx, infoX, 596, billText('用量日期:', 'Usage Date:'), invoiceDateText.value, 250)
  drawInvoicePair(ctx, infoX, 650, billText('请求数:', 'Requests:'), props.formatNumber(totals.value.requestCount), 250)
  drawInvoicePair(ctx, infoX, 704, billText('总 Token:', 'Total Tokens:'), props.formatNumber(totals.value.totalTokens), 250)

  const detailX = pageX
  const detailY = 780
  const detailW = pageW
  const rowH = 78
  const rows = billingRows.value.slice(0, 5)
  const visibleRowCount = Math.max(1, rows.length)
  const detailH = 166 + visibleRowCount * rowH + 222
  drawRoundRect(ctx, detailX, detailY, detailW, detailH, 8, '#ffffff', line)

  ctx.fillStyle = text
  ctx.font = `700 23px ${font}`
  ctx.fillText(billText('模拟明细', 'Simulated Details'), detailX + 36, detailY + 62)

  const headerY = detailY + 96
  drawRoundRect(ctx, detailX + 6, headerY, detailW - 12, 60, 7, '#f8fafc', line)
  ctx.fillStyle = '#1f2937'
  ctx.font = `500 18px ${font}`
  ctx.fillText(billText('模型', 'Description'), detailX + 36, headerY + 38)
  ctx.fillText(billText('Token', 'Quantity'), detailX + 575, headerY + 38)
  ctx.fillText(billText('单价', 'Unit Price'), detailX + 770, headerY + 38)
  ctx.textAlign = 'right'
  ctx.fillText(billText('金额', 'Amount'), detailX + detailW - 34, headerY + 38)
  ctx.textAlign = 'left'

  if (!rows.length) {
    ctx.fillStyle = muted
    ctx.font = `400 22px ${font}`
    ctx.fillText(billText('所选日期暂无可计价模型用量。', 'No metered model usage for this date.'), detailX + 36, headerY + 118)
  }

  rows.forEach((row, index) => {
    const y = headerY + 60 + index * rowH
    drawLine(ctx, detailX + 24, y, detailX + detailW - 24, y, '#edf0f4')
    ctx.fillStyle = text
    ctx.font = `600 21px ${font}`
    ctx.fillText(truncateText(ctx, row.model, 470), detailX + 36, y + 31)
    ctx.fillStyle = muted
    ctx.font = `400 17px ${font}`
    ctx.fillText(
      billText(
        `${row.requestCount} 次请求 · 输入 ${props.formatNumber(row.inputTokens)} · 输出 ${props.formatNumber(row.outputTokens)}`,
        `${row.requestCount} requests · input ${props.formatNumber(row.inputTokens)} · output ${props.formatNumber(row.outputTokens)}`,
      ),
      detailX + 36,
      y + 58,
    )

    ctx.fillStyle = text
    ctx.font = `400 20px ${font}`
    ctx.fillText(props.formatNumber(row.totalTokens), detailX + 575, y + 42)
    ctx.fillStyle = muted
    ctx.font = `400 16px ${font}`
    ctx.fillText('tokens', detailX + 575, y + 64)

    ctx.fillStyle = text
    ctx.font = `400 18px ${font}`
    if (row.price) {
      ctx.fillText(`${billText('入', 'In')} ${formatMoney(row.price.input, row.currency)}`, detailX + 770, y + 32)
      ctx.fillText(`${billText('出', 'Out')} ${formatMoney(row.price.output, row.currency)}`, detailX + 770, y + 58)
    } else {
      ctx.fillText(billText('待定', 'Pending'), detailX + 770, y + 42)
    }

    ctx.textAlign = 'right'
    ctx.fillStyle = text
    ctx.font = row.price ? `600 21px ${font}` : `400 19px ${font}`
    ctx.fillText(row.price ? formatInvoiceMoney(row.cost, row.currency) : billText('待定', 'Pending'), detailX + detailW - 34, y + 43)
    ctx.textAlign = 'left'
  })

  const subtotalY = headerY + 60 + visibleRowCount * rowH + 42
  drawLine(ctx, detailX + 24, subtotalY - 24, detailX + detailW - 24, subtotalY - 24, line)
  const amountX = detailX + detailW - 34
  ctx.fillStyle = text
  ctx.font = `400 20px ${font}`
  ctx.fillText(billText('小计', 'Subtotal'), detailX + 575, subtotalY + 8)
  drawAmountLines(ctx, totalInvoiceLines(), amountX, subtotalY + 8, `400 20px ${font}`, text, 28)
  ctx.fillText(billText('税费 (0%)', 'Tax (0%)'), detailX + 575, subtotalY + 76)
  ctx.textAlign = 'right'
  ctx.fillText('0.0000', amountX, subtotalY + 76)
  ctx.textAlign = 'left'

  drawLine(ctx, detailX + 570, subtotalY + 106, detailX + detailW - 24, subtotalY + 106, line)
  ctx.font = `700 24px ${font}`
  ctx.fillText(billText('合计', 'Total'), detailX + 575, subtotalY + 164)
  drawAmountLines(ctx, totalInvoiceLines(), amountX, subtotalY + 164, `700 24px ${font}`, text, 32)

  const statusY = detailY + detailH + 32
  drawRoundRect(ctx, pageX, statusY, pageW, 142, 8, '#ffffff', line)
  drawCheckIcon(ctx, pageX + 62, statusY + 72)
  ctx.fillStyle = text
  ctx.font = `700 23px ${font}`
  ctx.fillText(billText('模拟账单已生成', 'Mock Bill Generated'), pageX + 126, statusY + 58)
  ctx.font = `400 19px ${font}`
  ctx.fillText(billText('这张图片用于本地用量回顾和轻量分享。', 'This image is made for local usage review and casual sharing.'), pageX + 126, statusY + 98)

  const footerY = statusY + 186
  drawLine(ctx, pageX, footerY, pageX + pageW, footerY, line)
  ctx.fillStyle = text
  ctx.font = `400 18px ${font}`
  ctx.fillText(billText('只包含已匹配本地价格表的模型。', 'Only models matched by the local price table are included.'), pageX, footerY + 48)
  ctx.fillStyle = muted
  ctx.font = `400 17px ${font}`
  ctx.textAlign = 'center'
  ctx.fillText(billText('不是官方服务商账单，可作为本地用量快照。', 'Not an official provider bill. Use it as a playful local snapshot.'), width / 2, footerY + 112)
  ctx.fillText(
    `${billText('生成于', 'Generated')} ${generatedAtText.value} · ${billText('OmniProxy 模拟账单', 'OmniProxy simulated bill')}`,
    width / 2,
    footerY + 150,
  )
  ctx.textAlign = 'left'
}

function drawPosterReport(ctx, width, height) {
  const font = '"Segoe UI", "Microsoft YaHei", Arial'
  const pageX = 64
  const pageW = width - 128
  const rows = topRows.value.slice(0, 5)
  const text = '#16133a'
  const muted = '#5f5a7d'
  const accent = '#f97316'
  const blue = '#4f46e5'
  const mint = '#14b8a6'
  const yellow = '#facc15'
  const maxCost = Math.max(...rows.map((row) => row.cost), 0.0001)

  ctx.fillStyle = '#eef2ff'
  ctx.fillRect(0, 0, width, height)
  drawRoundRect(ctx, 42, 42, width - 84, height - 84, 8, '#fffdf8', '#14112c')

  ctx.fillStyle = blue
  ctx.fillRect(pageX, 92, pageW, 24)
  ctx.fillStyle = accent
  ctx.fillRect(pageX, 116, pageW * 0.42, 24)
  ctx.fillStyle = mint
  ctx.fillRect(pageX + pageW * 0.42, 116, pageW * 0.34, 24)
  ctx.fillStyle = yellow
  ctx.fillRect(pageX + pageW * 0.76, 116, pageW * 0.24, 24)

  ctx.fillStyle = text
  ctx.font = `800 36px ${font}`
  ctx.fillText('OMNIPROXY', pageX, 214)
  drawPill(ctx, pageX + pageW - 240, 170, 240, 54, billText('模拟账单', 'SIMULATED BILL'), '#16133a', '#ffffff', `700 18px ${font}`)

  ctx.font = `900 76px ${font}`
  ctx.fillText(billText('费用海报', 'Cost Poster'), pageX, 322)
  ctx.fillStyle = muted
  ctx.font = `400 24px ${font}`
  ctx.fillText(
    billText(`${selectedDate.value} 的本地模型用量快照`, `${selectedDate.value} local model usage snapshot`),
    pageX,
    366,
  )

  drawRoundRect(ctx, pageX, 430, pageW, 288, 8, '#16133a')
  ctx.fillStyle = '#ffffff'
  ctx.font = `600 24px ${font}`
  ctx.fillText(billText('今日模拟费用', 'Today Mock Cost'), pageX + 42, 500)
  drawAmountLines(ctx, totalInvoiceLines(), pageX + pageW - 42, 520, `900 62px ${font}`, '#ffffff', 68)
  ctx.fillStyle = '#c7d2fe'
  ctx.font = `400 20px ${font}`
  ctx.textAlign = 'right'
  ctx.fillText(billText('按本地价格表估算，不作为正式账单', 'Estimated from local prices, not an official bill'), pageX + pageW - 42, 640)
  ctx.textAlign = 'left'

  const stats = [
    [billText('Token', 'Tokens'), props.formatNumber(totals.value.totalTokens), '#4f46e5'],
    [billText('请求数', 'Requests'), props.formatNumber(totals.value.requestCount), '#f97316'],
    [billText('输入', 'Input'), props.formatNumber(totals.value.inputTokens), '#14b8a6'],
    [billText('输出', 'Output'), props.formatNumber(totals.value.outputTokens), '#7c3aed'],
  ]
  stats.forEach(([label, value, color], index) => {
    const x = pageX + (index % 2) * ((pageW - 22) / 2 + 22)
    const y = 766 + Math.floor(index / 2) * 152
    const w = (pageW - 22) / 2
    drawRoundRect(ctx, x, y, w, 124, 8, '#ffffff', '#16133a')
    ctx.fillStyle = color
    ctx.fillRect(x, y, 12, 124)
    ctx.fillStyle = muted
    ctx.font = `700 18px ${font}`
    ctx.fillText(label, x + 30, y + 42)
    ctx.fillStyle = text
    ctx.font = `900 34px ${font}`
    ctx.fillText(value, x + 30, y + 86)
  })

  const rankY = 1128
  ctx.fillStyle = text
  ctx.font = `900 34px ${font}`
  ctx.fillText(billText('模型消费榜', 'Model Spend Rank'), pageX, rankY)
  ctx.fillStyle = muted
  ctx.font = `400 19px ${font}`
  ctx.fillText(billText('按估算费用排序，展示前 5 个模型', 'Sorted by estimated cost, top 5 models'), pageX, rankY + 36)

  if (!rows.length) {
    drawRoundRect(ctx, pageX, rankY + 86, pageW, 118, 8, '#ffffff', '#16133a')
    ctx.fillStyle = muted
    ctx.font = `400 24px ${font}`
    ctx.fillText(billText('这一天还没有可计价模型用量。', 'No billable model usage for this day.'), pageX + 34, rankY + 154)
  }

  rows.forEach((row, index) => {
    const y = rankY + 86 + index * 116
    drawRoundRect(ctx, pageX, y, pageW, 88, 8, index === 0 ? '#fff7ed' : '#ffffff', '#16133a')
    drawPill(ctx, pageX + 24, y + 24, 52, 40, String(index + 1).padStart(2, '0'), '#16133a', '#ffffff', `800 17px ${font}`)
    ctx.fillStyle = text
    ctx.font = `800 23px ${font}`
    ctx.fillText(truncateText(ctx, row.model, 440), pageX + 98, y + 38)
    ctx.fillStyle = muted
    ctx.font = `400 17px ${font}`
    ctx.fillText(
      billText(
        `${row.requestCount} 次请求 · ${props.formatNumber(row.totalTokens)} Token`,
        `${row.requestCount} requests · ${props.formatNumber(row.totalTokens)} tokens`,
      ),
      pageX + 98,
      y + 66,
    )
    const barX = pageX + 600
    const barW = 260
    drawRoundRect(ctx, barX, y + 36, barW, 12, 6, '#e5e7eb')
    drawRoundRect(ctx, barX, y + 36, Math.max(22, barW * (row.cost / maxCost)), 12, 6, [blue, accent, mint, '#7c3aed', yellow][index] || blue)
    ctx.textAlign = 'right'
    ctx.fillStyle = text
    ctx.font = `900 22px ${font}`
    ctx.fillText(rowCostText(row), pageX + pageW - 28, y + 52)
    ctx.textAlign = 'left'
  })

  drawLine(ctx, pageX, height - 190, pageX + pageW, height - 190, '#16133a')
  ctx.fillStyle = muted
  ctx.font = `400 19px ${font}`
  ctx.fillText(billText('这是一张模拟账单分享图，只反映本地记录与本地价格表估算。', 'This is a mock bill image based on local records and local price rules.'), pageX, height - 132)
  ctx.fillText(`${billText('生成于', 'Generated')} ${generatedAtText.value}`, pageX, height - 94)
}

function drawNeonReport(ctx, width, height) {
  const font = '"Segoe UI", "Microsoft YaHei", Arial'
  const pageX = 70
  const pageW = width - 140
  const rows = topRows.value.slice(0, 6)
  const maxTokens = Math.max(...rows.map((row) => row.totalTokens), 1)

  ctx.fillStyle = '#090d1f'
  ctx.fillRect(0, 0, width, height)
  ctx.strokeStyle = 'rgba(56, 189, 248, 0.16)'
  ctx.lineWidth = 1
  for (let x = 70; x < width; x += 70) drawLine(ctx, x, 0, x, height, 'rgba(56, 189, 248, 0.11)')
  for (let y = 70; y < height; y += 70) drawLine(ctx, 0, y, width, y, 'rgba(56, 189, 248, 0.08)')

  drawRoundRect(ctx, pageX, 76, pageW, height - 152, 8, 'rgba(10, 18, 42, 0.92)', '#22d3ee')
  drawPill(ctx, pageX + 44, 126, 176, 48, 'NEON MODE', '#22d3ee', '#06101f', `800 17px ${font}`)
  ctx.fillStyle = '#f8fafc'
  ctx.font = `900 62px ${font}`
  ctx.fillText(billText('午夜霓虹账单', 'Midnight Neon Bill'), pageX + 44, 260)
  ctx.fillStyle = '#93c5fd'
  ctx.font = `400 22px ${font}`
  ctx.fillText(billText(`${selectedDate.value} · 模拟用量报告`, `${selectedDate.value} · simulated usage report`), pageX + 48, 306)

  ctx.textAlign = 'right'
  ctx.fillStyle = '#67e8f9'
  ctx.font = `900 58px ${font}`
  drawAmountLines(ctx, totalInvoiceLines(), pageX + pageW - 44, 246, `900 58px ${font}`, '#67e8f9', 62)
  ctx.fillStyle = '#a5b4fc'
  ctx.font = `400 18px ${font}`
  ctx.fillText(billText('不用于结算', 'not for settlement'), pageX + pageW - 44, 322)
  ctx.textAlign = 'left'

  const cards = [
    ['TOKENS', props.formatNumber(totals.value.totalTokens), '#67e8f9'],
    ['REQUESTS', props.formatNumber(totals.value.requestCount), '#f0abfc'],
    [billText('IGNORED', 'IGNORED'), props.formatNumber(ignoredRows.value.length), '#fbbf24'],
  ]
  cards.forEach(([label, value, color], index) => {
    const x = pageX + 44 + index * ((pageW - 88 - 32) / 3 + 16)
    const y = 410
    const w = (pageW - 88 - 32) / 3
    drawRoundRect(ctx, x, y, w, 150, 8, 'rgba(15, 23, 42, 0.86)', color)
    ctx.fillStyle = color
    ctx.font = `800 18px ${font}`
    ctx.fillText(label, x + 22, y + 42)
    ctx.fillStyle = '#f8fafc'
    ctx.font = `900 30px ${font}`
    ctx.fillText(value, x + 22, y + 94)
    drawLine(ctx, x + 22, y + 118, x + w - 22, y + 118, 'rgba(255, 255, 255, 0.16)')
  })

  ctx.fillStyle = '#f8fafc'
  ctx.font = `800 30px ${font}`
  ctx.fillText(billText('模型信号强度', 'Model Signal Strength'), pageX + 44, 660)

  if (!rows.length) {
    drawRoundRect(ctx, pageX + 44, 710, pageW - 88, 110, 8, 'rgba(15, 23, 42, 0.86)', '#334155')
    ctx.fillStyle = '#94a3b8'
    ctx.font = `400 22px ${font}`
    ctx.fillText(billText('暂无可计价模型信号。', 'No billable model signal.'), pageX + 74, 774)
  }

  rows.forEach((row, index) => {
    const y = 720 + index * 135
    const color = ['#22d3ee', '#f0abfc', '#818cf8', '#34d399', '#fbbf24', '#fb7185'][index] || '#22d3ee'
    drawRoundRect(ctx, pageX + 44, y, pageW - 88, 104, 8, 'rgba(15, 23, 42, 0.88)', 'rgba(148, 163, 184, 0.42)')
    ctx.fillStyle = color
    ctx.font = `900 26px ${font}`
    ctx.fillText(String(index + 1).padStart(2, '0'), pageX + 70, y + 45)
    ctx.fillStyle = '#f8fafc'
    ctx.font = `800 22px ${font}`
    ctx.fillText(truncateText(ctx, row.model, 470), pageX + 130, y + 40)
    ctx.fillStyle = '#94a3b8'
    ctx.font = `400 17px ${font}`
    ctx.fillText(
      billText(
        `${props.formatNumber(row.totalTokens)} Token · ${row.requestCount} 次请求`,
        `${props.formatNumber(row.totalTokens)} tokens · ${row.requestCount} requests`,
      ),
      pageX + 130,
      y + 70,
    )
    const barX = pageX + 600
    const barW = 270
    drawRoundRect(ctx, barX, y + 45, barW, 10, 5, 'rgba(148, 163, 184, 0.24)')
    drawRoundRect(ctx, barX, y + 45, Math.max(24, barW * (row.totalTokens / maxTokens)), 10, 5, color)
    ctx.textAlign = 'right'
    ctx.fillStyle = color
    ctx.font = `900 22px ${font}`
    ctx.fillText(rowCostText(row), pageX + pageW - 66, y + 56)
    ctx.textAlign = 'left'
  })

  drawLine(ctx, pageX + 44, height - 230, pageX + pageW - 44, height - 230, 'rgba(103, 232, 249, 0.5)')
  ctx.fillStyle = '#94a3b8'
  ctx.font = `400 18px ${font}`
  ctx.fillText(billText('模拟账单图片，仅用于本地用量回顾和分享。', 'Mock bill image for local usage review and sharing only.'), pageX + 44, height - 170)
  ctx.fillText(`${billText('生成于', 'Generated')} ${generatedAtText.value}`, pageX + 44, height - 132)
}

function drawReceiptReport(ctx, width, height) {
  const font = '"SFMono-Regular", Consolas, "Microsoft YaHei", monospace'
  const receiptW = 760
  const receiptX = (width - receiptW) / 2
  const receiptY = 74
  const receiptH = height - 148
  const rows = billingRows.value.slice(0, 8)

  ctx.fillStyle = '#c7d2fe'
  ctx.fillRect(0, 0, width, height)
  drawRoundRect(ctx, receiptX, receiptY, receiptW, receiptH, 8, '#fffdf4', '#111827')
  ctx.fillStyle = '#111827'
  ctx.font = `900 42px ${font}`
  ctx.textAlign = 'center'
  ctx.fillText(billText('OMNIPROXY 小票', 'OMNIPROXY RECEIPT'), width / 2, receiptY + 86)
  ctx.font = `500 20px ${font}`
  ctx.fillText(billText('模拟小票 · 本地估算 · 保存好玩', 'Mock receipt · local estimate · just for fun'), width / 2, receiptY + 126)
  ctx.textAlign = 'left'

  drawDashedLine(ctx, receiptX + 52, receiptY + 176, receiptX + receiptW - 52, receiptY + 176, '#111827')
  ctx.font = `600 20px ${font}`
  ctx.fillText(`${billText('日期', 'DATE')}: ${selectedDate.value}`, receiptX + 58, receiptY + 226)
  ctx.fillText(`${billText('编号', 'NO')}: ${statementId.value}`, receiptX + 58, receiptY + 262)
  drawDashedLine(ctx, receiptX + 52, receiptY + 304, receiptX + receiptW - 52, receiptY + 304, '#111827')

  ctx.font = `700 20px ${font}`
  ctx.fillText(billText('模型', 'MODEL'), receiptX + 58, receiptY + 354)
  ctx.textAlign = 'right'
  ctx.fillText(billText('金额', 'AMOUNT'), receiptX + receiptW - 58, receiptY + 354)
  ctx.textAlign = 'left'

  if (!rows.length) {
    ctx.fillStyle = '#64748b'
    ctx.font = `500 22px ${font}`
    ctx.fillText(billText('暂无可计价用量', 'NO BILLABLE USAGE'), receiptX + 58, receiptY + 432)
    ctx.fillStyle = '#111827'
  }

  rows.forEach((row, index) => {
    const y = receiptY + 410 + index * 100
    ctx.font = `800 20px ${font}`
    ctx.fillStyle = '#111827'
    ctx.fillText(truncateText(ctx, row.model, 430), receiptX + 58, y)
    ctx.font = `500 16px ${font}`
    ctx.fillStyle = '#64748b'
    ctx.fillText(
      billText(
        `${row.requestCount} 次请求 / ${props.formatNumber(row.totalTokens)} tokens`,
        `${row.requestCount} requests / ${props.formatNumber(row.totalTokens)} tokens`,
      ),
      receiptX + 58,
      y + 30,
    )
    ctx.textAlign = 'right'
    ctx.font = `800 19px ${font}`
    ctx.fillStyle = '#111827'
    ctx.fillText(rowCostText(row), receiptX + receiptW - 58, y + 16)
    ctx.textAlign = 'left'
  })

  const totalY = Math.min(receiptY + 410 + Math.max(rows.length, 1) * 100 + 44, receiptY + receiptH - 360)
  drawDashedLine(ctx, receiptX + 52, totalY, receiptX + receiptW - 52, totalY, '#111827')
  ctx.font = `900 28px ${font}`
  ctx.fillText(billText('合计', 'TOTAL'), receiptX + 58, totalY + 70)
  drawAmountLines(ctx, totalInvoiceLines(), receiptX + receiptW - 58, totalY + 70, `900 28px ${font}`, '#111827', 36)

  const statsY = totalY + 150
  ctx.font = `600 18px ${font}`
  ctx.fillStyle = '#111827'
  ctx.fillText(`${billText('TOKEN ', 'TOKENS ')} ${props.formatNumber(totals.value.totalTokens)}`, receiptX + 58, statsY)
  ctx.fillText(`${billText('请求  ', 'CALLS  ')} ${props.formatNumber(totals.value.requestCount)}`, receiptX + 58, statsY + 34)
  ctx.fillText(`${billText('忽略  ', 'IGNORED')} ${props.formatNumber(ignoredRows.value.length)}`, receiptX + 58, statsY + 68)

  drawDashedLine(ctx, receiptX + 52, receiptY + receiptH - 190, receiptX + receiptW - 52, receiptY + receiptH - 190, '#111827')
  ctx.textAlign = 'center'
  ctx.font = `700 20px ${font}`
  ctx.fillText(billText('这是一张轻量模拟账单', 'THIS IS A PLAYFUL MOCK BILL'), width / 2, receiptY + receiptH - 130)
  ctx.font = `500 16px ${font}`
  ctx.fillStyle = '#64748b'
  ctx.fillText(`${billText('生成于', 'Generated')} ${generatedAtText.value}`, width / 2, receiptY + receiptH - 92)
  ctx.textAlign = 'left'
}

function drawSummaryReport(ctx, width, height) {
  const font = '"Segoe UI", "Microsoft YaHei", Arial'
  const pageX = 70
  const pageW = width - 140
  const text = '#111827'
  const muted = '#667085'
  const line = '#d9dee7'
  const rows = topRows.value.slice(0, 6)
  const maxTokens = Math.max(...rows.map((row) => row.totalTokens), 1)

  ctx.fillStyle = '#f4f6f8'
  ctx.fillRect(0, 0, width, height)
  drawRoundRect(ctx, pageX, 60, pageW, height - 120, 8, '#ffffff', line)

  ctx.fillStyle = text
  ctx.font = `700 42px ${font}`
  ctx.fillText(billText('OmniProxy 能量仪表', 'OmniProxy Energy Meter'), pageX + 44, 132)
  ctx.font = `400 21px ${font}`
  ctx.fillStyle = muted
  ctx.fillText(billText('把当天模型调用做成一张状态面板', 'Turn daily model calls into a status panel'), pageX + 44, 172)
  ctx.textAlign = 'right'
  ctx.fillStyle = text
  ctx.font = `600 22px ${font}`
  ctx.fillText(selectedDate.value, pageX + pageW - 44, 132)
  ctx.fillStyle = muted
  ctx.font = `400 18px ${font}`
  ctx.fillText(`${billText('生成于', 'Generated')} ${generatedAtText.value}`, pageX + pageW - 44, 170)
  ctx.textAlign = 'left'
  drawLine(ctx, pageX + 44, 222, pageX + pageW - 44, 222, line)

  ctx.fillStyle = muted
  ctx.font = `500 20px ${font}`
  ctx.fillText(billText('估算费用', 'Estimated Cost'), pageX + 44, 296)
  drawAmountLines(
    ctx,
    totals.value.byCurrency.length
      ? totals.value.byCurrency.map((item) => formatInvoiceMoney(item.value, item.currency))
      : [billText('暂无可计价用量', 'No billable usage')],
    pageX + pageW - 44,
    296,
    `800 56px ${font}`,
    text,
    62,
  )

  const metricCards = [
    [
      billText('总 Token', 'Total Tokens'),
      props.formatNumber(totals.value.totalTokens),
      billText(`输入 ${props.formatNumber(totals.value.inputTokens)}`, `Input ${props.formatNumber(totals.value.inputTokens)}`),
    ],
    [billText('输出 Token', 'Output Tokens'), props.formatNumber(totals.value.outputTokens), billText('按本地价格表估算', 'Priced by local table')],
    [billText('请求数', 'Requests'), props.formatNumber(totals.value.requestCount), `Statement ${statementId.value}`],
    [billText('未纳入模型', 'Ignored Models'), props.formatNumber(ignoredRows.value.length), `${props.formatNumber(ignoredTokenTotal.value)} Token`],
  ]
  metricCards.forEach(([label, value, hint], index) => {
    const col = index % 2
    const row = Math.floor(index / 2)
    const x = pageX + 44 + col * ((pageW - 112) / 2 + 24)
    const y = 410 + row * 158
    const w = (pageW - 112) / 2
    drawRoundRect(ctx, x, y, w, 126, 8, '#f8fafc', '#e3e8ef')
    ctx.fillStyle = muted
    ctx.font = `500 18px ${font}`
    ctx.fillText(label, x + 24, y + 36)
    ctx.fillStyle = text
    ctx.font = `800 32px ${font}`
    ctx.fillText(value, x + 24, y + 78)
    ctx.fillStyle = muted
    ctx.font = `400 17px ${font}`
    ctx.fillText(hint, x + 24, y + 106)
  })

  const listY = 770
  ctx.fillStyle = text
  ctx.font = `700 26px ${font}`
  ctx.fillText(billText('主要模型', 'Top Models'), pageX + 44, listY)
  ctx.fillStyle = muted
  ctx.font = `400 18px ${font}`
  ctx.fillText(billText('按估算费用和 Token 量排序', 'Sorted by estimated cost and token volume'), pageX + 44, listY + 34)

  if (!rows.length) {
    drawRoundRect(ctx, pageX + 44, listY + 78, pageW - 88, 110, 8, '#f8fafc', '#e3e8ef')
    ctx.fillStyle = muted
    ctx.font = `400 22px ${font}`
    ctx.fillText(billText('所选日期暂无可计价模型用量。', 'No billable model usage for this date.'), pageX + 74, listY + 142)
  }

  rows.forEach((row, index) => {
    const y = listY + 78 + index * 126
    const x = pageX + 44
    const w = pageW - 88
    drawRoundRect(ctx, x, y, w, 102, 8, '#ffffff', '#e3e8ef')
    ctx.fillStyle = text
    ctx.font = `700 22px ${font}`
    ctx.fillText(truncateText(ctx, row.model, 520), x + 26, y + 34)
    ctx.fillStyle = muted
    ctx.font = `400 17px ${font}`
    ctx.fillText(
      billText(
        `${row.requestCount} 次请求 · ${props.formatNumber(row.totalTokens)} Token`,
        `${row.requestCount} requests · ${props.formatNumber(row.totalTokens)} tokens`,
      ),
      x + 26,
      y + 66,
    )
    ctx.textAlign = 'right'
    ctx.fillStyle = text
    ctx.font = `700 22px ${font}`
    ctx.fillText(rowCostText(row), x + w - 26, y + 42)
    ctx.fillStyle = muted
    ctx.font = `400 16px ${font}`
    ctx.fillText(row.price?.label || billText('本地价格表', 'Local price table'), x + w - 26, y + 68)
    ctx.textAlign = 'left'

    const barW = Math.max(36, (w - 52) * (row.totalTokens / maxTokens))
    drawRoundRect(ctx, x + 26, y + 82, w - 52, 6, 3, '#edf1f5')
    drawRoundRect(ctx, x + 26, y + 82, barW, 6, 3, index === 0 ? '#111827' : '#64748b')
  })

  const footerY = height - 210
  drawLine(ctx, pageX + 44, footerY, pageX + pageW - 44, footerY, line)
  ctx.fillStyle = muted
  ctx.font = `400 18px ${font}`
  ctx.fillText(billText('费用仅包含已匹配本地价格表的模型，未知模型不会计入金额。', 'Only models matched by the local price table are included.'), pageX + 44, footerY + 54)
  ctx.fillText(billText('模拟账单用于本地回顾和分享，不代表服务商最终账单。', 'Mock bill for local review and sharing, not a provider invoice.'), pageX + 44, footerY + 92)
}

function drawLedgerReport(ctx, width, height) {
  const font = '"Segoe UI", "Microsoft YaHei", Arial'
  const pageX = 56
  const pageW = width - 112
  const text = '#111827'
  const muted = '#5f6b7a'
  const line = '#d5dbe5'
  const rows = billingRows.value.slice(0, 12)

  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, width, height)
  drawRoundRect(ctx, pageX, 56, pageW, 188, 8, '#111827')
  ctx.fillStyle = '#ffffff'
  ctx.font = `800 42px ${font}`
  ctx.fillText(billText('OmniProxy 数据长图', 'OmniProxy Data Longform'), pageX + 34, 128)
  ctx.font = `400 20px ${font}`
  ctx.fillText(`${billText('日期', 'Date')} ${selectedDate.value} · ${statementId.value}`, pageX + 34, 172)
  ctx.textAlign = 'right'
  ctx.font = `700 30px ${font}`
  ctx.fillText(billTotalCostText.value, pageX + pageW - 34, 130)
  ctx.font = `400 18px ${font}`
  ctx.fillText(
    billText(
      `请求 ${props.formatNumber(totals.value.requestCount)} · Token ${props.formatNumber(totals.value.totalTokens)}`,
      `Requests ${props.formatNumber(totals.value.requestCount)} · Tokens ${props.formatNumber(totals.value.totalTokens)}`,
    ),
    pageX + pageW - 34,
    174,
  )
  ctx.textAlign = 'left'

  const tableX = pageX
  const tableY = 306
  const tableW = pageW
  const rowH = 82
  const columns = [
    [billText('模型', 'Model'), tableX + 26],
    [billText('请求', 'Requests'), tableX + 438],
    [billText('输入 Token', 'Input Tokens'), tableX + 548],
    [billText('输出 Token', 'Output Tokens'), tableX + 704],
    [billText('单价 / 1M', 'Rate / 1M'), tableX + 858],
  ]

  drawRoundRect(ctx, tableX, tableY, tableW, 62, 8, '#f1f5f9', '#dbe2eb')
  ctx.fillStyle = '#334155'
  ctx.font = `700 17px ${font}`
  columns.forEach(([label, x]) => ctx.fillText(label, x, tableY + 39))
  ctx.textAlign = 'right'
  ctx.fillText(billText('估算', 'Estimate'), tableX + tableW - 26, tableY + 39)
  ctx.textAlign = 'left'

  if (!rows.length) {
    drawRoundRect(ctx, tableX, tableY + 82, tableW, 126, 8, '#f8fafc', '#dbe2eb')
    ctx.fillStyle = muted
    ctx.font = `400 22px ${font}`
    ctx.fillText(billText('所选日期暂无可计价模型用量。', 'No billable model usage for this date.'), tableX + 34, tableY + 154)
  }

  rows.forEach((row, index) => {
    const y = tableY + 62 + index * rowH
    drawRoundRect(ctx, tableX, y, tableW, rowH, 0, index % 2 === 0 ? '#ffffff' : '#f8fafc')
    drawLine(ctx, tableX, y, tableX + tableW, y, '#edf0f4')
    ctx.fillStyle = text
    ctx.font = `700 19px ${font}`
    ctx.fillText(truncateText(ctx, row.model, 370), tableX + 26, y + 34)
    ctx.fillStyle = muted
    ctx.font = `400 15px ${font}`
    ctx.fillText(row.providers?.join(' / ') || 'local', tableX + 26, y + 60)
    ctx.fillStyle = text
    ctx.font = `600 18px ${font}`
    ctx.fillText(props.formatNumber(row.requestCount), tableX + 438, y + 47)
    ctx.fillText(props.formatNumber(row.inputTokens), tableX + 548, y + 47)
    ctx.fillText(props.formatNumber(row.outputTokens), tableX + 704, y + 47)
    ctx.fillStyle = muted
    ctx.font = `400 15px ${font}`
    ctx.fillText(priceRateText(row), tableX + 858, y + 47)
    ctx.textAlign = 'right'
    ctx.fillStyle = text
    ctx.font = `800 19px ${font}`
    ctx.fillText(rowCostText(row), tableX + tableW - 26, y + 47)
    ctx.textAlign = 'left'
  })

  const totalY = tableY + 62 + Math.max(rows.length, 1) * rowH + 50
  drawRoundRect(ctx, tableX, totalY, tableW, 184, 8, '#f8fafc', line)
  ctx.fillStyle = muted
  ctx.font = `500 18px ${font}`
  ctx.fillText(billText('合计', 'Total'), tableX + 32, totalY + 45)
  ctx.fillText(billText('未纳入', 'Ignored'), tableX + 32, totalY + 104)
  ctx.fillStyle = text
  ctx.font = `800 31px ${font}`
  drawAmountLines(ctx, totalInvoiceLines(), tableX + tableW - 32, totalY + 48, `800 31px ${font}`, text, 38)
  ctx.textAlign = 'right'
  ctx.fillStyle = muted
  ctx.font = `500 18px ${font}`
  ctx.fillText(
    billText(
      `${ignoredRows.value.length} 个模型 · ${props.formatNumber(ignoredTokenTotal.value)} Token`,
      `${ignoredRows.value.length} models · ${props.formatNumber(ignoredTokenTotal.value)} tokens`,
    ),
    tableX + tableW - 32,
    totalY + 104,
  )
  ctx.textAlign = 'left'

  const footerY = height - 170
  drawLine(ctx, pageX, footerY, pageX + pageW, footerY, line)
  ctx.fillStyle = muted
  ctx.font = `400 18px ${font}`
  ctx.fillText(billText('这是一张模拟账单长图，价格和汇率只来自本地价格表。', 'This mock bill image uses local price rules and local usage records only.'), pageX, footerY + 52)
  ctx.fillText(`${billText('生成于', 'Generated')} ${generatedAtText.value}`, pageX, footerY + 90)
}

function drawInvoiceLogo(ctx, x, y) {
  ctx.save()
  ctx.translate(x, y)
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.strokeStyle = '#111111'
  ctx.lineWidth = 4.2
  const cx = 28
  const cy = 28
  const radius = 23
  ;[
    [-0.14 * Math.PI, 0.62 * Math.PI],
    [0.54 * Math.PI, 1.30 * Math.PI],
    [1.22 * Math.PI, 1.98 * Math.PI],
  ].forEach(([start, end]) => {
    ctx.beginPath()
    ctx.arc(cx, cy, radius, start, end)
    ctx.stroke()
  })

  ctx.lineWidth = 3
  ;[0, (Math.PI * 2) / 3, (Math.PI * 4) / 3].forEach((angle) => {
    const innerX = cx + Math.cos(angle) * 7
    const innerY = cy + Math.sin(angle) * 7
    const outerX = cx + Math.cos(angle) * 20
    const outerY = cy + Math.sin(angle) * 20
    ctx.beginPath()
    ctx.moveTo(innerX, innerY)
    ctx.lineTo(outerX, outerY)
    ctx.stroke()

    ctx.beginPath()
    ctx.fillStyle = '#ffffff'
    ctx.arc(outerX, outerY, 5.4, 0, Math.PI * 2)
    ctx.fill()
    ctx.stroke()
  })

  ctx.beginPath()
  ctx.fillStyle = '#111111'
  ctx.arc(cx, cy, 5.8, 0, Math.PI * 2)
  ctx.fill()
  ctx.restore()
}

function drawInvoicePair(ctx, x, y, label, value, valueOffset = 190) {
  ctx.fillStyle = '#2f343b'
  ctx.font = '400 18px "Segoe UI", "Microsoft YaHei", Arial'
  ctx.fillText(label, x, y)
  ctx.fillStyle = '#111111'
  ctx.font = '400 20px "Segoe UI", "Microsoft YaHei", Arial'
  ctx.fillText(String(value || '-'), x + valueOffset, y)
}

function drawAmountLines(ctx, lines, xRight, y, font, color, lineHeight) {
  ctx.save()
  ctx.textAlign = 'right'
  ctx.fillStyle = color
  ctx.font = font
  lines.forEach((line, index) => {
    ctx.fillText(line, xRight, y + index * lineHeight)
  })
  ctx.restore()
}

function drawCheckIcon(ctx, x, y) {
  ctx.save()
  ctx.beginPath()
  ctx.fillStyle = '#2eb872'
  ctx.arc(x, y, 28, 0, Math.PI * 2)
  ctx.fill()
  ctx.strokeStyle = '#ffffff'
  ctx.lineWidth = 5
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.beginPath()
  ctx.moveTo(x - 13, y)
  ctx.lineTo(x - 4, y + 10)
  ctx.lineTo(x + 15, y - 12)
  ctx.stroke()
  ctx.restore()
}

function drawMeta(ctx, x, y, label, value) {
  ctx.fillStyle = '#94a3b8'
  ctx.font = '18px "Microsoft YaHei", Arial'
  ctx.fillText(label, x, y)
  ctx.fillStyle = '#0f172a'
  ctx.font = '600 22px "Microsoft YaHei", Arial'
  ctx.fillText(value, x, y + 32)
}

function drawLine(ctx, x1, y1, x2, y2, color) {
  ctx.beginPath()
  ctx.strokeStyle = color
  ctx.lineWidth = 1
  ctx.moveTo(x1, y1)
  ctx.lineTo(x2, y2)
  ctx.stroke()
}

function drawDashedLine(ctx, x1, y1, x2, y2, color) {
  ctx.save()
  ctx.setLineDash([12, 10])
  drawLine(ctx, x1, y1, x2, y2, color)
  ctx.restore()
}

function drawPill(ctx, x, y, width, height, text, fill, color, font) {
  drawRoundRect(ctx, x, y, width, height, height / 2, fill)
  ctx.save()
  ctx.fillStyle = color
  ctx.font = font
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(text, x + width / 2, y + height / 2)
  ctx.restore()
}

function drawRoundRect(ctx, x, y, width, height, radius, fill, stroke = '') {
  ctx.beginPath()
  ctx.moveTo(x + radius, y)
  ctx.lineTo(x + width - radius, y)
  ctx.quadraticCurveTo(x + width, y, x + width, y + radius)
  ctx.lineTo(x + width, y + height - radius)
  ctx.quadraticCurveTo(x + width, y + height, x + width - radius, y + height)
  ctx.lineTo(x + radius, y + height)
  ctx.quadraticCurveTo(x, y + height, x, y + height - radius)
  ctx.lineTo(x, y + radius)
  ctx.quadraticCurveTo(x, y, x + radius, y)
  ctx.closePath()
  if (fill) {
    ctx.fillStyle = fill
    ctx.fill()
  }
  if (stroke) {
    ctx.strokeStyle = stroke
    ctx.lineWidth = 1
    ctx.stroke()
  }
}

function truncateText(ctx, text, maxWidth) {
  const value = String(text || '')
  if (ctx.measureText(value).width <= maxWidth) return value
  let next = value
  while (next.length > 4 && ctx.measureText(`${next}...`).width > maxWidth) {
    next = next.slice(0, -1)
  }
  return `${next}...`
}
</script>

<template>
  <section class="billing-page">
    <article class="panel billing-hero-panel">
      <div class="billing-heading-stack">
        <div class="billing-heading-row">
          <div class="billing-title-copy">
            <h2>模拟账单</h2>
            <p>把当天模型用量做成可保存的模拟账单图，只用于本地回顾和分享</p>
          </div>
          <div class="billing-language-switch" role="radiogroup" aria-label="账单内容语言">
            <span>账单内容</span>
            <button
              v-for="language in billingLanguages"
              :key="language.key"
              type="button"
              :class="{ active: selectedLanguage === language.key }"
              :aria-checked="selectedLanguage === language.key"
              role="radio"
              @click="selectedLanguage = language.key"
            >
              {{ language.label }}
            </button>
          </div>
        </div>
        <div class="billing-control-row">
          <select v-model="selectedDate" aria-label="账单日期">
            <option v-for="date in availableDates" :key="date" :value="date">{{ date }}</option>
          </select>
          <div class="billing-action-buttons">
            <el-button :icon="Refresh" @click="$emit('refresh')">刷新</el-button>
            <el-button type="primary" :icon="View" :loading="previewBusy" @click="exportReportImage">
              预览模拟账单
            </el-button>
          </div>
        </div>
      </div>

      <div class="billing-summary-grid">
        <div class="billing-total-card">
          <span>估算费用</span>
          <strong>{{ totalCostText }}</strong>
          <small>按本地每 1M Token 单价计算</small>
        </div>
        <div>
          <span>总 Token</span>
          <strong>{{ formatNumber(totals.totalTokens) }}</strong>
          <small>输入 {{ formatNumber(totals.inputTokens) }} · 输出 {{ formatNumber(totals.outputTokens) }}</small>
        </div>
        <div>
          <span>请求数</span>
          <strong>{{ formatNumber(totals.requestCount) }}</strong>
          <small>{{ selectedDate }}</small>
        </div>
        <div>
          <span>未展示</span>
          <strong>{{ ignoredRows.length }}</strong>
          <small>{{ formatNumber(ignoredTokenTotal) }} Token 未计入金额</small>
        </div>
      </div>
    </article>

    <div class="billing-layout">
      <article class="panel billing-report-preview-panel">
        <div class="billing-template-head">
          <div>
            <strong>模拟账单模板</strong>
            <span>换一个风格，先预览再保存 PNG；账单图内容可切换中英文</span>
          </div>
        </div>
        <div class="billing-template-switch" role="radiogroup" aria-label="账单模板">
          <button
            v-for="template in reportTemplates"
            :key="template.key"
            type="button"
            :class="{ active: selectedTemplate === template.key }"
            :aria-checked="selectedTemplate === template.key"
            role="radio"
            @click="selectedTemplate = template.key"
          >
            <strong>{{ template.label }}</strong>
            <span>{{ template.description }}</span>
          </button>
        </div>

        <div :class="['billing-report-preview', `template-${selectedTemplate}`]">
          <div class="report-preview-top">
            <div>
              <strong>{{ activeBillTemplate.previewTitle }}</strong>
              <span>{{ activeBillTemplate.previewSubtitle }}</span>
            </div>
            <small>{{ activeBillTemplate.badge }}</small>
          </div>
          <div class="report-preview-meta">
            <div>
              <span>{{ billText('模拟编号', 'Mock ID') }}</span>
              <strong>{{ statementId }}</strong>
            </div>
            <div>
              <span>{{ billText('日期', 'Date') }}</span>
              <strong>{{ selectedDate }}</strong>
            </div>
            <div>
              <span>{{ billText('模板', 'Template') }}</span>
              <strong>{{ activeBillTemplate.label }}</strong>
            </div>
          </div>
          <div class="report-preview-total">
            <strong>{{ billTotalCostText }}</strong>
            <span>{{ billText('模拟费用', 'Mock Total') }}</span>
          </div>
          <div class="report-preview-metrics">
            <div>
              <span>{{ billText('总 Token', 'Total Tokens') }}</span>
              <strong>{{ formatNumber(totals.totalTokens) }}</strong>
            </div>
            <div>
              <span>{{ billText('请求数', 'Requests') }}</span>
              <strong>{{ formatNumber(totals.requestCount) }}</strong>
            </div>
          </div>
          <div class="report-preview-list">
            <div class="report-preview-list-head">
              <span>{{ billText('模型', 'Model') }}</span>
              <strong>{{ billText('用量', 'Usage') }}</strong>
              <small>{{ billText('金额', 'Amount') }}</small>
            </div>
            <div v-for="(row, index) in topRows.slice(0, 3)" :key="row.model">
              <span>{{ index + 1 }}</span>
              <strong>{{ row.model }}</strong>
              <small>{{ rowCostText(row) }}</small>
            </div>
            <div v-if="!topRows.length" class="empty compact-empty">
              {{ billText('暂无可计价模型', 'No billable model usage') }}
            </div>
          </div>
          <p>{{ activeBillTemplate.note }}</p>
        </div>
      </article>

      <article class="panel billing-table-panel">
        <div class="section-heading compact-heading">
          <div>
            <h2>模型费用明细</h2>
            <p>下方明细是生成模拟账单图的数据来源，未匹配价格的模型不会计入金额</p>
          </div>
        </div>

        <div class="billing-side-stack">
          <section class="billing-side-section billing-side-total">
            <span>账单洞察</span>
            <strong>{{ totalCostText }}</strong>
            <small>{{ selectedDate }} · {{ formatNumber(totals.totalTokens) }} Token · {{ formatNumber(totals.requestCount) }} 次请求</small>
          </section>

          <section class="billing-side-section">
            <div class="billing-side-section-head">
              <strong>模型占比</strong>
              <span>按估算费用排序</span>
            </div>
            <div v-if="topRows.length" class="billing-rank-bars">
              <div v-for="row in topRows.slice(0, 4)" :key="row.model" class="billing-rank-bar">
                <div>
                  <strong>{{ row.model }}</strong>
                  <span>{{ rowCostText(row) }} · {{ formatNumber(row.totalTokens) }} Token</span>
                </div>
                <i :style="{ width: rowCostShare(row) }"></i>
              </div>
            </div>
            <div v-else class="empty compact-empty">暂无可计价模型</div>
          </section>

          <section class="billing-side-section">
            <div class="billing-side-section-head">
              <strong>未纳入模型</strong>
              <span>{{ ignoredRows.length }} 个 · {{ formatNumber(ignoredTokenTotal) }} Token</span>
            </div>
            <div v-if="ignoredPreviewRows.length" class="billing-ignored-list">
              <div v-for="row in ignoredPreviewRows" :key="row.model">
                <strong>{{ row.model }}</strong>
                <span>{{ formatNumber(row.totalTokens) }} Token</span>
              </div>
            </div>
            <div v-else class="billing-ignored-list is-empty">
              <div>
                <strong>全部已计价</strong>
                <span>当前日期没有被价格表跳过的模型</span>
              </div>
            </div>
          </section>
        </div>

        <div class="billing-table">
          <div class="billing-row header">
            <span>模型</span>
            <span>输入 / 输出</span>
            <span>单价</span>
            <span>估算</span>
          </div>
          <div v-for="row in billingRows" :key="row.model" class="billing-row">
            <span>
              <strong>{{ row.model }}</strong>
              <small>{{ row.requestCount }} 次请求</small>
            </span>
            <span>
              {{ formatNumber(row.inputTokens) }} / {{ formatNumber(row.outputTokens) }}
              <small>{{ formatNumber(row.totalTokens) }} Token</small>
            </span>
            <span>
              <strong class="price-rate">{{ priceRateText(row) }}</strong>
              <small>每 1M · {{ row.price.label }}</small>
            </span>
            <span>
              <strong>{{ rowCostText(row) }}</strong>
            </span>
          </div>
          <div v-if="!billingRows.length" class="empty">所选日期暂无可计价模型用量</div>
        </div>
      </article>
    </div>

    <Transition name="modal-pop" appear>
      <div v-if="previewVisible" class="report-preview-modal-backdrop" @click.self="closeReportPreview">
        <section class="report-preview-modal" role="dialog" aria-modal="true" aria-label="账单预览">
          <header class="report-preview-modal-head">
            <div>
              <strong>模拟账单预览</strong>
              <span>{{ activeTemplate.label }} · {{ selectedDate }} · 内容{{ selectedLanguage === 'en' ? '英文' : '中文' }}</span>
            </div>
            <button type="button" aria-label="关闭预览" @click="closeReportPreview">×</button>
          </header>
          <div class="report-preview-image-frame">
            <img v-if="reportPreviewUrl" :src="reportPreviewUrl" alt="模拟账单预览图" />
          </div>
          <footer class="report-preview-modal-actions">
            <el-button @click="closeReportPreview">返回修改</el-button>
            <el-button type="primary" :icon="Download" @click="savePreviewImage">
              保存 PNG
            </el-button>
          </footer>
        </section>
      </div>
    </Transition>
  </section>
</template>
