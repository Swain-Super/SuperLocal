export type ScreenPhase = 'idle' | 'outgoing' | 'incoming' | 'choosing' | 'connecting' | 'active' | 'failed'

export type ScreenRole = '' | 'sharer' | 'viewer'

export type ScreenView = {
  phase: ScreenPhase
  screenId: string
  conversationId: string
  peerName: string
  reason: string
  role: ScreenRole
}

export type ScreenEvent = {
  type?: string
  screenId?: string
  conversationId?: string
  fromNickname?: string
  reason?: string
}

export function idleScreen(): ScreenView {
  return {
    phase: 'idle',
    screenId: '',
    conversationId: '',
    peerName: '',
    reason: '',
    role: '',
  }
}

export function outgoingScreen(conversationId: string, peerName: string): ScreenView {
  return {
    ...idleScreen(),
    phase: 'outgoing',
    conversationId,
    peerName,
    role: 'sharer',
  }
}

export function reduceScreen(state: ScreenView, event: ScreenEvent): ScreenView {
  const type = event.type ?? ''
  const same = state.screenId === '' || state.screenId === (event.screenId ?? '')
  if (type === 'screen.invite' && state.phase === 'idle') {
    return {
      ...idleScreen(),
      phase: 'incoming',
      screenId: event.screenId ?? '',
      conversationId: event.conversationId ?? '',
      peerName: event.fromNickname ?? '',
      role: 'viewer',
    }
  }
  if (type === 'screen.ringing' && state.phase === 'outgoing') {
    return { ...state, screenId: event.screenId ?? state.screenId }
  }
  if ((type === 'screen.end' || type === 'screen.reject') && state.phase === 'incoming' && state.screenId === (event.screenId ?? '')) {
    return idleScreen()
  }
  if (type === 'screen.accept' && state.phase === 'outgoing') {
    return { ...state, phase: 'choosing', screenId: event.screenId || state.screenId }
  }
  if (type === 'screen.answer' && state.phase === 'connecting') {
    return { ...state, phase: 'active' }
  }
  if (type === 'screen.reject' && (state.phase === 'outgoing' || state.phase === 'choosing' || state.phase === 'connecting')) {
    return { ...state, phase: 'failed', reason: event.reason ?? '' }
  }
  if (type === 'screen.hangup' && state.phase !== 'idle' && state.phase !== 'incoming' && same) {
    return { ...state, phase: 'failed', reason: state.role === 'viewer' ? 'sharer-end' : 'viewer-end' }
  }
  if (type === 'screen.end' && state.phase === 'connecting' && state.role === 'viewer') {
    return state
  }
  return state
}

export function screenStatusText(state: ScreenView): string {
  if (state.phase === 'outgoing') {
    return '等待对方接受'
  }
  if (state.phase === 'incoming') {
    return '邀请你查看屏幕'
  }
  if (state.phase === 'choosing') {
    return '对方已接受'
  }
  if (state.phase === 'connecting') {
    return state.role === 'viewer' ? '对方正在选择屏幕' : '正在连接'
  }
  if (state.phase === 'active') {
    return '正在共享'
  }
  if (state.reason === 'busy') {
    return '对方忙线'
  }
  if (state.reason === 'timeout') {
    return '对方未接受'
  }
  if (state.reason === 'offline') {
    return '对方离线'
  }
  if (state.reason === 'reject') {
    return '对方已拒绝'
  }
  if (state.reason === 'forbidden') {
    return '无法共享'
  }
  if (state.reason === 'sharer-end') {
    return '对方已结束共享'
  }
  if (state.reason === 'viewer-end') {
    return '对方已结束观看'
  }
  return state.reason || '无法共享'
}
