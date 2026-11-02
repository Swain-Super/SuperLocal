export type CallPhase = 'idle' | 'outgoing' | 'incoming' | 'connecting' | 'active' | 'failed'

export type CallView = {
  phase: CallPhase
  callId: string
  conversationId: string
  kind: 'audio' | 'video'
  peerName: string
  reason: string
  muted: boolean
  cameraOn: boolean
}

export type CallEvent = {
  type?: string
  callId?: string
  conversationId?: string
  kind?: string
  fromNickname?: string
  reason?: string
}

export function idleCall(): CallView {
  return {
    phase: 'idle',
    callId: '',
    conversationId: '',
    kind: 'audio',
    peerName: '',
    reason: '',
    muted: false,
    cameraOn: true,
  }
}

export function reduceCall(state: CallView, event: CallEvent): CallView {
  const type = event.type ?? ''
  if (type === 'call.invite' && state.phase === 'idle') {
    return {
      ...idleCall(),
      phase: 'incoming',
      callId: event.callId ?? '',
      conversationId: event.conversationId ?? '',
      kind: event.kind === 'video' ? 'video' : 'audio',
      peerName: event.fromNickname ?? '',
    }
  }
  if (type === 'call.ringing' && state.phase === 'outgoing') {
    return { ...state, callId: event.callId ?? state.callId }
  }
  if ((type === 'call.end' || type === 'call.reject') && state.phase === 'incoming' && state.callId === (event.callId ?? '')) {
    return idleCall()
  }
  if (type === 'call.accept' && state.phase === 'outgoing') {
    return { ...state, phase: 'connecting', callId: event.callId || state.callId }
  }
  if (type === 'call.answer' && state.phase === 'connecting') {
    return { ...state, phase: 'active' }
  }
  if (type === 'call.reject' && (state.phase === 'outgoing' || state.phase === 'connecting')) {
    return { ...state, phase: 'failed', reason: event.reason ?? '' }
  }
  if (type === 'call.hangup' && state.phase !== 'idle' && state.phase !== 'incoming') {
    return idleCall()
  }
  if (type === 'call.end' && state.phase === 'connecting') {
    return state
  }
  return state
}
