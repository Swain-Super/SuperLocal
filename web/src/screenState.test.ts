import assert from 'node:assert/strict'
import test from 'node:test'
import { idleScreen, outgoingScreen, reduceScreen, screenStatusText } from './screenState.ts'

test('邀请进入 incoming', () => {
  const next = reduceScreen(idleScreen(), { type: 'screen.invite', screenId: 's1', fromNickname: '甲', conversationId: 'c1' })
  assert.equal(next.phase, 'incoming')
  assert.equal(next.role, 'viewer')
  assert.equal(next.peerName, '甲')
  assert.equal(next.screenId, 's1')
})

test('发起不依赖抓屏，接受后进入选择屏幕', () => {
  const outgoing = outgoingScreen('c1', '乙')
  assert.equal(outgoing.phase, 'outgoing')
  assert.equal(outgoing.role, 'sharer')
  const choosing = reduceScreen(outgoing, { type: 'screen.accept', screenId: 's1' })
  assert.equal(choosing.phase, 'choosing')
  assert.equal(screenStatusText(choosing), '对方已接受')
})

test('观看方连接中忽略 screen.end，挂断显示对方结束', () => {
  const incoming = reduceScreen(idleScreen(), { type: 'screen.invite', screenId: 's1' })
  const connecting = { ...incoming, phase: 'connecting' as const }
  assert.equal(reduceScreen(connecting, { type: 'screen.end', screenId: 's1' }).phase, 'connecting')
  const ended = reduceScreen(connecting, { type: 'screen.hangup', screenId: 's1' })
  assert.equal(ended.phase, 'failed')
  assert.equal(screenStatusText(ended), '对方已结束共享')
})

test('其他邀请页收到 screen.end 后关闭', () => {
  const incoming = reduceScreen(idleScreen(), { type: 'screen.invite', screenId: 's1' })
  assert.equal(reduceScreen(incoming, { type: 'screen.end', screenId: 's1' }).phase, 'idle')
})

test('超时和拒绝进入失败文案', () => {
  const outgoing = outgoingScreen('c1', '乙')
  const timed = reduceScreen({ ...outgoing, screenId: 's1' }, { type: 'screen.reject', screenId: 's1', reason: 'timeout' })
  assert.equal(timed.phase, 'failed')
  assert.equal(screenStatusText(timed), '对方未接受')
  const rejected = reduceScreen({ ...outgoing, screenId: 's1' }, { type: 'screen.reject', screenId: 's1', reason: 'reject' })
  assert.equal(screenStatusText(rejected), '对方已拒绝')
})

test('answer 后进入正在共享', () => {
  const connecting = { ...outgoingScreen('c1', '乙'), phase: 'connecting' as const, screenId: 's1' }
  const active = reduceScreen(connecting, { type: 'screen.answer', screenId: 's1' })
  assert.equal(active.phase, 'active')
  assert.equal(screenStatusText(active), '正在共享')
})
