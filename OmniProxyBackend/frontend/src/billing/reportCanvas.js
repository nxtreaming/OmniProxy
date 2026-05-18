export function createReportCanvas(drawReport, options = {}) {
  const {
    templateKey,
    width = 1200,
    height = 2000,
    pixelRatio = globalThis.devicePixelRatio || 1,
  } = options
  const canvas = document.createElement('canvas')
  const scale = Math.max(1, Math.min(2, pixelRatio || 1))
  canvas.width = width * scale
  canvas.height = height * scale
  canvas.style.width = `${width}px`
  canvas.style.height = `${height}px`
  const ctx = canvas.getContext('2d')
  if (!ctx) return canvas
  ctx.scale(scale, scale)
  drawReport(ctx, width, height, templateKey)
  return canvas
}

export async function buildReportBlob(drawReport, options = {}) {
  const canvas = createReportCanvas(drawReport, options)
  return new Promise((resolve) => canvas.toBlob(resolve, 'image/png'))
}
