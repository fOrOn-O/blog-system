<script setup>
defineProps({ diff: { type: Object, required: true } })
</script>

<template>
  <section class="article-diff" aria-label="正文差异">
    <template v-for="(field, name) in diff.field_changes" :key="name">
      <div v-if="field.changed" class="diff-row">
        <strong>{{ name }}</strong><del>{{ field.before }}</del><ins>{{ field.after }}</ins>
      </div>
    </template>
    <p v-if="!diff.content.changed">没有结构化正文差异。格式或 HTML 属性变化仍可能生成新版本。</p>
    <div v-for="(change, index) in diff.content.changes" :key="index" class="diff-row">
      <span class="diff-label">{{ { modify: '修改', insert: '新增', delete: '删除' }[change.operation] }}</span>
      <del v-if="change.before">{{ change.before.text }}</del>
      <ins v-if="change.after">{{ change.after.text }}</ins>
    </div>
  </section>
</template>

<style scoped>
.article-diff { margin: 12px 0; max-height: 360px; overflow: auto; font-size: 14px; }
.diff-row { display: grid; gap: 6px; margin-bottom: 12px; }
.diff-label { color: var(--text-muted); font-size: 12px; }
del, ins { display: block; padding: 8px; white-space: pre-wrap; overflow-wrap: anywhere; text-decoration: none; border-radius: 4px; }
del { background: #fff0f0; color: #8e3030; }
ins { background: #edf8f0; color: #25633c; }
</style>
