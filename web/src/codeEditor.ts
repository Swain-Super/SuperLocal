import { languages } from '@codemirror/language-data'
import { Compartment, EditorState, type Extension } from '@codemirror/state'
import { EditorView, basicSetup } from 'codemirror'

const live = new Map<HTMLElement, { view: EditorView, lang: Compartment, wrap: Compartment }>()
const tickets = new Map<HTMLElement, number>()
const loaded = new Map<string, Promise<Extension>>()

const noneLabel = '无标题'

type LangChoice = { name: string, label: string, aliases: readonly string[] }

const extraAliases: Record<string, readonly string[]> = {
  Go: ['golang'],
  Rust: ['rs'],
  Ruby: ['rb'],
  Kotlin: ['kt'],
  JavaScript: ['nodejs'],
  'Objective-C': ['objc'],
  'Objective-C++': ['objc++'],
  Haskell: ['hs'],
}

const catalog: LangChoice[] = [
  { name: '', label: noneLabel, aliases: ['无标题', 'plain', 'text'] },
  ...languages
    .map((lang) => ({
      name: lang.name,
      label: lang.name,
      aliases: [...lang.alias, ...(extraAliases[lang.name] ?? [])],
    }))
    .sort((a, b) => a.label.localeCompare(b.label, 'en', { sensitivity: 'base' })),
]

const chrome = EditorView.theme({
  '&': {
    backgroundColor: '#fff',
    color: '#1f2329',
    fontSize: '13px',
  },
  '&.cm-focused': {
    outline: 'none',
  },
  '.cm-scroller': {
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    lineHeight: '1.6',
    minHeight: '120px',
    maxHeight: '280px',
    overflow: 'auto',
  },
  '.cm-content': {
    padding: '8px 0 12px',
    caretColor: '#0b6bcb',
  },
  '.cm-line': {
    padding: '0 12px',
  },
  '.cm-gutters': {
    backgroundColor: '#f7f8fa',
    color: '#8a9099',
    border: 'none',
    borderRight: '1px solid #eef0f2',
  },
  '.cm-lineNumbers .cm-gutterElement': {
    padding: '0 10px 0 12px',
    minWidth: '28px',
  },
  '.cm-activeLine': {
    backgroundColor: 'rgba(11, 107, 203, 0.06)',
  },
  '.cm-activeLineGutter': {
    backgroundColor: '#eef3f8',
    color: '#646a73',
  },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
    backgroundColor: '#d6e6f8',
  },
  '.cm-cursor, .cm-dropCursor': {
    borderLeftColor: '#0b6bcb',
  },
})

function loadCodeLanguage(name: string): Promise<Extension> {
  const desc = languages.find((lang) => lang.name === name)
  if (!desc) {
    return Promise.resolve([])
  }
  let job = loaded.get(desc.name)
  if (!job) {
    job = desc.load().then((support) => support.extension).catch(() => [])
    loaded.set(desc.name, job)
  }
  return job
}

export function mountCodeEditor(host: HTMLElement, text: string, lang: string, onDoc: (text: string) => void) {
  const langBox = new Compartment()
  const wrapBox = new Compartment()
  const view = new EditorView({
    parent: host,
    state: EditorState.create({
      doc: text,
      extensions: [
        basicSetup,
        langBox.of([]),
        wrapBox.of(EditorView.lineWrapping),
        chrome,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            onDoc(update.state.doc.toString())
          }
        }),
      ],
    }),
  })
  live.set(host, { view, lang: langBox, wrap: wrapBox })
  if (lang) {
    setCodeLanguage(host, lang)
  }
  return view
}

export function setCodeLanguage(host: HTMLElement, lang: string) {
  const item = live.get(host)
  if (!item) {
    return
  }
  const ticket = (tickets.get(host) ?? 0) + 1
  tickets.set(host, ticket)
  void loadCodeLanguage(lang).then((extension) => {
    if (tickets.get(host) !== ticket || !live.has(host)) {
      return
    }
    item.view.dispatch({ effects: item.lang.reconfigure(extension) })
  })
}

export function setCodeWrap(host: HTMLElement, wrap: boolean) {
  const item = live.get(host)
  if (!item) {
    return
  }
  item.view.dispatch({ effects: item.wrap.reconfigure(wrap ? EditorView.lineWrapping : []) })
}

export function focusCodeEditor(host: HTMLElement) {
  live.get(host)?.view.focus()
}

export function releaseCodeEditors(root: ParentNode) {
  for (const [host, item] of live) {
    if (!host.isConnected || !root.contains(host)) {
      item.view.destroy()
      live.delete(host)
      tickets.delete(host)
    }
  }
  for (const picker of [...pickers]) {
    if (!picker.card.isConnected || !root.contains(picker.card)) {
      picker.close()
      pickers.delete(picker)
    }
  }
}

