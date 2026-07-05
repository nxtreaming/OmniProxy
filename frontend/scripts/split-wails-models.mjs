import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendRoot = dirname(dirname(fileURLToPath(import.meta.url)))
const modelsPath = join(frontendRoot, 'wailsjs', 'go', 'models.ts')
const modelsDir = join(frontendRoot, 'wailsjs', 'go', 'models')

if (!existsSync(modelsPath)) {
  process.exit(0)
}

const source = readFileSync(modelsPath, 'utf8').replace(/\r\n/g, '\n')
if (source.includes("from './models/")) {
  cleanSplitModelFiles()
  process.exit(0)
}

const namespaces = namespaceBlocks(source)
if (!namespaces.length) {
  process.exit(0)
}

mkdirSync(modelsDir, { recursive: true })
for (const file of readdirSync(modelsDir)) {
  if (file.endsWith('.ts')) {
    rmSync(join(modelsDir, file), { force: true })
  }
}

const namespaceClasses = new Map()
for (const block of namespaces) {
  if (block.name === 'main') {
    namespaceClasses.set(block.name, splitMainNamespace(block.text))
  } else {
    const classes = classNames(block.text)
    namespaceClasses.set(block.name, classes)
    writeNamespaceFile(block.name, block.text)
  }
}

writeRootBarrel(namespaceClasses)

function namespaceBlocks(text) {
  const matches = [...text.matchAll(/^export namespace ([A-Za-z0-9_]+) \{/gm)]
  return matches.map((match, index) => {
    const start = match.index
    const end = index + 1 < matches.length ? matches[index + 1].index : text.length
    return {
      name: match[1],
      text: text.slice(start, end).trimEnd(),
    }
  })
}

function splitMainNamespace(text) {
  const blocks = classBlocks(text)
  const classOrder = blocks.map((block) => block.name)
  const blockByName = new Map(blocks.map((block) => [block.name, block.text]))
  const groups = [
    {
      file: 'main_core',
      classes: [
        'activeRequestResponse',
        'apiKeyBatchImportRequest',
        'apiKeyBatchImportSkipped',
        'apiKeyBatchImportResult',
        'appInfo',
        'claudeModelsConfigureRequest',
        'clientConfigureResult',
        'codexAuthExportResult',
        'codexConfigureResult',
        'mimoConfigureResult',
      ],
    },
    {
      file: 'main_history',
      classes: ['retryAttemptResponse', 'historyResponse', 'logResponse'],
    },
    {
      file: 'main_openrouter',
      classes: [
        'openRouterChatMessage',
        'openRouterChatRequest',
        'openRouterChatUsageResponse',
        'openRouterChatResponse',
        'openRouterPricing',
        'openRouterModelResponse',
        'openRouterModelsResponse',
      ],
    },
    {
      file: 'main_tokens',
      classes: [
        'balancePackageResponse',
        'healthResponse',
        'tokenExportResult',
        'tokenStatsResponse',
        'usageResponse',
        'tokenResponse',
        'validationResponse',
      ],
    },
    {
      file: 'main_update',
      classes: ['updateDiagnostics', 'updateDownloadRequest', 'updateDownloadStatus', 'updateInfo'],
    },
  ]

  const assigned = new Set(groups.flatMap((group) => group.classes))
  const unknown = classOrder.filter((name) => !assigned.has(name))
  if (unknown.length) {
    groups.push({ file: 'main_misc', classes: unknown })
  }

  for (const group of groups) {
    const body = group.classes.map((name) => blockByName.get(name)).filter(Boolean).join('\n')
    if (!body) continue
    const imports = body.includes('token.') ? "import { token } from './token'\n\n" : ''
    writeGeneratedFile(join(modelsDir, `${group.file}.ts`), `${imports}export namespace main {\n${body}\n}\n`)
  }

  const imports = groups
    .filter((group) => group.classes.some((name) => blockByName.has(name)))
    .map((group) => `import { main as ${group.file}Models } from './${group.file}'`)
  const lines = [...imports, '', 'export namespace main {']
  for (const name of classOrder) {
    const group = groups.find((item) => item.classes.includes(name))
    if (group) {
      lines.push(`  export import ${name} = ${group.file}Models.${name}`)
    }
  }
  lines.push('}')
  writeGeneratedFile(join(modelsDir, 'main.ts'), `${lines.join('\n')}\n`)
  return classOrder
}

function classBlocks(namespaceText) {
  const lines = namespaceText.split('\n')
  const starts = []
  for (let index = 0; index < lines.length; index += 1) {
    const match = lines[index].match(/^\s*export class ([A-Za-z0-9_]+)/)
    if (match) {
      starts.push({ name: match[1], start: index })
    }
  }
  return starts.map((entry, index) => {
    const end = index + 1 < starts.length ? starts[index + 1].start : lines.length - 1
    return {
      name: entry.name,
      text: lines.slice(entry.start, end).join('\n').trimEnd(),
    }
  })
}

function classNames(namespaceText) {
  return classBlocks(namespaceText).map((block) => block.name)
}

function writeNamespaceFile(name, text) {
  writeGeneratedFile(join(modelsDir, `${name}.ts`), `${text}\n`)
}

function writeRootBarrel(classesByNamespace) {
  const imports = []
  const names = [...classesByNamespace.keys()]
  for (const name of names) {
    imports.push(`import { ${name} as ${name}Models } from './models/${name}'`)
  }

  const lines = [...imports, '']
  for (const name of names) {
    lines.push(`export namespace ${name} {`)
    for (const className of classesByNamespace.get(name)) {
      lines.push(`  export import ${className} = ${name}Models.${className}`)
    }
    lines.push('}')
    lines.push('')
  }
  if (lines[lines.length - 1] === '') {
    lines.pop()
  }
  writeGeneratedFile(modelsPath, `${lines.join('\n')}\n`)
}

function cleanSplitModelFiles() {
  if (!existsSync(modelsDir)) {
    return
  }
  const targets = [modelsPath]
  for (const file of readdirSync(modelsDir)) {
    if (file.endsWith('.ts')) {
      targets.push(join(modelsDir, file))
    }
  }
  for (const target of targets) {
    writeGeneratedFile(target, readFileSync(target, 'utf8'))
  }
}

function writeGeneratedFile(path, text) {
  writeFileSync(path, cleanGeneratedText(text), 'utf8')
}

function cleanGeneratedText(text) {
  return `${text.replace(/\r\n/g, '\n').replace(/[ \t]+$/gm, '').trimEnd()}\n`
}
