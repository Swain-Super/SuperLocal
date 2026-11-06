import assert from 'node:assert/strict'
import test from 'node:test'
import { isHttpsPage } from './httpsPage.ts'

test('只有 https 页面可以通话和共享屏幕', () => {
  assert.equal(isHttpsPage('https:'), true)
  assert.equal(isHttpsPage('http:'), false)
  assert.equal(isHttpsPage(''), false)
})
