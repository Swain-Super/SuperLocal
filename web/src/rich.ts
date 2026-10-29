export type RichFont = '' | 'song' | 'hei' | 'kai' | 'yahei'
export type RichAlign = '' | 'left' | 'center' | 'right'
export type RichList = '' | 'ul' | 'ol'

export type RichStyle = {
  bold: boolean
  italic: boolean
  underline: boolean
  strike: boolean
  color: string
  highlight: string
  size: number
  font: RichFont
  align: RichAlign
  list: RichList
}

export type RichRun = {
  text: string
  style: RichStyle
}

export const richFonts: { id: Exclude<RichFont, ''>, label: string, face: string }[] = [
  { id: 'yahei', label: '雅黑', face: 'Microsoft YaHei' },
  { id: 'song', label: '宋体', face: 'SimSun' },
  { id: 'hei', label: '黑体', face: 'SimHei' },
  { id: 'kai', label: '楷体', face: 'KaiTi' },
]

export const richSizes: { px: number, command: string }[] = [
  { px: 12, command: '2' },
  { px: 14, command: '3' },
  { px: 16, command: '4' },
  { px: 18, command: '5' },
  { px: 20, command: '6' },
  { px: 24, command: '7' },
]

export const richColors = ['#1F2329', '#E11D48', '#F97316', '#16A34A', '#0B6BCB', '#7C3AED']

export const richMarks = ['#FDE68A', '#BBF7D0', '#BFDBFE', '#FBCFE8', '#FED7AA', '#E5E7EB']

export type RichBlock =
  | { kind: 'line', align: RichAlign, runs: RichRun[] }
  | { kind: 'list', list: Exclude<RichList, ''>, items: RichRun[][] }

export const fontStacks: Record<Exclude<RichFont, ''>, string> = {
  song: '"Songti SC", SimSun, serif',
  hei: '"Heiti SC", SimHei, sans-serif',
  kai: '"Kaiti SC", KaiTi, serif',
  yahei: '"PingFang SC", "Microsoft YaHei", sans-serif',
}

const sizeByCommand: Record<string, number> = Object.fromEntries(richSizes.map((item) => [item.command, item.px]))

const tagRe = /\[\[(\/|b|i|u|x|c=[0-9A-Fa-f]{6}|h=[0-9A-Fa-f]{6}|s=(?:12|14|16|18|20|24)|f=(?:song|hei|kai|yahei)|a=(?:left|center|right)|l=(?:ul|ol))\]\]/g

export function emptyStyle(): RichStyle {
  return {
    bold: false,
    italic: false,
    underline: false,
    strike: false,
    color: '',
    highlight: '',
    size: 0,
    font: '',
    align: '',
    list: '',
  }
}

export function isShake(body: string): boolean {
  return body.trim() === '[[shake]]'
}

export function plainText(body: string): string {
  if (isShake(body)) {
    return '抖动了窗口'
  }
  return parseRich(body).map((run) => run.text).join('')
}

export function parseRich(input: string): RichRun[] {
  const runs: RichRun[] = []
  const stack: RichStyle[] = [emptyStyle()]
  let last = 0
  for (const match of input.matchAll(tagRe)) {
    const index = match.index ?? 0
    pushRun(runs, input.slice(last, index), stack[stack.length - 1])
    const token = match[1] ?? ''
    if (token === '/') {
      if (stack.length > 1) {
        stack.pop()
      }
    } else {
      const next = { ...stack[stack.length - 1] }
      if (token === 'b') {
        next.bold = true
      } else if (token === 'i') {
        next.italic = true
      } else if (token === 'u') {
        next.underline = true
      } else if (token === 'x') {
        next.strike = true
      } else if (token.startsWith('c=')) {
        next.color = `#${token.slice(2).toUpperCase()}`
      } else if (token.startsWith('h=')) {
        next.highlight = `#${token.slice(2).toUpperCase()}`
      } else if (token.startsWith('s=')) {
        next.size = Number(token.slice(2))
      } else if (token === 'f=song' || token === 'f=hei' || token === 'f=kai' || token === 'f=yahei') {
        next.font = token.slice(2) as RichFont
      } else if (token === 'a=left' || token === 'a=center' || token === 'a=right') {
        next.align = token.slice(2) as RichAlign
      } else if (token === 'l=ul' || token === 'l=ol') {
        next.list = token.slice(2) as RichList
      }
      stack.push(next)
    }
    last = index + match[0].length
  }
  pushRun(runs, input.slice(last), stack[stack.length - 1])
  return runs
}

