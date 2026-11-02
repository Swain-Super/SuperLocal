import assert from 'node:assert/strict'
import test from 'node:test'
import { idleCall, reduceCall } from './callState.ts'

test('来电进入 incoming', () => {
  const next = reduceCall(idleCall(), { type: 'call.invite', callId: 'c1', kind: 'video', fromNickname: '甲' })
  assert.equal(next.phase, 'incoming')
  assert.equal(next.callId, 'c1')
  assert.equal(next.peerName, '甲')
  assert.equal(next.kind, 'video')
})

test('来电被结束或拒绝后回到空闲', () => {
  const incoming = reduceCall(idleCall(), { type: 'call.invite', callId: 'c1' })
  assert.equal(reduceCall(incoming, { type: 'call.end', callId: 'c1' }).phase, 'idle')
  assert.equal(reduceCall(incoming, { type: 'call.reject', callId: 'c1' }).phase, 'idle')
})

test('对方接听后进入 connecting，answer 后进入 active', () => {
  const outgoing = { ...idleCall(), phase: 'outgoing' as const, callId: 'c1' }
  const connecting = reduceCall(outgoing, { type: 'call.accept', callId: 'c1' })
  assert.equal(connecting.phase, 'connecting')
  assert.equal(reduceCall(connecting, { type: 'call.answer', callId: 'c1' }).phase, 'active')
})

test('忙线进入 failed，接通中的 call.end 不打断', () => {
  const outgoing = { ...idleCall(), phase: 'outgoing' as const, callId: 'c1' }
  const failed = reduceCall(outgoing, { type: 'call.reject', callId: 'c1', reason: 'busy' })
  assert.equal(failed.phase, 'failed')
  assert.equal(failed.reason, 'busy')
  const connecting = { ...outgoing, phase: 'connecting' as const }
  assert.equal(reduceCall(connecting, { type: 'call.end', callId: 'c1' }).phase, 'connecting')
})

test('通话中挂断回到空闲', () => {
  const active = { ...idleCall(), phase: 'active' as const, callId: 'c1' }
  assert.equal(reduceCall(active, { type: 'call.hangup', callId: 'c1' }).phase, 'idle')
})