export function destroyCodeEditors() {
  for (const [host, item] of live) {
    item.view.destroy()
    live.delete(host)
    tickets.delete(host)
  }
  for (const picker of pickers) {
    picker.close()
  }
  pickers.clear()
}

type LangPicker = {
  card: HTMLElement
  button: HTMLButtonElement
  menu: HTMLElement | null
  close: () => void
}

const pickers = new Set<LangPicker>()
let pickerListening = false

function ensurePickerStyle() {
  if (document.getElementById('sl-lang-style')) {
    return
  }
  const style = document.createElement('style')
  style.id = 'sl-lang-style'
  style.textContent = `
.draft-lang-menu {
  position: fixed;
  z-index: 80;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  width: 240px;
  padding: 8px;
  border: 1px solid #e6e8eb;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(31, 35, 41, 0.12);
}
.draft-lang-search {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
  height: 32px;
  margin-bottom: 6px;
  padding: 0 8px;
  border: 1px solid #e6e8eb;
  border-radius: 8px;
  background: #fff;
  color: #8a9099;
}
.draft-lang-search:focus-within {
  border-color: #0b6bcb;
}
.draft-lang-search svg {
  width: 14px;
  height: 14px;
  flex: none;
}
.draft-lang-search input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: #1f2329;
  font: inherit;
  font-size: 13px;
}
.draft-lang-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.draft-lang-list button {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 32px;
  padding: 0 8px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: #1f2329;
  font: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}
.draft-lang-list button .name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.draft-lang-list button.on {
  color: #0b6bcb;
}
.draft-lang-list button.active,
.draft-lang-list button:hover {
  background: #f3f5f7;
}
.draft-lang-list button .mark {
  width: 14px;
  flex: none;
  color: #0b6bcb;
  font-size: 12px;
  text-align: center;
}
.draft-lang-empty {
  padding: 12px 8px;
  color: #8a9099;
  font-size: 13px;
}
`
  document.head.appendChild(style)
}

function matchesLang(item: LangChoice, query: string) {
  const q = query.trim().toLowerCase()
  if (q === '') {
    return true
  }
  if (item.label.toLowerCase().includes(q)) {
    return true
  }
  return item.aliases.some((alias) => alias.includes(q))
}

function langRank(item: LangChoice, query: string) {
  const q = query.trim().toLowerCase()
  if (item.label.toLowerCase() === q || item.aliases.some((alias) => alias === q)) {
    return 0
  }
  if (item.label.toLowerCase().startsWith(q)) {
    return 1
  }
  return 2
}

function listenForPickerDismiss() {
  if (pickerListening) {
    return
  }
  pickerListening = true
  document.addEventListener('pointerdown', (event) => {
    const target = event.target
    if (!(target instanceof Node)) {
      return
    }
    for (const picker of pickers) {
      if (!picker.menu) {
        continue
      }
      if (picker.menu.contains(target) || picker.button.contains(target)) {
        continue
      }
      picker.close()
    }
  })
  document.addEventListener('scroll', (event) => {
    const target = event.target
    for (const picker of pickers) {
      if (!picker.menu) {
        continue
      }
      if (target instanceof Node && picker.menu.contains(target)) {
        continue
      }
      picker.close()
    }
  }, true)
  window.addEventListener('resize', () => {
    for (const picker of pickers) {
      picker.close()
    }
  })
}

function scrollLangRow(row: HTMLElement, box: HTMLElement) {
  const top = row.offsetTop - box.offsetTop
  const bottom = top + row.offsetHeight
  if (top < box.scrollTop) {
    box.scrollTop = top
  } else if (bottom > box.scrollTop + box.clientHeight) {
    box.scrollTop = bottom - box.clientHeight
  }
}

function placeLangMenu(menu: HTMLElement, anchor: HTMLElement) {
  const rect = anchor.getBoundingClientRect()
  const width = 240
  const gap = 6
  const below = window.innerHeight - rect.bottom
  const above = rect.top
  const openUp = below < 260 && above > below
  const room = Math.max(160, Math.min(360, (openUp ? above : below) - gap - 8))
  menu.style.left = `${Math.max(8, Math.min(rect.left, window.innerWidth - width - 8))}px`
  menu.style.width = `${width}px`
  menu.style.maxHeight = `${room}px`
  if (openUp) {
    menu.style.top = 'auto'
    menu.style.bottom = `${window.innerHeight - rect.top + gap}px`
  } else {
    menu.style.bottom = 'auto'
    menu.style.top = `${rect.bottom + gap}px`
  }
}

