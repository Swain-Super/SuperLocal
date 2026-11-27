const size = 33
const dataBytes = 80
const eccBytes = 20

export function qrModules(text: string): boolean[][] {
  const bits = dataBits(text)
  const func = blank()
  const modules = blank()
  drawFunction(func, modules)
  placeData(func, modules, bits)
  drawFormat(modules)
  return modules
}

function blank(): boolean[][] {
  return Array.from({ length: size }, () => Array.from({ length: size }, () => false))
}

function dataBits(text: string): number[] {
  const bytes = Array.from(new TextEncoder().encode(text))
  if (bytes.length > 78) throw new Error('加入地址太长')
  const bits: number[] = []
  const push = (value: number, length: number) => {
    for (let i = length - 1; i >= 0; i -= 1) bits.push((value >>> i) & 1)
  }
  push(0b0100, 4)
  push(bytes.length, 8)
  for (const byte of bytes) push(byte, 8)
  const capacity = dataBytes * 8
  for (let i = 0; i < Math.min(4, capacity - bits.length); i += 1) bits.push(0)
  while (bits.length % 8 !== 0) bits.push(0)
  const words: number[] = []
  for (let i = 0; i < bits.length; i += 8) {
    let value = 0
    for (let j = 0; j < 8; j += 1) value = (value << 1) | bits[i + j]
    words.push(value)
  }
  for (let i = 0; words.length < dataBytes; i += 1) words.push(i % 2 === 0 ? 0xec : 0x11)
  const all = words.concat(reedSolomon(words, eccBytes))
  const out: number[] = []
  for (const word of all) {
    for (let i = 7; i >= 0; i -= 1) out.push((word >>> i) & 1)
  }
  for (let i = 0; i < 7; i += 1) out.push(0)
  return out
}

function reedSolomon(data: number[], degree: number): number[] {
  const divisor = new Array<number>(degree).fill(0)
  divisor[degree - 1] = 1
  let root = 1
  for (let i = 0; i < degree; i += 1) {
    for (let j = 0; j < divisor.length; j += 1) {
      divisor[j] = gfMul(divisor[j], root)
      if (j + 1 < divisor.length) divisor[j] ^= divisor[j + 1]
    }
    root = gfMul(root, 2)
  }
  const rest = new Array<number>(degree).fill(0)
  for (const byte of data) {
    const factor = (byte ^ rest[0]) & 0xff
    rest.copyWithin(0, 1)
    rest[degree - 1] = 0
    for (let i = 0; i < degree; i += 1) rest[i] ^= gfMul(divisor[i], factor)
  }
  return rest
}

function gfMul(a: number, b: number): number {
  let product = 0
  for (let i = 0; i < 8; i += 1) {
    if ((b & 1) !== 0) product ^= a
    a = (a & 0x80) !== 0 ? ((a << 1) ^ 0x11d) & 0xff : (a << 1) & 0xff
    b >>= 1
  }
  return product
}

function drawFunction(func: boolean[][], modules: boolean[][]) {
  const markFinder = (row: number, col: number) => {
    for (let y = -1; y <= 7; y += 1) {
      for (let x = -1; x <= 7; x += 1) {
        const r = row + y
        const c = col + x
        if (r < 0 || c < 0 || r >= size || c >= size) continue
        func[r][c] = true
        const edge = x === 0 || y === 0 || x === 6 || y === 6
        const core = x >= 2 && x <= 4 && y >= 2 && y <= 4
        modules[r][c] = x >= 0 && x <= 6 && y >= 0 && y <= 6 && (edge || core)
      }
    }
  }
  markFinder(0, 0)
  markFinder(0, size - 7)
  markFinder(size - 7, 0)
  for (const center of [6, 26]) {
    for (const other of [6, 26]) {
      if (func[center][other]) continue
      for (let y = -2; y <= 2; y += 1) {
        for (let x = -2; x <= 2; x += 1) {
          func[center + y][other + x] = true
          const edge = Math.abs(x) === 2 || Math.abs(y) === 2
          modules[center + y][other + x] = edge || (x === 0 && y === 0)
        }
      }
    }
  }
  for (let i = 0; i < size; i += 1) {
    if (!func[6][i]) {
      func[6][i] = true
      modules[6][i] = i % 2 === 0
    }
    if (!func[i][6]) {
      func[i][6] = true
      modules[i][6] = i % 2 === 0
    }
  }
  func[size - 8][8] = true
  modules[size - 8][8] = true
  for (let i = 0; i < 9; i += 1) {
    func[8][i] = true
    func[i][8] = true
  }
  for (let i = 0; i < 8; i += 1) {
    func[8][size - 1 - i] = true
    func[size - 1 - i][8] = true
  }
}

function placeData(func: boolean[][], modules: boolean[][], bits: number[]) {
  let index = 0
  let upward = true
  for (let right = size - 1; right >= 1; right -= 2) {
    if (right === 6) right = 5
    for (let vert = 0; vert < size; vert += 1) {
      const row = upward ? size - 1 - vert : vert
      for (let j = 0; j < 2; j += 1) {
        const col = right - j
        if (func[row][col]) continue
        const bit = index < bits.length ? bits[index] : 0
        index += 1
        const dark = bit === 1
        modules[row][col] = (row + col) % 2 === 0 ? !dark : dark
      }
    }
    upward = !upward
  }
}

function drawFormat(modules: boolean[][]) {
  const bits = [1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0]
  const alongRow = [0, 1, 2, 3, 4, 5, 7, 8]
  for (let i = 0; i < alongRow.length; i += 1) modules[8][alongRow[i]] = bits[i] === 1
  const downCol = [7, 5, 4, 3, 2, 1, 0]
  for (let i = 0; i < downCol.length; i += 1) modules[downCol[i]][8] = bits[8 + i] === 1
  for (let i = 0; i < 7; i += 1) modules[size - 1 - i][8] = bits[i] === 1
  for (let i = 0; i < 8; i += 1) modules[8][size - 8 + i] = bits[7 + i] === 1
}
