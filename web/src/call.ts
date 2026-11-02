import { reactive, ref } from 'vue'
import { idleCall, reduceCall, type CallEvent, type CallView } from './callState'

export const call = reactive<CallView>(idleCall())
export const localStream = ref<MediaStream | null>(null)
export const remoteStream = ref<MediaStream | null>(null)
export const remoteCameraOn = ref(true)

type Signal = CallEvent & {
  sdp?: string
  candidate?: RTCIceCandidateInit
  turn?: { urls?: string[]; username?: string; credential?: string }
}

let send: (payload: object) => void = () => {}
let pc: RTCPeerConnection | null = null
let iceServers: RTCIceServer[] = []

export function bindCallSender(fn: (payload: object) => void) {
  send = fn
}

export function onSignal(event: Signal) {
  const next = reduceCall(call, event)
  Object.assign(call, next)
  if (event.turn?.urls?.length) {
    iceServers = [{ urls: event.turn.urls, username: event.turn.username, credential: event.turn.credential }]
  }
  if (event.type === 'call.accept' && call.phase === 'connecting') {
    void sendOffer()
  }
  if (event.type === 'call.offer' && event.sdp) {
    void answerOffer(event.sdp)
  }
  if (event.type === 'call.answer' && event.sdp) {
    void pc?.setRemoteDescription({ type: 'answer', sdp: event.sdp })
  }
  if (event.type === 'call.candidate' && event.candidate && pc) {
    void pc.addIceCandidate(event.candidate)
  }
  if (next.phase === 'idle') {
    cleanupMedia()
  }
}

export async function startCall(conversationId: string, kind: 'audio' | 'video', peerName: string, enabled: boolean) {
  if (!enabled || !window.isSecureContext || call.phase !== 'idle') {
    return
  }
  let stream: MediaStream
  try {
    stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: kind === 'video' })
  } catch {
    Object.assign(call, { ...idleCall(), phase: 'failed', conversationId, kind, peerName, reason: '无法使用麦克风或摄像头' })
    return
  }
  localStream.value = stream
  Object.assign(call, { ...idleCall(), phase: 'outgoing', conversationId, kind, peerName, cameraOn: kind === 'video' })
  send({ type: 'call.invite', conversationId, kind })
}

export async function acceptCall() {
  if (call.phase !== 'incoming') {
    return
  }
  let stream: MediaStream
  try {
    stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: call.kind === 'video' })
  } catch {
    send({ type: 'call.reject', callId: call.callId })
    Object.assign(call, { ...call, phase: 'failed', reason: '无法使用麦克风或摄像头' })
    return
  }
  localStream.value = stream
  Object.assign(call, { ...call, phase: 'connecting' })
  ensurePeer()
  send({ type: 'call.accept', callId: call.callId })
}

export function rejectCall() {
  if (call.phase !== 'incoming') {
    return
  }
  send({ type: 'call.reject', callId: call.callId })
  Object.assign(call, idleCall())
  cleanupMedia()
}

export function hangupCall() {
  if (call.phase === 'idle') {
    return
  }
  send({ type: 'call.hangup', callId: call.callId })
  Object.assign(call, idleCall())
  cleanupMedia()
}

export function dismissCall() {
  Object.assign(call, idleCall())
  cleanupMedia()
}

export function toggleMute() {
  call.muted = !call.muted
  for (const track of localStream.value?.getAudioTracks() ?? []) {
    track.enabled = !call.muted
  }
}

export function toggleCamera() {
  call.cameraOn = !call.cameraOn
  for (const track of localStream.value?.getVideoTracks() ?? []) {
    track.enabled = call.cameraOn
  }
}

function ensurePeer() {
  if (pc) {
    return
  }
  pc = new RTCPeerConnection({ iceServers })
  const stream = localStream.value
  for (const track of stream?.getTracks() ?? []) {
    if (stream) {
      pc.addTrack(track, stream)
    }
  }
  pc.onicecandidate = (event) => {
    if (!event.candidate) {
      return
    }
    send({ type: 'call.candidate', callId: call.callId, candidate: event.candidate.toJSON() })
  }
  pc.ontrack = (event) => {
    const stream = event.streams[0] ?? new MediaStream([event.track])
    remoteStream.value = stream
    if (event.track.kind !== 'video') {
      return
    }
    const sync = () => {
      remoteCameraOn.value = !event.track.muted && event.track.readyState === 'live'
    }
    event.track.onmute = sync
    event.track.onunmute = sync
    event.track.onended = sync
    sync()
  }
  pc.onconnectionstatechange = () => {
    if (pc?.connectionState === 'connected') {
      call.phase = 'active'
    }
    if (pc?.connectionState === 'failed') {
      failCall('连接失败')
    }
  }
}

async function sendOffer() {
  try {
    ensurePeer()
    if (!pc) {
      return
    }
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    send({ type: 'call.offer', callId: call.callId, sdp: offer.sdp })
  } catch {
    failCall('连接失败')
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
    send({ type: 'call.answer', callId: call.callId, sdp: answer.sdp })
    call.phase = 'active'
  } catch {
    failCall('连接失败')
  }
}

function failCall(reason: string) {
  send({ type: 'call.hangup', callId: call.callId })
  Object.assign(call, { ...call, phase: 'failed', reason })
  cleanupMedia()
}

function cleanupMedia() {
  for (const track of localStream.value?.getTracks() ?? []) {
    track.stop()
  }
  localStream.value = null
  remoteStream.value = null
  remoteCameraOn.value = true
  pc?.close()
  pc = null
  iceServers = []
}
