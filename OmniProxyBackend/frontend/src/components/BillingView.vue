<script setup>
import { computed, ref } from 'vue'
import { Download, Refresh } from '@element-plus/icons-vue'
import { localDateKey } from '../utils/format'

const props = defineProps({
  entries: {
    type: Array,
    default: () => [],
  },
  formatNumber: {
    type: Function,
    required: true,
  },
})

defineEmits(['refresh'])

const selectedDate = ref(localDateKey())

const priceRules = [
  { pattern: /^deepseek-(chat|v3|v4-flash)/i, label: 'DeepSeek Chat / V3', currency: 'USD', input: 0.27, output: 1.1 },
  { pattern: /^deepseek-(reasoner|r1|v4-pro)/i, label: 'DeepSeek Reasoner / R1', currency: 'USD', input: 0.55, output: 2.19 },
  { pattern: /^gemini-.*flash/i, label: 'Gemini Flash', currency: 'USD', input: 0.3, output: 2.5 },
  { pattern: /^gemini-.*pro/i, label: 'Gemini Pro', currency: 'USD', input: 1.25, output: 10 },
  { pattern: /^kimi|moonshot/i, label: 'Kimi', currency: 'CNY', input: 2, output: 8 },
  { pattern: /^(glm|zhipu)/i, label: 'Zhipu GLM', currency: 'CNY', input: 5, output: 15 },
  { pattern: /^minimax/i, label: 'MiniMax', currency: 'CNY', input: 4, output: 16 },
  { pattern: /^mimo/i, label: 'Xiaomi MiMo', currency: 'CNY', input: 2, output: 8 },
]

const availableDates = computed(() => {
  const dates = new Set([localDateKey()])
  for (const entry of props.entries || []) {
    const date = entryDate(entry)
    if (date) dates.add(date)
  }
  return [...dates].sort((a, b) => b.localeCompare(a)).slice(0, 30)
})

const billingRows = computed(() => buildBillingRows(props.entries, selectedDate.value))
const pricedRows = computed(() => billingRows.value.filter((row) => row.price))
const unknownRows = computed(() => billingRows.value.filter((row) => !row.price))
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
    if (row.price) {
      byCurrency.set(row.currency, (byCurrency.get(row.currency) || 0) + row.cost)
    }
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
  if (!totals.value.byCurrency.length) return '未配置价格'
  return totals.value.byCurrency.map((item) => formatMoney(item.value, item.currency)).join(' + ')
})
const unknownTokenPercent = computed(() => {
  const total = totals.value.totalTokens
  if (!total) return 0
  const unknown = unknownRows.value.reduce((sum, row) => sum + row.totalTokens, 0)
  return Math.round((unknown / total) * 100)
})
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

function buildBillingRows(entries, date) {
  const byModel = new Map()
  for (const entry of entries || []) {
    if (entryDate(entry) !== date) continue
    const model = String(entry.model || entry.protocol || '未记录模型').trim() || '未记录模型'
    const current = byModel.get(model) || {
      model,
      requestCount: 0,
      inputTokens: 0,
      outputTokens: 0,
      totalTokens: 0,
      providers: new Set(),
      clients: new Set(),
    }
    const total = Number(entry.totalTokens || 0)
    const output = Number(entry.outputTokens || 0)
    const input = Number(entry.inputTokens || 0) || Math.max(0, total - output)
    current.requestCount += 1
    current.inputTokens += input
    current.outputTokens += output
    current.totalTokens += total || input + output
    if (entry.provider) current.providers.add(entry.provider)
    if (entry.clientName || entry.clientKey) current.clients.add(entry.clientName || entry.clientKey)
    byModel.set(model, current)
  }

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
  if (!totals.value.byCurrency.length) return ['Pending price rules']
  return totals.value.byCurrency.map((item) => formatInvoiceMoney(item.value, item.currency))
}

function priceText(row) {
  if (!row.price) return '未配置价格'
  return `${formatMoney(row.price.input, row.currency)} / ${formatMoney(row.price.output, row.currency)} 每 1M`
}

function rowCostText(row) {
  return row.price ? formatMoney(row.cost, row.currency) : '未配置'
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
  ctx.fillText('If a model is missing from the local price table, its amount is marked as pending.', pageX, footerY + 48)
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
          <p>按请求历史里的模型和输入/输出 Token 估算，不代表厂商真实账单</p>
        </div>
        <div class="billing-actions">
          <select v-model="selectedDate">
            <option v-for="date in availableDates" :key="date" :value="date">{{ date }}</option>
          </select>
          <el-button :icon="Refresh" @click="$emit('refresh')">刷新</el-button>
          <el-button type="primary" :icon="Download" @click="exportReportImage">生成今日报告图</el-button>
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
          <span>未计价</span>
          <strong>{{ unknownTokenPercent }}%</strong>
          <small>{{ unknownRows.length }} 个模型未配置价格</small>
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
            <div v-for="(row, index) in topRows.slice(0, 4)" :key="row.model">
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
            <p>默认价格表先覆盖常见 DeepSeek、Gemini、Kimi、GLM、MiniMax 与 MiMo 模型</p>
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
              {{ priceText(row) }}
              <small v-if="row.price">{{ row.price.label }}</small>
            </span>
            <span>
              <strong>{{ rowCostText(row) }}</strong>
            </span>
          </div>
          <div v-if="!billingRows.length" class="empty">今天暂无请求历史用量</div>
        </div>
      </article>
    </div>
  </section>
</template>
