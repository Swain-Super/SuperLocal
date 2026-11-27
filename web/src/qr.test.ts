import assert from 'node:assert/strict'
import test from 'node:test'
import { qrModules } from './qr.ts'

test('version 4 byte QR matches the known matrix', () => {
  const modules = qrModules('http://192.168.1.8:18080/')
  assert.equal(modules.length, 33)
  assert.equal(modules[0].length, 33)
  assert.equal(modules[0][0], true)
  assert.equal(modules[3][3], true)
  assert.equal(modules[6][0], true)
  let sum = 0
  modules.forEach((row, y) => {
    row.forEach((on, x) => {
      if (on) sum = (Math.imul(sum, 131) + y * 33 + x) >>> 0
    })
  })
  assert.equal(sum >>> 0, 679154931)
})
