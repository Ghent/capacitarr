<script setup lang="ts">
const commands = [
  { prompt: '$ ', text: 'mkdir capacitarr && cd capacitarr', delay: 40 },
  { prompt: '$ ', text: 'docker compose up -d', delay: 50 },
  { prompt: '', text: '⠋ Pulling capacitarr:latest...', delay: 0, isOutput: true, pause: 600 },
  { prompt: '', text: '⠙ Creating container...', delay: 0, isOutput: true, pause: 400 },
  { prompt: '', text: '✓ Container started on port 2187', delay: 0, isOutput: true, class: 'text-emerald-400' },
  { prompt: '', text: '', delay: 0, isOutput: true, pause: 200 },
  { prompt: '$ ', text: 'open http://localhost:2187', delay: 45 },
  { prompt: '', text: '🚀 Capacitarr is ready!', delay: 0, isOutput: true, class: 'text-violet-400 font-semibold' },
]

const termRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)
const displayLines = ref<Array<{ text: string; class?: string }>>([])
const currentLine = ref('')
const showCursor = ref(true)
const isTyping = ref(false)
const copied = ref(false)

const composeYaml = `services:
  capacitarr:
    image: ghcr.io/ghent/capacitarr:latest
    ports:
      - "2187:2187"
    volumes:
      - ./data:/data
    restart: unless-stopped`

async function sleep(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

async function typeText(text: string, delay: number) {
  isTyping.value = true
  for (let i = 0; i < text.length; i++) {
    currentLine.value += text[i]
    await sleep(delay + Math.random() * 20)
  }
  isTyping.value = false
}

async function runAnimation() {
  for (const cmd of commands) {
    currentLine.value = cmd.prompt
    if (cmd.isOutput) {
      if (cmd.pause) await sleep(cmd.pause)
      currentLine.value = cmd.text
      displayLines.value.push({ text: currentLine.value, class: cmd.class })
      currentLine.value = ''
      await sleep(100)
    } else {
      await sleep(300)
      await typeText(cmd.text, cmd.delay)
      displayLines.value.push({ text: currentLine.value })
      currentLine.value = ''
      await sleep(400)
    }
  }
  showCursor.value = false
}

async function copyCompose() {
  try {
    await navigator.clipboard.writeText(composeYaml)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    // Fallback for non-HTTPS
    const ta = document.createElement('textarea')
    ta.value = composeYaml
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  }
}

onMounted(() => {
  if (!termRef.value) return
  const observer = new IntersectionObserver(
    ([entry]) => {
      if (entry.isIntersecting) {
        isVisible.value = true
        runAnimation()
        observer.disconnect()
      }
    },
    { threshold: 0.3 },
  )
  observer.observe(termRef.value)
})
</script>

<template>
  <div ref="termRef" class="terminal-wrapper" :class="{ visible: isVisible }">
    <div class="terminal">
      <div class="terminal-header">
        <div class="terminal-dots">
          <span class="td td-red" />
          <span class="td td-yellow" />
          <span class="td td-green" />
        </div>
        <span class="terminal-title">Terminal</span>
        <button
          class="terminal-copy"
          :class="{ copied }"
          @click="copyCompose"
          :title="copied ? 'Copied!' : 'Copy docker-compose.yml'"
        >
          <UIcon :name="copied ? 'i-lucide-check' : 'i-lucide-clipboard'" class="size-3.5" />
          <span>{{ copied ? 'Copied!' : 'Copy' }}</span>
        </button>
      </div>
      <div class="terminal-body">
        <div
          v-for="(line, i) in displayLines"
          :key="i"
          class="terminal-line"
          :class="line.class"
        >{{ line.text }}</div>
        <div class="terminal-line terminal-current">
          {{ currentLine }}<span v-if="showCursor" class="terminal-cursor" :class="{ typing: isTyping }">▋</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.terminal-wrapper {
  max-width: 36rem;
  margin: 0 auto;
  opacity: 0;
  transform: translateY(1.5rem);
  transition: opacity 0.6s ease, transform 0.6s ease;
}

.terminal-wrapper.visible {
  opacity: 1;
  transform: translateY(0);
}

.terminal {
  border-radius: 0.75rem;
  overflow: hidden;
  border: 1px solid var(--color-neutral-200);
  background: var(--color-neutral-950);
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.25),
    0 0 0 1px rgba(139, 92, 246, 0.1);
}

:root.dark .terminal {
  border-color: var(--color-neutral-800);
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 40px -15px rgba(139, 92, 246, 0.15);
}

.terminal-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 1rem;
  background: var(--color-neutral-900);
  border-bottom: 1px solid var(--color-neutral-800);
}

.terminal-dots {
  display: flex;
  gap: 0.375rem;
}

.td {
  width: 0.625rem;
  height: 0.625rem;
  border-radius: 50%;
}

.td-red { background: #ff5f57; }
.td-yellow { background: #febc2e; }
.td-green { background: #28c840; }

.terminal-title {
  flex: 1;
  font-size: 0.6875rem;
  font-family: var(--font-mono);
  color: var(--color-neutral-500);
}

.terminal-copy {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.6875rem;
  font-family: var(--font-mono);
  color: var(--color-neutral-400);
  background: var(--color-neutral-800);
  border: 1px solid var(--color-neutral-700);
  cursor: pointer;
  transition: all 0.2s;
}

.terminal-copy:hover {
  color: white;
  border-color: var(--color-neutral-600);
}

.terminal-copy.copied {
  color: var(--color-emerald-400);
  border-color: var(--color-emerald-700);
}

.terminal-body {
  padding: 1rem 1.25rem;
  font-family: var(--font-mono);
  font-size: 0.8125rem;
  line-height: 1.75;
  color: var(--color-neutral-300);
  min-height: 12rem;
}

.terminal-line {
  white-space: pre-wrap;
  min-height: 1.3em;
}

.terminal-current {
  display: flex;
}

.terminal-cursor {
  animation: blink 1s step-end infinite;
  color: var(--color-violet-400);
  margin-left: 1px;
}

.terminal-cursor.typing {
  animation: none;
  opacity: 1;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}
</style>
