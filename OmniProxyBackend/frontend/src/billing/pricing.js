export const priceRules = [
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

export function resolvePrice(model) {
  const normalized = String(model || '').trim()
  return priceRules.find((rule) => rule.pattern.test(normalized)) || null
}
