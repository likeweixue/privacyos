<template>
  <div class="file-preview">
    <div class="preview-head">
      <FileIcon :type="entry.type" :name="entry.name" :is-dir="entry.is_dir" :size="30" />
      <div class="preview-title">
        <strong>{{ entry.name }}</strong>
        <span>{{ entry.is_dir ? t('files.folder') : (entry.mime || entry.type) }} · {{ entry.is_dir ? '—' : formatBytes(entry.size) }}</span>
      </div>
    </div>

    <div v-if="entry.is_dir" class="preview-folder">
      <el-icon :size="34"><FolderOpened /></el-icon>
      <p>{{ t('files.folderPreviewHint') }}</p>
    </div>
    <div v-else class="preview-media">
      <img v-if="isImage" :src="url" :alt="entry.name" />
      <video v-else-if="isVideo" :src="url" controls preload="metadata" />
      <audio v-else-if="isAudio" :src="url" controls />
      <iframe v-else-if="isPdf" :src="url" :title="entry.name" />
      <pre v-else-if="isText" class="preview-text">{{ text || t('common.loading') }}</pre>
      <div v-else class="preview-unsupported">
        <el-icon :size="32"><Document /></el-icon>
        <p>{{ t('files.noPreview') }}</p>
      </div>
    </div>

    <div class="preview-meta">
      <div><span>{{ t('files.modified') }}</span><strong>{{ formatDate(entry.modified) }}</strong></div>
      <div><span>{{ t('files.owner') }}</span><strong>{{ entry.owner || '—' }}</strong></div>
      <div v-if="entry.mode"><span>{{ t('files.permissions') }}</span><strong class="pos-mono">{{ entry.mode }}</strong></div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Document, FolderOpened } from '@element-plus/icons-vue'
import FileIcon from './FileIcon.vue'
import { apiURL } from '../api'
import { formatBytes, formatDate, fileExt } from '../utils/format'

const props = defineProps({ entry: { type: Object, required: true } })
const { t } = useI18n()
const text = ref('')

const ext = computed(() => fileExt(props.entry.name || '').toLowerCase())
const isImage = computed(() => ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp', 'svg', 'heic'].includes(ext.value) || props.entry.type === 'image')
const isVideo = computed(() => ['mp4', 'webm', 'mkv', 'mov', 'm4v'].includes(ext.value) || props.entry.type === 'video')
const isAudio = computed(() => ['mp3', 'flac', 'wav', 'aac', 'ogg', 'm4a'].includes(ext.value) || props.entry.type === 'audio')
const isPdf = computed(() => ext.value === 'pdf')
const isText = computed(() => ['txt', 'md', 'log', 'json', 'yaml', 'yml', 'toml', 'ini', 'conf', 'csv', 'js', 'ts', 'css', 'html', 'xml', 'sh', 'py', 'go', 'rs'].includes(ext.value))
const url = computed(() => apiURL(`/files/download?${new URLSearchParams({ path: props.entry.path })}`))

async function loadText() {
  text.value = ''
  if (!isText.value) return
  try {
    const res = await fetch(url.value)
    const body = await res.text()
    text.value = body.length > 200000 ? body.slice(0, 200000) + '\\n…' : body
  } catch (error) {
    text.value = t('files.noPreview')
  }
}

watch(() => props.entry?.path, loadText, { immediate: true })
</script>

<style scoped>
.file-preview { display: flex; flex-direction: column; gap: 10px; min-width: 0; }
.preview-head { display: flex; align-items: center; gap: 9px; }
.preview-title { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.preview-title strong { font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.preview-title span { font-size: 10px; color: var(--pos-muted); }
.preview-media { border: 1px solid var(--pos-border); background: var(--pos-panel-2); display: flex; align-items: center; justify-content: center; min-height: 140px; overflow: auto; }
.preview-media img { max-width: 100%; max-height: 320px; object-fit: contain; border-radius: 0; }
.preview-media video { width: 100%; max-height: 320px; }
.preview-media audio { width: 100%; padding: 20px; }
.preview-media iframe { width: 100%; height: 360px; border: none; }
.preview-text { margin: 0; padding: 10px; width: 100%; max-height: 360px; overflow: auto; font-size: 11px; line-height: 1.55; white-space: pre-wrap; word-break: break-word; }
.preview-unsupported, .preview-folder { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 30px 12px; color: var(--pos-muted); text-align: center; }
.preview-folder p, .preview-unsupported p { margin: 0; font-size: 11px; }
.preview-meta { display: flex; flex-direction: column; gap: 1px; border-top: 1px solid var(--pos-border); }
.preview-meta div { display: flex; justify-content: space-between; gap: 8px; padding: 5px 0; font-size: 10.5px; border-bottom: 1px solid var(--pos-border); }
.preview-meta span { color: var(--pos-muted); }
.preview-meta strong { font-weight: 500; text-align: right; overflow: hidden; text-overflow: ellipsis; }
</style>