export function mountLanguagePicker(card: HTMLElement, host: HTMLElement, onChange: () => void) {
  const bar = card.querySelector('.draft-code-bar')
  if (!(bar instanceof HTMLElement)) {
    return
  }
  listenForPickerDismiss()
  const wrap = document.createElement('div')
  wrap.className = 'draft-lang-wrap'
  const button = document.createElement('button')
  button.type = 'button'
  button.className = 'draft-lang'
  button.setAttribute('aria-label', '语言')
  button.setAttribute('aria-haspopup', 'listbox')
  button.setAttribute('aria-expanded', 'false')
  const nameEl = document.createElement('span')
  nameEl.className = 'draft-lang-name'
  nameEl.textContent = noneLabel
  button.append(nameEl)
  const hidden = document.createElement('input')
  hidden.type = 'hidden'
  hidden.className = 'draft-lang-value'
  hidden.value = ''
  wrap.append(button, hidden)
  bar.prepend(wrap)

  let value = ''
  let menu: HTMLElement | null = null
  let list: HTMLElement | null = null
  let shown: LangChoice[] = []
  let active = 0

  const picker: LangPicker = {
    card,
    button,
    menu: null,
    close: () => undefined,
  }

  function close() {
    menu?.remove()
    menu = null
    list = null
    picker.menu = null
    button.setAttribute('aria-expanded', 'false')
  }

  function paintActive() {
    list?.querySelectorAll('button').forEach((row, index) => {
      row.classList.toggle('active', index === active)
    })
  }

  function choose(name: string) {
    value = name
    hidden.value = name
    nameEl.textContent = name || noneLabel
    close()
    setCodeLanguage(host, name)
    onChange()
    focusCodeEditor(host)
  }

  function render(query: string) {
    if (!list) {
      return
    }
    const q = query.trim().toLowerCase()
    shown = catalog.filter((item) => matchesLang(item, query))
    if (q !== '') {
      shown.sort((a, b) => langRank(a, q) - langRank(b, q) || a.label.localeCompare(b.label, 'en', { sensitivity: 'base' }))
    }
    const selected = shown.findIndex((item) => item.name === value)
    active = selected >= 0 ? selected : 0
    list.replaceChildren()
    if (shown.length === 0) {
      const empty = document.createElement('div')
      empty.className = 'draft-lang-empty'
      empty.textContent = '没有匹配的语言'
      list.append(empty)
      return
    }
    shown.forEach((item, index) => {
      const row = document.createElement('button')
      row.type = 'button'
      row.setAttribute('role', 'option')
      row.setAttribute('aria-selected', item.name === value ? 'true' : 'false')
      if (item.name === value) {
        row.classList.add('on')
      }
      if (index === active) {
        row.classList.add('active')
      }
      const label = document.createElement('span')
      label.className = 'name'
      label.textContent = item.label
      const mark = document.createElement('span')
      mark.className = 'mark'
      mark.textContent = item.name === value ? '✓' : ''
      row.append(label, mark)
      row.addEventListener('mousedown', (event) => {
        event.preventDefault()
        choose(item.name)
      })
      row.addEventListener('mouseenter', () => {
        active = index
        paintActive()
      })
      list?.append(row)
    })
  }

  function open() {
    for (const other of pickers) {
      if (other !== picker) {
        other.close()
      }
    }
    ensurePickerStyle()
    const panel = document.createElement('div')
    panel.className = 'draft-lang-menu'
    const searchRow = document.createElement('label')
    searchRow.className = 'draft-lang-search'
    searchRow.innerHTML = '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6" fill="none" stroke="currentColor" stroke-width="2"/><path d="M16 16l4 4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>'
    const input = document.createElement('input')
    input.type = 'text'
    input.placeholder = '搜索'
    input.setAttribute('aria-label', '搜索语言')
    input.autocomplete = 'off'
    input.spellcheck = false
    searchRow.append(input)
    const rows = document.createElement('div')
    rows.className = 'draft-lang-list'
    rows.setAttribute('role', 'listbox')
    rows.setAttribute('aria-label', '语言')
    panel.append(searchRow, rows)
    document.body.append(panel)
    menu = panel
    picker.menu = panel
    list = rows
    button.setAttribute('aria-expanded', 'true')
    render('')
    placeLangMenu(panel, button)
    const selectedRow = rows.querySelector('[aria-selected="true"]')
    if (selectedRow instanceof HTMLElement) {
      scrollLangRow(selectedRow, rows)
    }
    input.focus()
    input.addEventListener('input', () => render(input.value))
    input.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        close()
        button.focus()
        return
      }
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault()
        if (shown.length === 0) {
          return
        }
        active = (active + (event.key === 'ArrowDown' ? 1 : shown.length - 1)) % shown.length
        paintActive()
        const row = list?.querySelector('button.active')
        if (row instanceof HTMLElement && list) {
          scrollLangRow(row, list)
        }
        return
      }
      if (event.key === 'Enter') {
        event.preventDefault()
        const item = shown[active]
        if (item) {
          choose(item.name)
        }
      }
    })
  }

  picker.close = close
  pickers.add(picker)
  button.addEventListener('mousedown', (event) => {
    event.preventDefault()
  })
  button.addEventListener('click', (event) => {
    event.stopPropagation()
    if (menu) {
      close()
      return
    }
    open()
  })
}
