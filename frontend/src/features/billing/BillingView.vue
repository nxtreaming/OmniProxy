<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Download, Refresh, View } from '@element-plus/icons-vue'
import { buildBillingRows, entryDate } from './reporting/aggregate'
import { buildReportBlob as buildCanvasReportBlob, createReportCanvas as createCanvasReport } from './reporting/reportCanvas'
import { createBillingReportDrawers } from './reporting/reportDrawers'
import { localDateKey } from '../../utils/format'
import GeminiSelect from '../../components/GeminiSelect.vue'

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

const billingDateOptions = computed(() =>
  availableDates.value.map((date) => ({ value: date, label: date })),
)

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

function reportDrawingState() {
  return {
    props,
    selectedLanguage,
    selectedDate,
    activeTemplate,
    activeBillTemplate,
    billingRows,
    ignoredRows,
    topRows,
    totals,
    billText,
    formatBillText,
    statementId,
    invoiceNumber,
    invoiceDateText,
    generatedAtText,
    totalCostText,
    billTotalCostText,
    ignoredTokenTotal,
    totalInvoiceLines,
    priceRateText,
    rowCostText,
    rowCostShare,
    formatMoney,
    formatInvoiceMoney,
  }
}

function drawBillingReport(ctx, width, height, templateKey = selectedTemplate.value) {
  createBillingReportDrawers(reportDrawingState()).drawBillingReport(ctx, width, height, templateKey)
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
          <GeminiSelect
            v-model="selectedDate"
            class="billing-date-select"
            :options="billingDateOptions"
            aria-label="账单日期"
          />
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

<style src="./BillingView.css"></style>
