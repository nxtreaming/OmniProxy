<script setup>
import { computed, ref, watch } from 'vue'
import { Download, Refresh } from '@element-plus/icons-vue'
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

const priceRules = [
  { pattern: /^gpt-5\.5/i, label: 'OpenAI GPT-5.5', currency: 'USD', input: 5, output: 30 },
  { pattern: /^gpt-5\.4-mini/i, label: 'OpenAI GPT-5.4 mini', currency: 'USD', input: 0.75, output: 4.5 },
  { pattern: /^gpt-5\.4/i, label: 'OpenAI GPT-5.4', currency: 'USD', input: 2.5, output: 15 },
  { pattern: /^claude-(opus-4\.[5-7]|opus-4-5|opus-4-6|opus-4-7)/i, label: 'Claude Opus 4.5+', currency: 'USD', input: 5, output: 25 },
  { pattern: /^claude-(opus|3-opus|opus-4|opus-4-1)/i, label: 'Claude Opus', currency: 'USD', input: 15, output: 75 },
  { pattern: /^claude-(sonnet|3-7-sonnet|4-sonnet|sonnet-4)/i, label: 'Claude Sonnet', currency: 'USD', input: 3, output: 15 },
  { pattern: /^claude-(haiku-4\.5|4-5-haiku)/i, label: 'Claude Haiku 4.5', currency: 'USD', input: 1, output: 5 },
  { pattern: /^claude-(haiku|3-5-haiku)/i, label: 'Claude Haiku', currency: 'USD', input: 0.8, output: 4 },
  { pattern: /^deepseek-v4-pro/i, label: 'DeepSeek V4 Pro', currency: 'USD', input: 0.435, output: 0.87 },
  { pattern: /^deepseek-(chat|reasoner|v4-flash)/i, label: 'DeepSeek V4 Flash', currency: 'USD', input: 0.14, output: 0.28 },
  { pattern: /^gemini-2\.5-pro/i, label: 'Gemini 2.5 Pro', currency: 'USD', input: 1.25, output: 10 },
  { pattern: /^gemini-2\.5-flash-lite/i, label: 'Gemini 2.5 Flash-Lite', currency: 'USD', input: 0.1, output: 0.4 },
  { pattern: /^gemini-2\.5-flash/i, label: 'Gemini 2.5 Flash', currency: 'USD', input: 0.3, output: 2.5 },
  { pattern: /^kimi[-_]?k2\.6|moonshot[-_]?k2\.6/i, label: 'Kimi K2.6', currency: 'USD', input: 0.95, output: 4 },
  { pattern: /^kimi[-_]?k2\.5|moonshot[-_]?k2\.5/i, label: 'Kimi K2.5', currency: 'USD', input: 0.6, output: 3 },
  { pattern: /^kimi[-_]?k2|moonshot[-_]?k2/i, label: 'Kimi K2', currency: 'USD', input: 0.6, output: 2.5 },
  { pattern: /^moonshot-v1-128k/i, label: 'Moonshot v1 128K', currency: 'CNY', input: 10, output: 30 },
  { pattern: /^moonshot-v1-32k/i, label: 'Moonshot v1 32K', currency: 'CNY', input: 5, output: 20 },
  { pattern: /^moonshot-v1-8k/i, label: 'Moonshot v1 8K', currency: 'CNY', input: 2, output: 10 },
  { pattern: /^minimax[-_]?m2-highspeed/i, label: 'MiniMax M2 Highspeed', currency: 'USD', input: 0.6, output: 2.4 },
  { pattern: /^minimax[-_]?m2\.(7|5|1)|^minimax[-_]?m2\b/i, label: 'MiniMax M2', currency: 'USD', input: 0.3, output: 1.2 },
  { pattern: /^mimo[-_]?v2\.5($|-)/i, label: 'Xiaomi MiMo V2.5', currency: 'USD', input: 0.4, output: 2 },
  { pattern: /^mimo[-_]?v2[-_]?pro/i, label: 'Xiaomi MiMo V2 Pro', currency: 'USD', input: 1, output: 3 },
  { pattern: /^glm-(4\.7|4\.5|4)-flash/i, label: 'Zhipu GLM Flash', currency: 'CNY', input: 0, output: 0 },
]

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
const ignoredTokenTotal = computed(() => ignoredRows.value.reduce((sum, row) => sum + row.totalTokens, 0))
const statementId = computed(() => `OP-${selectedDate.value.replaceAll('-', '')}`)
const invoiceNumber = computed(() => `INV-${selectedDate.value}-${invoiceSuffix(selectedDate.value)}`)
const invoiceDateText = computed(() => formatDateLong(selectedDate.value))
const generatedAtText = computed(() =>
  new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date()),
)