export function serializeEditor(root: HTMLElement): string {
  return serializeChildren(root).replace(/\n+$/, '')
}

export function richBlocks(input: string): RichBlock[] {
  const lines = splitLines(parseRich(input))
  const blocks: RichBlock[] = []
  for (const line of lines) {
    const list = line.find((run) => run.style.list === 'ul' || run.style.list === 'ol')?.style.list
    if (list === 'ul' || list === 'ol') {
      const prev = blocks[blocks.length - 1]
      if (prev && prev.kind === 'list' && prev.list === list) {
        prev.items.push(line)
      } else {
        blocks.push({ kind: 'list', list, items: [line] })
      }
      continue
    }
    const align = line.find((run) => run.style.align)?.style.align ?? ''
    blocks.push({
      kind: 'line',
      align: align === 'center' || align === 'right' || align === 'left' ? align : '',
      runs: line,
    })
  }
  return blocks
}

function splitLines(runs: RichRun[]): RichRun[][] {
  const lines: RichRun[][] = [[]]
  for (const run of runs) {
    const parts = run.text.split('\n')
    parts.forEach((part, index) => {
      if (index > 0) {
        lines.push([])
      }
      if (part !== '') {
        lines[lines.length - 1].push({ text: part, style: run.style })
      }
    })
  }
  return lines
}

function pushRun(runs: RichRun[], text: string, style: RichStyle) {
  if (text === '') {
    return
  }
  const prev = runs[runs.length - 1]
  if (prev && sameStyle(prev.style, style)) {
    prev.text += text
    return
  }
  runs.push({ text, style: { ...style } })
}

function sameStyle(a: RichStyle, b: RichStyle): boolean {
  return a.bold === b.bold
    && a.italic === b.italic
    && a.underline === b.underline
    && a.strike === b.strike
    && a.color === b.color
    && a.highlight === b.highlight
    && a.size === b.size
    && a.font === b.font
    && a.align === b.align
    && a.list === b.list
}

function serializeChildren(parent: Node): string {
  let out = ''
  for (const node of parent.childNodes) {
    if (isBlock(node) && out !== '' && !out.endsWith('\n')) {
      out += '\n'
    }
    out += serializeNode(node)
  }
  return out
}

function serializeNode(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) {
    return node.textContent ?? ''
  }
  if (!(node instanceof HTMLElement)) {
    return ''
  }
  if (node.tagName === 'BR') {
    return '\n'
  }
  if (node.tagName === 'UL' || node.tagName === 'OL') {
    return serializeList(node)
  }
  if (node.classList.contains('draft-code')) {
    const select = node.querySelector('select')
    const lang = select instanceof HTMLSelectElement ? select.value : ''
    const code = node.querySelector('.draft-code-body')
    const body = code instanceof HTMLElement ? code.innerText : ''
    const fence = /^[\w#+.-]{1,32}$/.test(lang) ? `\`\`\`${lang}` : '```'
    return `\n${fence}\n${body.replace(/\n$/, '')}\n\`\`\`\n`
  }
  if (node.tagName === 'BLOCKQUOTE') {
    const inner = serializeChildren(node).replace(/\n+$/, '')
    return `${inner.split('\n').map((line) => (line ? `「${line}」` : line)).join('\n')}\n`
  }
  if (node.tagName === 'A') {
    const href = node.getAttribute('href') ?? ''
    const text = serializeChildren(node)
    if (/^https?:\/\//i.test(href) && text !== href) {
      return `${text} ${href}`
    }
    return text || href
  }
  const inner = serializeChildren(node)
  return wrapStyle(inner, styleFromElement(node))
}

function serializeList(list: HTMLElement): string {
  const kind = list.tagName === 'OL' ? 'ol' : 'ul'
  let out = ''
  for (const child of list.children) {
    if (!(child instanceof HTMLElement) || child.tagName !== 'LI') {
      continue
    }
    const inner = serializeChildren(child).replace(/\n+$/, '')
    out += `[[l=${kind}]]${inner}[[/]]\n`
  }
  return out
}

