import { drawLine, drawRoundRect, truncateText } from './reportDrawingPrimitives'

export function createInvoiceReportDrawers(state) {
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

  return {
    drawSummaryReport,
    drawLedgerReport,
    drawInvoiceLogo,
    drawInvoicePair,
    drawAmountLines,
    drawCheckIcon,
  }
}