function buildBillingRows(entries, date, dailyUsage) {
  const byModel = new Map()
  if (Array.isArray(dailyUsage) && dailyUsage.length) {
    for (const item of dailyUsage) {
      if (String(item.date || '') !== date) continue
      addUsageRow(byModel, {
        model: item.model,
        provider: item.provider,
        clientName: item.clientName,
        clientKey: item.clientKey,
        requestCount: Number(item.requestCount || 0),
        inputTokens: Number(item.inputTokens || 0),
        outputTokens: Number(item.outputTokens || 0),
        totalTokens: Number(item.totalTokens || 0),
      })
    }
    return finalizeBillingRows(byModel)
  }

  for (const entry of entries || []) {
    if (entryDate(entry) !== date) continue
    const total = Number(entry.totalTokens || 0)
    const output = Number(entry.outputTokens || 0)
    const input = Number(entry.inputTokens || 0) || Math.max(0, total - output)
    addUsageRow(byModel, {
      model: entry.model,
      provider: entry.provider,
      clientName: entry.clientName,
      clientKey: entry.clientKey,
      requestCount: 1,
      inputTokens: input,
      outputTokens: output,
      totalTokens: total || input + output,
    })
  }

  return finalizeBillingRows(byModel)
}

function addUsageRow(byModel, item) {
  const model = String(item.model || '').trim()
  if (!model) return
  const totalTokens = Number(item.totalTokens || 0)
  if (totalTokens <= 0) return
  const current = byModel.get(model) || {
    model,
    requestCount: 0,
    inputTokens: 0,
    outputTokens: 0,
    totalTokens: 0,
    providers: new Set(),
    clients: new Set(),
  }
  current.requestCount += Number(item.requestCount || 0)
  current.inputTokens += Number(item.inputTokens || 0)
  current.outputTokens += Number(item.outputTokens || 0)
  current.totalTokens += totalTokens
  if (item.provider) current.providers.add(item.provider)
  if (item.clientName || item.clientKey) current.clients.add(item.clientName || item.clientKey)
  byModel.set(model, current)
}

function finalizeBillingRows(byModel) {
  return [...byModel.values()]
    .map((row) => {
      const price = resolvePrice(row.model)
      const cost = price
        ? (row.inputTokens / 1_000_000) * price.input + (row.outputTokens / 1_000_000) * price.output
        : 0
      return {
        ...row,
        providers: [...row.providers],
        clients: [...row.clients],
        price,
        cost,
        currency: price?.currency || '',
      }
    })
    .sort((a, b) => b.cost - a.cost || b.totalTokens - a.totalTokens)
}

function resolvePrice(model) {
  const normalized = String(model || '').trim()
  return priceRules.find((rule) => rule.pattern.test(normalized)) || null
}

function entryDate(entry) {
  return String(entry?.time || '').slice(0, 10)
}

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
  return new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  }).format(parsed)
}

function totalInvoiceLines() {
  if (!totals.value.byCurrency.length) return ['No billable usage']
  return totals.value.byCurrency.map((item) => formatInvoiceMoney(item.value, item.currency))
}

function priceRateText(row) {
  return `${formatMoney(row.price.input, row.currency)} / ${formatMoney(row.price.output, row.currency)}`
}

function rowCostText(row) {
  return formatMoney(row.cost, row.currency)
}

async function exportReportImage() {
  const canvas = document.createElement('canvas')
  const width = 1200
  const height = 2000
  const scale = Math.max(1, Math.min(2, window.devicePixelRatio || 1))
  canvas.width = width * scale
  canvas.height = height * scale
  canvas.style.width = `${width}px`
  canvas.style.height = `${height}px`
  const ctx = canvas.getContext('2d')
  ctx.scale(scale, scale)
  drawBillingReport(ctx, width, height)
  const blob = await new Promise((resolve) => canvas.toBlob(resolve, 'image/png'))
  if (!blob) return
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `omniproxy-billing-${selectedDate.value}.png`
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 500)
}

