import { reactive, ref } from 'vue'
import { idleScreen, outgoingScreen, reduceScreen, type ScreenEvent, type ScreenView } from './screenState'
import { isHttpsPage } from './httpsPage'

export const screen = reactive<ScreenView>(idleScreen())
export const remoteScreen = ref<MediaStream | null>(null)

type Signal = ScreenEvent & {
  sdp?: string
  candidate?: RTCIceCandidateInit
  turn?: { urls?: string[]; username?: string; credential?: string }
}

let send: (payload: object) => void = () => {}
let pc: RTCPeerConnection | null = null
let iceServers: RTCIceServer[] = []
let localStream: MediaStream | null = null

export function bindScreenSender(fn: (payload: object) => void) {
  send = fn
}

export function canShareScreen(): boolean {
  return typeof navigator.mediaDevices?.getDisplayMedia === 'function'
}

export function onScreenSignal(event: Signal) {
  const next = reduceScreen(screen, event)
  Object.assign(screen, next)
  if (event.turn?.urls?.length) {
    iceServers = [{ urls: event.turn.urls, username: event.turn.username, credential: event.turn.credential }]
  }
  if (event.type === 'screen.offer' && event.sdp && screen.role === 'viewer') {
    void answerOffer(event.sdp)
  }
  if (event.type === 'screen.answer' && event.sdp) {
    void pc?.setRemoteDescription({ type: 'answer', sdp: event.sdp })
  }
  if (event.type === 'screen.candidate' && event.candidate && pc) {
    void pc.addIceCandidate(event.candidate)
  }
  if (next.phase === 'idle' || next.phase === 'failed') {
    cleanupMedia()
  }
}

export function startScreen(conversationId: string, peerName: string, enabled: boolean): boolean {
  if (!enabled || !isHttpsPage(location.protocol) || screen.phase !== 'idle' || !canShareScreen()) {
    return false
  }
  Object.assign(screen, outgoingScreen(conversationId, peerName))
  send({ type: 'screen.invite', conversationId })
  return true
}

export function chooseScreen() {
  if (screen.phase !== 'choosing') {
    return
  }
  void captureAndOffer()
}

export function acceptScreen() {
  if (screen.phase !== 'incoming') {
    return
  }
  Object.assign(screen, { ...screen, phase: 'connecting' })
  ensurePeer()
  send({ type: 'screen.accept', screenId: screen.screenId })
}

export function rejectScreen() {
  if (screen.phase !== 'incoming') {
    return
  }
  send({ type: 'screen.reject', screenId: screen.screenId })
  Object.assign(screen, idleScreen())
  cleanupMedia()
}

export function hangupScreen() {
  if (screen.phase === 'idle' || screen.phase === 'failed') {
    return
  }
  send({ type: 'screen.hangup', screenId: screen.screenId })
  Object.assign(screen, idleScreen())
  cleanupMedia()
}

export function dismissScreen() {
  Object.assign(screen, idleScreen())
  cleanupMedia()
}

function ensurePeer() {
  if (pc) {
    return
  }
  pc = new RTCPeerConnection({ iceServers })
  for (const track of localStream?.getTracks() ?? []) {
    if (localStream) {
      pc.addTrack(track, localStream)
    }
  }
  pc.onicecandidate = (event) => {
    if (!event.candidate) {
      return
    }
    send({ type: 'screen.candidate', screenId: screen.screenId, candidate: event.candidate.toJSON() })
  }
  pc.ontrack = (event) => {
    remoteScreen.value = event.streams[0] ?? new MediaStream([event.track])
    if (screen.phase === 'connecting' || screen.phase === 'active') {
      screen.phase = 'active'
    }
  }
  pc.onconnectionstatechange = () => {
    if (pc?.connectionState === 'connected' && (screen.phase === 'connecting' || screen.phase === 'active')) {
      screen.phase = 'active'
    }
    if (pc?.connectionState === 'failed') {
      failScreen('连接失败')
    }
  }
}

async function captureAndOffer() {
  let stream: MediaStream
  try {
    stream = await captureDisplay()
  } catch {
    failScreen('无法共享')
    return
  }
  if (screen.phase !== 'choosing') {
    stopStream(stream)
    return
  }
  localStream = stream
  const track = stream.getVideoTracks()[0]
  if (track) {
    track.onended = () => {
      if (screen.phase !== 'idle' && screen.phase !== 'failed') {
        hangupScreen()
      }
    }
  }
  screen.phase = 'connecting'
  try {
    ensurePeer()
    if (!pc) {
      return
    }
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    send({ type: 'screen.offer', screenId: screen.screenId, sdp: offer.sdp })
  } catch {
    failScreen('连接失败')
  }
}

async function answerOffer(sdp: string) {
  try {
    ensurePeer()
    if (!pc) {
      return
    }
    await pc.setRemoteDescription({ type: 'offer', sdp })
    const answer = await pc.createAnswer()
    await pc.setLocalDescription(answer)
    send({ type: 'screen.answer', screenId: screen.screenId, sdp: answer.sdp })
    if (screen.phase === 'connecting') {
      screen.phase = 'active'
    }
  } catch {
    failScreen('连接失败')
  }
}

async function captureDisplay(): Promise<MediaStream> {
  const media = navigator.mediaDevices
  try {
    return await media.getDisplayMedia({ video: { cursor: 'always' } as MediaTrackConstraints, audio: false })
  } catch (err) {
    const name = err instanceof DOMException ? err.name : ''
    if (name === 'OverconstrainedError' || name === 'NotSupportedError' || name === 'TypeError') {
      return await media.getDisplayMedia({ video: true, audio: false })
    }
    throw err
  }
}

function failScreen(reason: string) {
  if (screen.phase !== 'idle' && screen.phase !== 'failed') {
    send({ type: 'screen.hangup', screenId: screen.screenId })
  }
  Object.assign(screen, { ...screen, phase: 'failed', reason })
  cleanupMedia()
}

function cleanupMedia() {
  stopStream(localStream)
  localStream = null
  remoteScreen.value = null
  pc?.close()
  pc = null
  iceServers = []
}

function stopStream(stream: MediaStream | null) {
  for (const track of stream?.getTracks() ?? []) {
    track.stop()
  }
}
