<script setup>
import { sourcePath } from '@/utils/knowledge'
defineProps({ sources: { type: Array, default: () => [] }, publicLinks: { type: Boolean, default: true } })
</script>

<template>
  <section v-if="sources.length" aria-label="回答来源" class="knowledge-sources">
    <h3>来源</h3>
    <ul>
      <li v-for="source in sources" :key="`${source.article_id}:${source.version_no}:${source.chunk_index}`">
        <router-link v-if="publicLinks && sourcePath(source)" :to="sourcePath(source)">{{ source.title }}</router-link>
        <span v-else>{{ source.title }}</span>
        <span> · #{{ source.article_id }} / V{{ source.version_no }} / 分块 {{ source.chunk_index }}</span>
        <p v-if="source.heading_path?.length">{{ source.heading_path.map(h => h.text).join(' / ') }}</p>
      </li>
    </ul>
    <p v-if="publicLinks" class="hint">链接打开文章当前公开版本；来源版本号代表本次回答使用的快照。</p>
  </section>
</template>

<style scoped>
ul { padding-left: 20px; } li { margin: 12px 0; line-height: 1.6; } p { margin: 4px 0; }
.hint { color: var(--text-muted); font-size: 12px; }
</style>
