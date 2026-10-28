import { defineConfig, presetUno } from 'unocss'

export default defineConfig({
  presets: [presetUno()],
  theme: {
    colors: {
      primary: '#0B6BCB',
      pressed: '#0958A8',
      page: '#F3F5F7',
      panel: '#FFFFFF',
      border: '#E6E8EB',
      text: '#1F2329',
      secondary: '#646A73',
      online: '#22A06B',
      bubble: '#F2F3F5',
    },
    fontFamily: {
      sans: '"PingFang SC", "Microsoft YaHei", sans-serif',
    },
    fontSize: {
      body: '14px',
      aux: '12px',
    },
    borderRadius: {
      panel: '8px',
    },
  },
})
