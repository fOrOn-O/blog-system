<template>
  <div class="rich-text-editor" :class="{ 'is-focused': editor?.isFocused }">
    <div v-if="editor" class="editor-toolbar" role="toolbar" aria-label="文章正文格式工具栏">
      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-button toolbar-button--wide"
          :class="{ 'is-active': editor.isActive('paragraph') }"
          title="普通段落"
          @click="editor.chain().focus().setParagraph().run()"
        >
          正文
        </button>
        <button
          type="button"
          class="toolbar-button"
          :class="{ 'is-active': editor.isActive('heading', { level: 2 }) }"
          title="二级标题"
          @click="editor.chain().focus().toggleHeading({ level: 2 }).run()"
        >
          H2
        </button>
        <button
          type="button"
          class="toolbar-button"
          :class="{ 'is-active': editor.isActive('heading', { level: 3 }) }"
          title="三级标题"
          @click="editor.chain().focus().toggleHeading({ level: 3 }).run()"
        >
          H3
        </button>
      </div>

      <div class="toolbar-divider" aria-hidden="true"></div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-button toolbar-button--bold"
          :class="{ 'is-active': editor.isActive('bold') }"
          :disabled="!editor.can().chain().focus().toggleBold().run()"
          title="粗体"
          @click="editor.chain().focus().toggleBold().run()"
        >
          B
        </button>
        <button
          type="button"
          class="toolbar-button toolbar-button--italic"
          :class="{ 'is-active': editor.isActive('italic') }"
          :disabled="!editor.can().chain().focus().toggleItalic().run()"
          title="斜体"
          @click="editor.chain().focus().toggleItalic().run()"
        >
          I
        </button>
        <button
          type="button"
          class="toolbar-button toolbar-button--code"
          :class="{ 'is-active': editor.isActive('code') }"
          :disabled="!editor.can().chain().focus().toggleCode().run()"
          title="行内代码"
          @click="editor.chain().focus().toggleCode().run()"
        >
          &lt;/&gt;
        </button>
        <button
          type="button"
          class="toolbar-button"
          :class="{ 'is-active': editor.isActive('link') }"
          title="添加或编辑链接"
          @click="setLink"
        >
          链接
        </button>
      </div>

      <div class="toolbar-divider" aria-hidden="true"></div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-button"
          :class="{ 'is-active': editor.isActive('blockquote') }"
          title="引用"
          @click="editor.chain().focus().toggleBlockquote().run()"
        >
          引用
        </button>
        <button
          type="button"
          class="toolbar-button"
          :class="{ 'is-active': editor.isActive('bulletList') }"
          title="无序列表"
          @click="editor.chain().focus().toggleBulletList().run()"
        >
          • 列表
        </button>
        <button
          type="button"
          class="toolbar-button"
          :class="{ 'is-active': editor.isActive('orderedList') }"
          title="有序列表"
          @click="editor.chain().focus().toggleOrderedList().run()"
        >
          1. 列表
        </button>
        <button
          type="button"
          class="toolbar-button toolbar-button--code"
          :class="{ 'is-active': editor.isActive('codeBlock') }"
          title="代码块"
          @click="editor.chain().focus().toggleCodeBlock().run()"
        >
          { }
        </button>
      </div>

      <div class="toolbar-divider" aria-hidden="true"></div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-button toolbar-button--wide"
          :disabled="uploading"
          title="上传并插入图片"
          @click="selectImage"
        >
          {{ uploading ? '上传中…' : '插入图片' }}
        </button>
        <input
          ref="imageInput"
          class="visually-hidden"
          type="file"
          accept="image/jpeg,image/png,image/gif,image/webp"
          tabindex="-1"
          @change="handleImageUpload"
        />
      </div>

      <div class="toolbar-spacer"></div>

      <div class="toolbar-group">
        <button
          type="button"
          class="toolbar-button"
          :disabled="!editor.can().chain().focus().undo().run()"
          title="撤销"
          @click="editor.chain().focus().undo().run()"
        >
          ↶
        </button>
        <button
          type="button"
          class="toolbar-button"
          :disabled="!editor.can().chain().focus().redo().run()"
          title="重做"
          @click="editor.chain().focus().redo().run()"
        >
          ↷
        </button>
      </div>
    </div>

    <EditorContent :editor="editor" class="editor-surface" />
    <div class="editor-footer">正文将以结构化 HTML 保存</div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import { uploadImage } from '@/api/upload'

