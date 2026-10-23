<template>
  <span class="file-icon" :style="{ background: meta.bg, color: meta.color }">
    <el-icon :size="size"><component :is="meta.icon" /></el-icon>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { fileExt } from '../utils/format'

const props = defineProps({
  type: { type: String, default: 'file' },
  name: { type: String, default: '' },
  size: { type: Number, default: 20 },
  isDir: { type: Boolean, default: false }
})

const map = {
  folder: { icon: 'Folder', color: '#fbbf24', bg: 'rgba(251,191,36,.14)' },
  image: { icon: 'Picture', color: '#2dd4bf', bg: 'rgba(45,212,191,.14)' },
  video: { icon: 'VideoCamera', color: '#60a5fa', bg: 'rgba(96,165,250,.14)' },
  audio: { icon: 'Headset', color: '#c084fc', bg: 'rgba(192,132,252,.14)' },
  archive: { icon: 'Box', color: '#fb923c', bg: 'rgba(251,146,60,.14)' },
  document: { icon: 'Document', color: '#94a3b8', bg: 'rgba(148,163,184,.14)' },
  file: { icon: 'Document', color: '#94a3b8', bg: 'rgba(148,163,184,.14)' }
}

const meta = computed(() => {
  let key = props.isDir ? 'folder' : props.type
  if (!key || key === 'file') {
    const ext = fileExt(props.name)
    if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'heic', 'bmp'].includes(ext)) key = 'image'
    else if (['mp4', 'mkv', 'mov', 'avi', 'webm'].includes(ext)) key = 'video'
    else if (['mp3', 'flac', 'wav', 'm4a', 'aac'].includes(ext)) key = 'audio'
    else if (['zip', 'gz', 'tar', '7z', 'rar', 'xz'].includes(ext)) key = 'archive'
  }
  return map[key] || map.file
})
</script>

<style scoped>
.file-icon {
  width: 40px;
  height: 40px;
  border-radius: 11px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
</style>
