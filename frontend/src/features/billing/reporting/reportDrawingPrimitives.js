export function drawLine(ctx, x1, y1, x2, y2, color) {
  ctx.beginPath()
  ctx.strokeStyle = color
  ctx.lineWidth = 1
  ctx.moveTo(x1, y1)
  ctx.lineTo(x2, y2)
  ctx.stroke()
}

export function drawDashedLine(ctx, x1, y1, x2, y2, color) {
  ctx.save()
  ctx.setLineDash([12, 10])
  drawLine(ctx, x1, y1, x2, y2, color)
  ctx.restore()
}

export function drawPill(ctx, x, y, width, height, text, fill, color, font) {
  drawRoundRect(ctx, x, y, width, height, height / 2, fill)
  ctx.save()
  ctx.fillStyle = color
  ctx.font = font
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(text, x + width / 2, y + height / 2)
  ctx.restore()
}

export function drawRoundRect(ctx, x, y, width, height, radius, fill, stroke = '') {
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

export function truncateText(ctx, text, maxWidth) {
  const value = String(text || '')
  if (ctx.measureText(value).width <= maxWidth) return value
  let next = value
  while (next.length > 4 && ctx.measureText(`${next}...`).width > maxWidth) {
    next = next.slice(0, -1)
  }
  return `${next}...`
}
