import { readdirSync, readFileSync, statSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const maxLines = Number.parseInt(process.argv[2] ?? process.env.MAX_LINES ?? '700', 10)

const includedExtensions = new Set(['.go', '.vue', '.js', '.ts', '.css'])
const excludedDirectories = new Set([
  '.git',
  'build',
  'dist',
  'frontend-dist',
  'node_modules',
  'vendor',
])

if (!Number.isFinite(maxLines) || maxLines <= 0) {
  console.error('Usage: node scripts/check-lines.mjs [max-lines]')
  process.exit(2)
}

const files = []

function walk(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (!excludedDirectories.has(entry.name)) {
        walk(path.join(directory, entry.name))
      }
      continue
    }
    if (entry.isFile() && includedExtensions.has(path.extname(entry.name))) {
      files.push(path.join(directory, entry.name))
    }
  }
}

function lineCount(filePath) {
  const content = readFileSync(filePath, 'utf8')
  if (content.length === 0) return 0
  const lines = content.split(/\r\n|\r|\n/)
  return lines.at(-1) === '' ? lines.length - 1 : lines.length
}

walk(repoRoot)

const results = files
  .filter((filePath) => statSync(filePath).isFile())
  .map((filePath) => ({
    lines: lineCount(filePath),
    path: path.relative(repoRoot, filePath).split(path.sep).join('/'),
  }))
  .sort((left, right) => right.lines - left.lines || left.path.localeCompare(right.path))

const offenders = results.filter((result) => result.lines > maxLines)
if (offenders.length > 0) {
  console.error(`Files over ${maxLines} lines:`)
  for (const offender of offenders) {
    console.error(`${String(offender.lines).padStart(5)}  ${offender.path}`)
  }
  process.exit(1)
}

const largest = results[0]
if (largest) {
  console.log(`Line count check passed: ${results.length} files, max ${largest.lines}/${maxLines} in ${largest.path}`)
} else {
  console.log(`Line count check passed: no source files found`)
}
