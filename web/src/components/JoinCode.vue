<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { joinURL } from '../api'
import { qrModules } from '../qr'

const url = ref('')
const cells = ref<{ x: number; y: number }[]>([])

onMounted(async () => {
  try {
    const next = await joinURL()
    if (!next) return
    const marks: { x: number; y: number }[] = []
    qrModules(next).forEach((row, y) => {
      row.forEach((on, x) => {
        if (on) marks.push({ x, y })
      })
    })
    cells.value = marks
    url.value = next
  } catch {
    url.value = ''
  }
})
</script>

<template>
  <section v-if="url" class="join">
    <p class="join-title">快捷加入</p>
    <svg class="join-code" viewBox="0 0 41 41" role="img" aria-label="快捷加入二维码">
      <rect width="41" height="41" fill="#ffffff" />
      <rect
        v-for="cell in cells"
        :key="`${cell.x}-${cell.y}`"
        :x="cell.x + 4"
        :y="cell.y + 4"
        width="1"
        height="1"
        fill="#1f2329"
      />
    </svg>
    <p class="join-url">{{ url }}</p>
  </section>
</template>

<style scoped>
.join {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid #e6e8eb;
}
.join-title {
  margin: 0;
  color: #1f2329;
  font-size: 14px;
  font-weight: 600;
}
.join-code {
  width: 148px;
  height: 148px;
  background: #fff;
}
.join-url {
  margin: 0;
  max-width: 100%;
  color: #646a73;
  font-size: 12px;
  line-height: 1.4;
  text-align: center;
  word-break: break-all;
}
</style>
