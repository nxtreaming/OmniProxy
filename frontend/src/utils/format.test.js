import { strict as assert } from 'node:assert'
import test from 'node:test'

import { formatDuration, formatNumber, localDateKey } from './format.js'

test('formatDuration keeps compact millisecond and second labels', () => {
  assert.equal(formatDuration(42), '42ms')
  assert.equal(formatDuration(1250), '1.25s')
  assert.equal(formatDuration(12500), '12.5s')
})

test('formatDuration rolls long values into minutes and seconds', () => {
  assert.equal(formatDuration(61000), '1m 1s')
  assert.equal(formatDuration(125000), '2m 5s')
})

test('formatNumber and localDateKey produce stable display values', () => {
  assert.equal(formatNumber(1234567), '1,234,567')
  assert.equal(localDateKey(new Date(2026, 3, 9)), '2026-04-09')
})
