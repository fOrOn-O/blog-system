<script setup>
import { computed } from 'vue'
import { renderAssistantMarkdown } from '@/utils/assistant-markdown'
const props = defineProps({ text: { type: String, default: '' } })
const html = computed(() => renderAssistantMarkdown(props.text))
</script>

<template>
  <!-- 仅渲染经过 DOMPurify 清洗的助手消息，不用于展示或批准 proposal HTML。 -->
  <div class="assistant-markdown" v-html="html" />
</template>

<style scoped>
.assistant-markdown { min-width: 0; overflow-wrap: anywhere; line-height: 1.7; }
.assistant-markdown :deep(p) { margin: .55em 0; }
.assistant-markdown :deep(ul), .assistant-markdown :deep(ol) { padding-left: 1.6em; }
.assistant-markdown :deep(pre) { max-width: 100%; overflow: auto; padding: 12px; border-radius: 6px; background: #e9eef5; }
.assistant-markdown :deep(code) { font-family: monospace; font-size: .9em; overflow-wrap: anywhere; }
.assistant-markdown :deep(pre code) { white-space: pre; overflow-wrap: normal; }
.assistant-markdown :deep(blockquote) { border-left: 3px solid #9bacbf; padding-left: 12px; margin-left: 0; color: #52667a; }
.assistant-markdown :deep(table) { display: block; max-width: 100%; overflow-x: auto; border-collapse: collapse; }
.assistant-markdown :deep(th), .assistant-markdown :deep(td) { border: 1px solid #c9d3df; padding: 6px 10px; min-width: 90px; }
.assistant-markdown :deep(a) { color: #285ab0; text-decoration: underline; overflow-wrap: anywhere; }
</style>
