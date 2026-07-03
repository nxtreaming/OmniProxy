import { cpSync, existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendRoot = dirname(dirname(fileURLToPath(import.meta.url)))
const distDir = join(frontendRoot, 'dist')
const backendRoot = resolve(frontendRoot, '..', 'OmniProxyBackend')
const backendDistDir = resolve(backendRoot, 'frontend-dist')

if (!existsSync(distDir)) {
  throw new Error(`Frontend dist directory does not exist: ${distDir}`)
}

if (!backendDistDir.startsWith(`${backendRoot}${sep}`)) {
  throw new Error(`Refusing to sync frontend dist outside backend root: ${backendDistDir}`)
}

writeFileSync(join(distDir, '.gitkeep'), '\n')
removeWithRetry(backendDistDir)
mkdirSync(backendDistDir, { recursive: true })
cpSync(distDir, backendDistDir, { recursive: true })

function removeWithRetry(target) {
  let lastError
  for (const delayMs of [0, 100, 250, 500, 1000]) {
    if (delayMs > 0) {
      sleep(delayMs)
    }
    try {
      rmSync(target, { recursive: true, force: true })
      return
    } catch (error) {
      if (!['EBUSY', 'EPERM', 'ENOTEMPTY'].includes(error?.code)) {
        throw error
      }
      lastError = error
    }
  }
  throw lastError
}

function sleep(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms)
}