function drawBillingReport(ctx, width, height) {
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
  ctx.fillText('Invoice', pageX + pageW, 104)
  ctx.textAlign = 'left'

  ctx.font = `700 21px ${font}`
  ctx.fillText('OmniProxy Local', pageX, 208)
  ctx.font = `400 21px ${font}`
  ctx.fillStyle = text
  ctx.fillText('AI token dispatch gateway', pageX, 252)
  ctx.fillText('Local request history', pageX, 294)
  ctx.fillText('Estimate only', pageX, 336)

  const metaX = pageX + 680
  drawInvoicePair(ctx, metaX, 218, 'Invoice Number:', invoiceNumber.value)
  drawInvoicePair(ctx, metaX, 280, 'Invoice Date:', invoiceDateText.value)
  drawInvoicePair(ctx, metaX, 342, 'Billing Method:', 'Local price table')
  drawInvoicePair(ctx, metaX, 404, 'Status:', 'Calculated')

  drawLine(ctx, pageX, 470, pageX + pageW, 470, line)

  ctx.fillStyle = text
  ctx.font = `700 22px ${font}`
  ctx.fillText('Bill To', pageX, 538)
  ctx.font = `400 21px ${font}`
  const clientNames = [...new Set(billingRows.value.flatMap((row) => row.clients || []))].filter(Boolean)
  ctx.fillText(clientNames[0] || 'OmniProxy User', pageX, 596)
  ctx.fillText(`Statement ${statementId.value}`, pageX, 638)
  ctx.fillText('Local workspace', pageX, 680)

  const infoX = pageX + 530
  ctx.font = `700 22px ${font}`
  ctx.fillText('Usage Information', infoX, 538)
  drawInvoicePair(ctx, infoX, 596, 'Usage Date:', invoiceDateText.value, 250)
  drawInvoicePair(ctx, infoX, 650, 'Requests:', props.formatNumber(totals.value.requestCount), 250)
  drawInvoicePair(ctx, infoX, 704, 'Total Tokens:', props.formatNumber(totals.value.totalTokens), 250)

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
  ctx.fillText('Invoice Details', detailX + 36, detailY + 62)

  const headerY = detailY + 96
  drawRoundRect(ctx, detailX + 6, headerY, detailW - 12, 60, 7, '#f8fafc', line)
  ctx.fillStyle = '#1f2937'
  ctx.font = `500 18px ${font}`
  ctx.fillText('Description', detailX + 36, headerY + 38)
  ctx.fillText('Quantity', detailX + 575, headerY + 38)
  ctx.fillText('Unit Price', detailX + 770, headerY + 38)
  ctx.textAlign = 'right'
  ctx.fillText('Amount', detailX + detailW - 34, headerY + 38)
  ctx.textAlign = 'left'

  if (!rows.length) {
    ctx.fillStyle = muted
    ctx.font = `400 22px ${font}`
    ctx.fillText('No metered model usage for this date.', detailX + 36, headerY + 118)
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
      `${row.requestCount} requests · input ${props.formatNumber(row.inputTokens)} · output ${props.formatNumber(row.outputTokens)}`,
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
      ctx.fillText(`In ${formatMoney(row.price.input, row.currency)}`, detailX + 770, y + 32)
      ctx.fillText(`Out ${formatMoney(row.price.output, row.currency)}`, detailX + 770, y + 58)
    } else {
      ctx.fillText('Pending', detailX + 770, y + 42)
    }

    ctx.textAlign = 'right'
    ctx.fillStyle = text
    ctx.font = row.price ? `600 21px ${font}` : `400 19px ${font}`
    ctx.fillText(row.price ? formatInvoiceMoney(row.cost, row.currency) : 'Pending', detailX + detailW - 34, y + 43)
    ctx.textAlign = 'left'
  })

  const subtotalY = headerY + 60 + visibleRowCount * rowH + 42
  drawLine(ctx, detailX + 24, subtotalY - 24, detailX + detailW - 24, subtotalY - 24, line)
  const amountX = detailX + detailW - 34
  ctx.fillStyle = text
  ctx.font = `400 20px ${font}`
  ctx.fillText('Subtotal', detailX + 575, subtotalY + 8)
  drawAmountLines(ctx, totalInvoiceLines(), amountX, subtotalY + 8, `400 20px ${font}`, text, 28)
  ctx.fillText('Tax (0%)', detailX + 575, subtotalY + 76)
  ctx.textAlign = 'right'
  ctx.fillText('0.0000', amountX, subtotalY + 76)
  ctx.textAlign = 'left'

  drawLine(ctx, detailX + 570, subtotalY + 106, detailX + detailW - 24, subtotalY + 106, line)
  ctx.font = `700 24px ${font}`
  ctx.fillText('Total', detailX + 575, subtotalY + 164)
  drawAmountLines(ctx, totalInvoiceLines(), amountX, subtotalY + 164, `700 24px ${font}`, text, 32)

  const statusY = detailY + detailH + 32
  drawRoundRect(ctx, pageX, statusY, pageW, 142, 8, '#ffffff', line)
  drawCheckIcon(ctx, pageX + 62, statusY + 72)
  ctx.fillStyle = text
  ctx.font = `700 23px ${font}`
  ctx.fillText('Usage Calculated', pageX + 126, statusY + 58)
  ctx.font = `400 19px ${font}`
  ctx.fillText('This estimate was generated from local OmniProxy request history and model price rules.', pageX + 126, statusY + 98)

  const footerY = statusY + 186
  drawLine(ctx, pageX, footerY, pageX + pageW, footerY, line)
  ctx.fillStyle = text
  ctx.font = `400 18px ${font}`
  ctx.fillText('Only models matched by the local price table are included in this estimate.', pageX, footerY + 48)
  ctx.fillStyle = muted
  ctx.font = `400 17px ${font}`
  ctx.textAlign = 'center'
  ctx.fillText('Please keep this invoice image for your local cost records.', width / 2, footerY + 112)
  ctx.fillText(`Generated ${generatedAtText.value} · OmniProxy estimate only`, width / 2, footerY + 150)
  ctx.textAlign = 'left'
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
      <div class="section-heading">
        <div>
          <h2>费用账单</h2>
          <p>只统计已匹配价格且有 Token 的模型请求，健康检查和额度刷新不纳入账单</p>
        </div>
        <div class="billing-actions">
          <select v-model="selectedDate">
            <option v-for="date in availableDates" :key="date" :value="date">{{ date }}</option>
          </select>
          <el-button :icon="Refresh" @click="$emit('refresh')">刷新</el-button>
          <el-button type="primary" :icon="Download" @click="exportReportImage">生成所选日期报告图</el-button>
        </div>
      </div>

      <div class="billing-summary-grid">
        <div class="billing-total-card">
          <span>估算费用</span>
          <strong>{{ totalCostText }}</strong>
          <small>按每 1M Token 单价计算</small>
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
          <span>未纳入</span>
          <strong>{{ ignoredRows.length }}</strong>
          <small>{{ formatNumber(ignoredTokenTotal) }} Token 未计费</small>
        </div>
      </div>
    </article>

    <div class="billing-layout">
      <article class="panel billing-report-preview-panel">
        <div class="billing-report-preview">
          <div class="report-preview-top">
            <div>
              <strong>Usage Statement</strong>
              <span>OmniProxy estimated charges</span>
            </div>
            <small>ESTIMATE</small>
          </div>
          <div class="report-preview-meta">
            <div>
              <span>Statement ID</span>
              <strong>{{ statementId }}</strong>
            </div>
            <div>
              <span>Bill period</span>
              <strong>{{ selectedDate }}</strong>
            </div>
          </div>
          <div class="report-preview-total">
            <strong>{{ totalCostText }}</strong>
            <span>Estimated total</span>
          </div>
          <div class="report-preview-metrics">
            <div>
              <span>总 Token</span>
              <strong>{{ formatNumber(totals.totalTokens) }}</strong>
            </div>
            <div>
              <span>请求数</span>
              <strong>{{ formatNumber(totals.requestCount) }}</strong>
            </div>
          </div>
          <div class="report-preview-list">
            <div class="report-preview-list-head">
              <span>Model</span>
              <strong>Usage</strong>
              <small>Amount</small>
            </div>
            <div v-for="(row, index) in topRows.slice(0, 3)" :key="row.model">
              <span>{{ index + 1 }}</span>
              <strong>{{ row.model }}</strong>
              <small>{{ rowCostText(row) }}</small>
            </div>
            <div v-if="!topRows.length" class="empty compact-empty">暂无可计价模型</div>
          </div>
          <p>费用为本地价格表估算，未知模型不会计入金额。</p>
        </div>
      </article>

      <article class="panel billing-table-panel">
        <div class="section-heading compact-heading">
          <div>
            <h2>模型费用明细</h2>
            <p>默认价格表覆盖 OpenAI、Claude、DeepSeek、Gemini、Kimi K2、MiniMax 与 MiMo；未匹配价格的模型不会计入金额</p>
          </div>
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
  </section>
</template>
