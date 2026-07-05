const openAICompatibleProviders = [
  'openai',
  'deepseek',
  'kimi',
  'xiaomi',
  'zhipu',
  'minimax',
  'openrouter',
  'tokenrouter',
  'sub2api',
  'newapi',
  'anyrouter',
  'zo',
  'prem',
  'custom',
]

export const routeDefinitions = [
  {
    key: 'codex',
    title: 'Codex',
    protocol: 'OpenAI Responses',
    endpoint: (port) => `http://127.0.0.1:${port}/codex/v1`,
    fallback: { provider: 'openai', credentialType: 'codex_auth_json', model: 'gpt-5.4' },
    providers: openAICompatibleProviders,
    modelPresets: ['gpt-5.4', 'gpt-5.4-high', 'gpt-5.5', 'gpt-5.5-high', 'gpt-5-codex'],
  },
  {
    key: 'claude',
    title: 'Claude Code / Desktop',
    protocol: 'Anthropic Messages',
    endpoint: (port) => `http://127.0.0.1:${port}/anthropic-router`,
    fallback: { provider: 'anthropic', credentialType: 'api_key', model: 'claude-sonnet-4-5-20250929' },
    providers: [
      'anthropic',
      'deepseek',
      'kimi',
      'xiaomi',
      'zhipu',
      'minimax',
      'sub2api',
      'newapi',
      'anyrouter',
      'zo',
      'prem',
      'custom',
    ],
    modelPresets: [
      'claude-sonnet-4-5-20250929',
      'claude-opus-4-7',
      'claude-sonnet-4-6',
      'deepseek-v4-pro',
      'mimo-v2.5-pro[1m]',
      'kimi-for-coding',
      'glm-5.1',
    ],
  },
  {
    key: 'openai',
    title: 'OpenAI 兼容',
    protocol: 'Chat / Responses',
    endpoint: (port) => `http://127.0.0.1:${port}/opencode-router/v1`,
    fallback: { provider: 'openai', credentialType: 'api_key', model: 'gpt-5.4' },
    providers: openAICompatibleProviders,
    modelPresets: ['gpt-5.4', 'gpt-5.4-high', 'gpt-5.5', 'gpt-5.5-high', 'deepseek-v4-pro', 'kimi-for-coding', 'glm-5.1', 'MiniMax-M2.7'],
  },
  {
    key: 'gemini',
    title: 'Gemini CLI',
    protocol: 'Gemini Native',
    endpoint: (port) => `http://127.0.0.1:${port}/gemini`,
    fallback: { provider: 'gemini', credentialType: 'api_key', model: 'gemini-3-pro-preview' },
    providers: ['gemini', 'sub2api', 'newapi'],
    modelPresets: ['gemini-3-pro-preview', 'gemini-3-flash-preview', 'gemini-2.5-pro', 'gemini-2.5-flash'],
  },
]