const props = defineProps({
  modelValue: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:modelValue'])

const imageInput = ref(null)
const uploading = ref(false)
const imageInsertPosition = ref(null)

const editor = useEditor({
  content: props.modelValue || '',
  extensions: [
    StarterKit.configure({
      heading: {
        levels: [2, 3]
      },
      link: {
        openOnClick: false,
        autolink: true,
        linkOnPaste: true,
        defaultProtocol: 'https',
        HTMLAttributes: {
          target: '_blank',
          rel: 'noopener noreferrer'
        }
      }
    }),
    Image.configure({
      inline: false,
      allowBase64: false
    })
  ],
  editorProps: {
    attributes: {
      class: 'rich-text-content',
      'aria-label': '文章正文编辑器'
    }
  },
  onUpdate: ({ editor: currentEditor }) => {
    emit('update:modelValue', currentEditor.getHTML())
  }
})

watch(
  () => props.modelValue,
  (value) => {
    if (!editor.value) return

    const nextContent = value || ''
    if (editor.value.getHTML() !== nextContent) {
      editor.value.commands.setContent(nextContent, { emitUpdate: false })
    }
  }
)

const setLink = () => {
  if (!editor.value) return

  const previousUrl = editor.value.getAttributes('link').href || 'https://'
  const input = window.prompt('请输入链接地址；留空可移除当前链接', previousUrl)
  if (input === null) return

  const value = input.trim()
  if (!value) {
    editor.value.chain().focus().extendMarkRange('link').unsetLink().run()
    return
  }

  const url = /^(https?:\/\/|mailto:|tel:)/i.test(value) ? value : `https://${value}`
  editor.value.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
}

const selectImage = () => {
  if (!editor.value || uploading.value) return

  imageInsertPosition.value = editor.value.state.selection.from
  imageInput.value?.click()
}

const handleImageUpload = async (event) => {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file || !editor.value) return

  const allowedTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']
  if (!allowedTypes.includes(file.type)) {
    ElMessage.error('只支持 JPG、PNG、GIF、WebP 格式')
    return
  }

  if (file.size > 10 * 1024 * 1024) {
    ElMessage.error('图片大小不能超过 10MB')
    return
  }

  uploading.value = true
  try {
    const response = await uploadImage(file)
    const data = response.data || response
    if (!data?.url) {
      throw new Error('上传响应中缺少图片地址')
    }

    const maxPosition = editor.value.state.doc.content.size
    const position = Math.min(imageInsertPosition.value ?? maxPosition, maxPosition)
    editor.value
      .chain()
      .focus()
      .setTextSelection(position)
      .setImage({ src: data.url, alt: file.name })
      .run()

    ElMessage.success('图片已插入正文')
  } catch (error) {
    console.error('正文图片上传失败:', error)
    if (!error?.response) {
      ElMessage.error(error.message || '正文图片上传失败')
    }
  } finally {
    uploading.value = false
    imageInsertPosition.value = null
  }
}
</script>

<style lang="scss" scoped>
.rich-text-editor {
  width: 100%;
  overflow: hidden;
  border: 1px solid #dfe3e8;
  border-radius: 8px;
  background: #fff;
  transition: border-color 0.2s, box-shadow 0.2s;

  &.is-focused {
    border-color: #409eff;
    box-shadow: 0 0 0 2px rgb(64 158 255 / 12%);
  }
}

.editor-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-height: 48px;
  padding: 8px 10px;
  border-bottom: 1px solid #e8ebef;
  background: #fafbfc;
}

.toolbar-group {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.toolbar-divider {
  align-self: stretch;
  width: 1px;
  min-height: 24px;
  margin: 2px 3px;
  background: #dfe3e8;
}

.toolbar-spacer {
  flex: 1 1 auto;
}

.toolbar-button {
  min-width: 32px;
  height: 32px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 5px;
  background: transparent;
  color: #4b5563;
  font: inherit;
  font-size: 13px;
  line-height: 30px;
  cursor: pointer;
  transition: color 0.15s, background-color 0.15s, border-color 0.15s;

  &:hover:not(:disabled) {
    border-color: #d6e8ff;
    background: #edf5ff;
    color: #1677d2;
  }

  &.is-active {
    border-color: #b8d9ff;
    background: #e6f2ff;
    color: #0969c3;
  }

  &:disabled {
    color: #b5bcc5;
    cursor: not-allowed;
  }
}

.toolbar-button--wide {
  min-width: 48px;
}

.toolbar-button--bold {
  font-weight: 700;
}

.toolbar-button--italic {
  font-style: italic;
}

.toolbar-button--code {
  font-family: Consolas, Monaco, monospace;
}

.editor-surface {
  background: #fff;
}

.editor-surface :deep(.rich-text-content) {
  min-height: 380px;
  padding: 26px 30px 40px;
  color: #263238;
  font-size: 16px;
  line-height: 1.85;
  outline: none;

  > *:first-child {
    margin-top: 0;
  }

  > *:last-child {
    margin-bottom: 0;
  }

  p {
    margin: 0 0 1.15em;
  }

  h2,
  h3 {
    color: #17212b;
    font-weight: 650;
    line-height: 1.35;
  }

  h2 {
    margin: 1.9em 0 0.75em;
    font-size: 1.6em;
  }

  h3 {
    margin: 1.6em 0 0.65em;
    font-size: 1.3em;
  }

  ul,
  ol {
    margin: 0 0 1.2em;
    padding-left: 1.7em;
  }

  li {
    margin: 0.35em 0;
  }

  blockquote {
    margin: 1.4em 0;
    padding: 0.7em 1.1em;
    border-left: 4px solid #84bdf3;
    background: #f5f9fd;
    color: #56616d;
  }

  blockquote p:last-child {
    margin-bottom: 0;
  }

  code {
    padding: 0.15em 0.38em;
    border-radius: 4px;
    background: #f0f2f4;
    color: #b42318;
    font-family: Consolas, Monaco, 'Courier New', monospace;
    font-size: 0.9em;
  }

  pre {
    margin: 1.4em 0;
    overflow-x: auto;
    border-radius: 7px;
    background: #17212b;
    color: #e6edf3;
    padding: 18px 20px;
    line-height: 1.65;
  }

  pre code {
    padding: 0;
    background: transparent;
    color: inherit;
    font-size: 0.9em;
  }

  a {
    color: #0969c3;
    text-decoration: underline;
    text-underline-offset: 3px;
  }

  img {
    display: block;
    max-width: 100%;
    height: auto;
    margin: 1.5em auto;
    border-radius: 7px;
  }
}

.editor-footer {
  padding: 7px 12px;
  border-top: 1px solid #eef0f2;
  background: #fafbfc;
  color: #98a1ab;
  font-size: 12px;
  text-align: right;
}

.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  white-space: nowrap;
}

@media (max-width: 768px) {
  .toolbar-spacer {
    display: none;
  }

  .editor-surface :deep(.rich-text-content) {
    min-height: 320px;
    padding: 20px 16px 32px;
  }
}
</style>