function isBlock(node: Node): boolean {
  return node instanceof HTMLElement && (node.tagName === 'DIV' || node.tagName === 'P')
}

function wrapStyle(text: string, style: RichStyle): string {
  if (text === '') {
    return ''
  }
  let open = ''
  let close = ''
  if (style.font) {
    open += `[[f=${style.font}]]`
    close = `[[/]]${close}`
  }
  if (style.size) {
    open += `[[s=${style.size}]]`
    close = `[[/]]${close}`
  }
  if (style.color) {
    open += `[[c=${style.color.slice(1)}]]`
    close = `[[/]]${close}`
  }
  if (style.highlight) {
    open += `[[h=${style.highlight.slice(1)}]]`
    close = `[[/]]${close}`
  }
  if (style.underline) {
    open += '[[u]]'
    close = `[[/]]${close}`
  }
  if (style.strike) {
    open += '[[x]]'
    close = `[[/]]${close}`
  }
  if (style.italic) {
    open += '[[i]]'
    close = `[[/]]${close}`
  }
  if (style.bold) {
    open += '[[b]]'
    close = `[[/]]${close}`
  }
  if (style.align) {
    open = `[[a=${style.align}]]${open}`
    close = `${close}[[/]]`
  }
  return `${open}${text}${close}`
}

function styleFromElement(el: HTMLElement): RichStyle {
  const style = emptyStyle()
  const tag = el.tagName
  const weight = el.style.fontWeight
  const deco = `${el.style.textDecorationLine} ${el.style.textDecoration}`
  style.bold = tag === 'B' || tag === 'STRONG' || weight === 'bold' || (weight !== '' && Number(weight) >= 600)
  style.italic = tag === 'I' || tag === 'EM' || el.style.fontStyle === 'italic'
  style.underline = tag === 'U' || deco.includes('underline')
  style.strike = tag === 'S' || tag === 'STRIKE' || tag === 'DEL' || deco.includes('line-through')
  style.highlight = highlightFrom(el.style.backgroundColor)
  const align = (el.style.textAlign || el.getAttribute('align') || '').toLowerCase()
  if (align === 'left' || align === 'center' || align === 'right') {
    style.align = align
  }
  const inlineColor = tag === 'FONT' ? el.getAttribute('color') ?? '' : el.style.color
  style.color = normalizeColor(inlineColor)
  const sizeAttr = tag === 'FONT' ? el.getAttribute('size') ?? '' : ''
  if (sizeByCommand[sizeAttr]) {
    style.size = sizeByCommand[sizeAttr]
  } else {
    const px = /^(\d+)px$/.exec(el.style.fontSize)
    const value = px ? Number(px[1]) : 0
    if (richSizes.some((item) => item.px === value)) {
      style.size = value
    }
  }
  const face = tag === 'FONT' ? el.getAttribute('face') ?? '' : el.style.fontFamily
  style.font = fontKey(face)
  return style
}

function fontKey(face: string): RichFont {
  const value = face.toLowerCase()
  if (value.includes('simsun') || value.includes('songti') || value.includes('宋')) {
    return 'song'
  }
  if (value.includes('yahei') || value.includes('pingfang') || value.includes('雅黑')) {
    return 'yahei'
  }
  if (value.includes('simhei') || value.includes('heiti') || value.includes('黑体')) {
    return 'hei'
  }
  if (value.includes('kaiti') || value.includes('楷')) {
    return 'kai'
  }
  return ''
}

function highlightFrom(value: string): string {
  const color = normalizeColor(value)
  if (color === '' || color === '#FFFFFF') {
    return ''
  }
  return color
}

function normalizeColor(value: string): string {
  const text = value.trim()
  const hex = /^#([0-9a-fA-F]{6})$/.exec(text)
  if (hex) {
    return `#${hex[1].toUpperCase()}`
  }
  const rgb = /^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)(?:\s*,\s*([\d.]+))?/.exec(text)
  if (!rgb) {
    return ''
  }
  if (rgb[4] != null && Number(rgb[4]) === 0) {
    return ''
  }
  const channels = [rgb[1], rgb[2], rgb[3]].map((channel) => Number(channel).toString(16).padStart(2, '0'))
  return `#${channels.join('').toUpperCase()}`
}