export const gatewayPlatformPresets = [
  {
    key: 'openai',
    routeCredentials: { codex: 'codex_auth_json', openai: 'api_key' },
    models: [
      routeModel('gpt-5.4', ['codex', 'openai']),
      routeModel('gpt-5.4-high', ['codex', 'openai']),
      routeModel('gpt-5.5', ['codex', 'openai']),
      routeModel('gpt-5-codex', ['codex', 'openai']),
    ],
  },
  {
    key: 'anthropic',
    routeCredentials: { claude: 'api_key' },
    models: [
      routeModel('claude-sonnet-4-5-20250929', ['claude'], 'Claude Sonnet 4.5'),
      routeModel('claude-opus-4-5-20251101', ['claude'], 'Claude Opus 4.5'),
    ],
  },
  {
    key: 'deepseek',
    models: [
      routeModel('deepseek-v4-pro', ['codex', 'openai', 'claude'], 'DeepSeek V4 Pro'),
      routeModel('deepseek-v4-flash', ['openai', 'claude'], 'DeepSeek V4 Flash'),
    ],
  },
  {
    key: 'kimi',
    models: [
      routeModel('kimi-for-coding', ['codex', 'openai', 'claude'], 'Kimi for Coding'),
    ],
  },
  {
    key: 'xiaomi',
    models: [
      {
        id: 'mimo-v2.5-pro',
        label: 'MiMo V2.5 Pro',
        routeModels: { codex: 'mimo-v2.5-pro', openai: 'mimo-v2.5-pro', claude: 'mimo-v2.5-pro[1m]' },
      },
      routeModel('mimo-v2.5', ['openai', 'claude'], 'MiMo V2.5'),
    ],
  },
  {
    key: 'zhipu',
    models: [
      routeModel('glm-5.1', ['codex', 'openai', 'claude'], 'GLM-5.1'),
    ],
  },
  {
    key: 'minimax',
    models: [
      routeModel('MiniMax-M2.7', ['codex', 'openai', 'claude'], 'MiniMax M2.7'),
    ],
  },
  {
    key: 'gemini',
    routeCredentials: { gemini: 'api_key' },
    models: [
      routeModel('gemini-3-pro-preview', ['gemini'], 'Gemini 3 Pro Preview'),
      routeModel('gemini-3-flash-preview', ['gemini'], 'Gemini 3 Flash Preview'),
      routeModel('gemini-2.5-pro', ['gemini'], 'Gemini 2.5 Pro'),
      routeModel('gemini-2.5-flash', ['gemini'], 'Gemini 2.5 Flash'),
    ],
  },
  {
    key: 'openrouter',
    routeCredentials: { codex: 'api_key', openai: 'api_key' },
    models: [
      routeModel('openrouter/auto', ['codex', 'openai'], 'OpenRouter Auto'),
      routeModel('anthropic/claude-sonnet-4.5', ['openai'], 'Claude Sonnet via OpenRouter'),
      routeModel('openai/gpt-5.4', ['codex', 'openai'], 'GPT-5.4 via OpenRouter'),
    ],
  },
  {
    key: 'tokenrouter',
    routeCredentials: { codex: 'api_key', openai: 'api_key' },
    models: [
      routeModel('auto:balance', ['codex', 'openai'], 'Auto Balance'),
      routeModel('auto:quality', ['codex', 'openai'], 'Auto Quality'),
      routeModel('auto:speed', ['codex', 'openai'], 'Auto Speed'),
      routeModel('auto:cost', ['codex', 'openai'], 'Auto Cost'),
    ],
  },
  {
    key: 'sub2api',
    routeCredentials: { codex: 'api_key', openai: 'api_key', claude: 'api_key', gemini: 'api_key' },
    models: [
      routeModel('gpt-5.4', ['codex', 'openai']),
      routeModel('claude-sonnet-4-5', ['claude'], 'Claude Sonnet 4.5'),
      routeModel('gemini-3-pro-preview', ['gemini'], 'Gemini 3 Pro Preview'),
    ],
  },
  {
    key: 'newapi',
    routeCredentials: { codex: 'api_key', openai: 'api_key', claude: 'api_key', gemini: 'api_key' },
    models: [
      routeModel('gpt-5.4', ['codex', 'openai']),
      routeModel('claude-sonnet-4-5', ['claude'], 'Claude Sonnet 4.5'),
      routeModel('gemini-3-pro-preview', ['gemini'], 'Gemini 3 Pro Preview'),
    ],
  },
  {
    key: 'anyrouter',
    routeCredentials: { codex: 'api_key', openai: 'api_key', claude: 'api_key' },
    models: [
      routeModel('gpt-5-codex', ['codex', 'openai'], 'GPT-5 Codex'),
      routeModel('claude-opus-4-5-20251101', ['claude'], 'Claude Opus 4.5'),
    ],
  },
  {
    key: 'zo',
    routeCredentials: { codex: 'api_key', openai: 'api_key', claude: 'api_key' },
    models: [
      routeModel('gpt-5.4', ['codex', 'openai'], 'Zo GPT-5.4'),
      routeModel('claude-opus-4-7', ['claude'], 'Zo Claude Opus 4.7'),
      routeModel('claude-sonnet-4-6', ['claude'], 'Zo Claude Sonnet 4.6'),
    ],
  },
  {
    key: 'prem',
    routeCredentials: { codex: 'api_key', openai: 'api_key', claude: 'api_key' },
    models: [
      routeModel('deepseek-v4-pro', ['codex', 'openai', 'claude'], 'DeepSeek V4 Pro'),
      routeModel('qwen3.5', ['codex', 'openai'], 'Qwen 3.5'),
    ],
  },
  {
    key: 'custom',
    routeCredentials: { codex: 'api_key', openai: 'api_key', claude: 'api_key' },
    models: [
      routeModel('custom-model', ['codex', 'openai', 'claude'], 'Custom Model'),
    ],
  },
]

function routeModel(model, routeKeys, label = model) {
  return {
    id: `${model}:${routeKeys.join(',')}`,
    label,
    routeModels: Object.fromEntries(routeKeys.map((key) => [key, model])),
  }
}
