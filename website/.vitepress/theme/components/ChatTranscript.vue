<template>
  <div class="chat-transcript">
    <div
      v-for="(message, index) in transcript"
      :key="index"
      class="chat-message"
      :class="`chat-message-${message.role}`"
    >
      <div class="chat-message-meta">
        {{ message.role === 'user' ? 'You' : 'Assistant' }}
      </div>
      <div class="chat-bubble" v-html="renderMarkdown(message.content)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'

interface OpenWebUIMessage {
  id?: string
  parentId?: string | null
  childrenIds?: string[]
  role: 'user' | 'assistant' | string
  content?: string
  output?: Array<{
    type: string
    content?: Array<{ type: string; text?: string }>
    [key: string]: unknown
  }>
}

const props = defineProps<{
  messages: Record<string, OpenWebUIMessage>
}>()

const md = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: false,
})

const transcript = computed(() => {
  const messages = props.messages
  const entries = Object.values(messages)

  // Build the conversation chain from each root message.
  const roots = entries.filter((m) => !m.parentId)
  const result: Array<{ role: string; content: string }> = []

  for (const root of roots) {
    let current: OpenWebUIMessage | undefined = root
    while (current) {
      result.push({
        role: current.role,
        content: extractContent(current),
      })
      current = current.childrenIds?.length
        ? messages[current.childrenIds[0]]
        : undefined
    }
  }

  return result
})

function extractContent(message: OpenWebUIMessage): string {
  if (message.role === 'user') {
    return message.content || ''
  }

  if (!message.output?.length) {
    return message.content || ''
  }

  const parts: string[] = []
  for (const item of message.output) {
    if (item.type !== 'message' || !item.content) continue
    for (const c of item.content) {
      if (c.type === 'output_text' && c.text) {
        parts.push(c.text)
      }
    }
  }

  return parts.join('\n\n') || message.content || ''
}

function renderMarkdown(content: string): string {
  return md.render(content)
}
</script>

<style scoped>
.chat-transcript {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  margin: 1.5rem 0;
  padding: 1rem;
  border-radius: 12px;
  background: var(--vp-c-bg-soft);
}

.chat-message {
  display: flex;
  flex-direction: column;
  max-width: 85%;
}

.chat-message-user {
  align-self: flex-end;
}

.chat-message-assistant {
  align-self: flex-start;
}

.chat-message-meta {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--vp-c-text-2);
  margin-bottom: 0.25rem;
  padding: 0 0.5rem;
  text-transform: capitalize;
}

.chat-bubble {
  padding: 0.75rem 1rem;
  border-radius: 1rem;
  line-height: 1.5;
  color: var(--vp-c-text-1);
  background: var(--vp-c-bg);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

.chat-message-user .chat-bubble {
  border-bottom-right-radius: 0.25rem;
  background: var(--vp-c-brand-1);
  color: #fff;
}

.chat-message-user .chat-bubble :deep(a) {
  color: #fff;
  text-decoration: underline;
}

.chat-message-user .chat-bubble :deep(code) {
  background: rgba(255, 255, 255, 0.2);
  color: #fff;
}

.chat-message-assistant .chat-bubble {
  border-bottom-left-radius: 0.25rem;
}

.chat-bubble :deep(p) {
  margin: 0 0 0.75rem;
}

.chat-bubble :deep(p:last-child) {
  margin-bottom: 0;
}

.chat-bubble :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0.75rem 0;
  font-size: 0.875rem;
}

.chat-bubble :deep(th),
.chat-bubble :deep(td) {
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--vp-c-divider);
  text-align: left;
}

.chat-bubble :deep(th) {
  background: var(--vp-c-bg-soft);
  font-weight: 600;
}

.chat-bubble :deep(ul),
.chat-bubble :deep(ol) {
  margin: 0.5rem 0;
  padding-left: 1.25rem;
}

.chat-bubble :deep(li) {
  margin: 0.25rem 0;
}

.chat-bubble :deep(code) {
  padding: 0.125rem 0.25rem;
  border-radius: 4px;
  background: var(--vp-c-bg-soft);
  font-family: var(--vp-font-family-mono);
  font-size: 0.85em;
}

.chat-bubble :deep(pre) {
  padding: 0.75rem;
  border-radius: 8px;
  background: var(--vp-c-bg-alt);
  overflow-x: auto;
}

.chat-bubble :deep(pre code) {
  background: transparent;
  padding: 0;
}

@media (max-width: 640px) {
  .chat-message {
    max-width: 95%;
  }
}
</style>
