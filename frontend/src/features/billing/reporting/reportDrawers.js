import { createInvoiceReportDrawers } from './reportInvoiceDrawers'
import { drawDashedLine, drawLine, drawPill, drawRoundRect, truncateText } from './reportDrawingPrimitives'

export function createBillingReportDrawers(state) {
  const {
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
  } = state
  const {
    drawSummaryReport,
    drawLedgerReport,
    drawInvoiceLogo,
    drawInvoicePair,
    drawAmountLines,
    drawCheckIcon,
  } = createInvoiceReportDrawers(state)

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

  return {
    drawBillingReport,
  }
}
