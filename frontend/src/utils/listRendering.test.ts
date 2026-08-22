import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

test('card grids skip off-screen layout work', () => {
  const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '../assets/css/global.css'), 'utf8')
  assert.match(css, /content-visibility:\s*auto/)
  assert.match(css, /contain-intrinsic-size:/)
  assert.match(css, /\.ebook-card/)
  assert.match(css, /\.odob-card/)
  assert.match(css, /\.course-card/)
})
