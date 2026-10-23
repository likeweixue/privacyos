<template>
  <div
    class="os-window"
    :class="{ active: isActive, maximized: win.maximized, minimized: win.minimized }"
    :style="windowStyle"
    @pointerdown.capture="activate"
  >
    <header class="win-titlebar" @pointerdown="startDrag" @dblclick="onTitleDoubleClick">
      <span class="win-app-icon" :style="{ color: meta.color }">
        <el-icon :size="14"><component :is="meta.icon" /></el-icon>
      </span>
      <span class="win-title">{{ label }}</span>
      <span class="win-group">{{ t(groupLabel(meta.group)) }}</span>
      <div class="spacer" />
      <div class="win-controls" @pointerdown.stop>
        <button type="button" class="win-btn" :title="t('desktop.minimize')" @click="store.minimize(win.id)">
          <span class="glyph minimize" />
        </button>
        <button type="button" class="win-btn" :title="win.maximized ? t('desktop.restore') : t('desktop.maximize')" @click="store.toggleMaximize(win.id)">
          <span class="glyph" :class="win.maximized ? 'restore' : 'maximize'" />
        </button>
        <button type="button" class="win-btn close" :title="t('desktop.close')" @click="$emit('close', win)">
          <span class="glyph close" />
        </button>
      </div>
    </header>

    <div class="window-body" :class="{ fill: meta.key === 'files' }">
      <component :is="meta.component" />
    </div>

    <div v-if="!win.maximized" class="resize-handle" @pointerdown.stop.prevent="startResize" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useDesktopStore } from '../stores/desktop'
import { apps } from '../apps'

const props = defineProps({ win: { type: Object, required: true } })
defineEmits(['close'])
const { t } = useI18n()
const store = useDesktopStore()
const router = useRouter()
const meta = computed(() => apps[props.win.app] || {})
const label = computed(() => {
  const base = t(meta.value.titleKey)
  return store.instanceCount(props.win.app) > 1
    ? `${base} · ${store.instanceNo(props.win)}`
    : base
})
const isActive = computed(() => store.activeId === props.win.id)
const windowStyle = computed(() => props.win.maximized
  ? { left: '0px', top: '0px', width: '100%', height: '100%', zIndex: props.win.z }
  : { left: `${props.win.x}px`, top: `${props.win.y}px`, width: `${props.win.w}px`, height: `${props.win.h}px`, zIndex: props.win.z })

function groupLabel(group) {
  return { core: 'nav.groups.core', features: 'nav.groups.features', system: 'nav.groups.system' }[group] || 'nav.admin'
}
function activate() {
  if (store.activeId !== props.win.id) {
    store.focus(props.win.id)
    if (router.currentRoute.value.name !== props.win.app) {
      router.replace({ name: props.win.app }).catch(() => {})
    }
  }
}
function onTitleDoubleClick() {
  store.toggleMaximize(props.win.id)
}
function startDrag(event) {
  if (event.button !== 0 || props.win.maximized) return
  if (event.target.closest('.win-controls')) return
  const startX = event.clientX
  const startY = event.clientY
  const originX = props.win.x
  const originY = props.win.y
  const onMove = (e) => store.move(props.win.id, originX + e.clientX - startX, originY + e.clientY - startY)
  const onUp = () => {
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    document.body.classList.remove('os-dragging')
  }
  document.body.classList.add('os-dragging')
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}
function startResize(event) {
  if (props.win.maximized) return
  const startX = event.clientX
  const startY = event.clientY
  const originW = props.win.w
  const originH = props.win.h
  const onMove = (e) => store.resize(props.win.id, originW + e.clientX - startX, originH + e.clientY - startY)
  const onUp = () => {
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    document.body.classList.remove('os-resizing')
  }
  document.body.classList.add('os-resizing')
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}
</script>

<style scoped>
.os-window {
  position: absolute;
  display: flex;
  flex-direction: column;
  min-width: 440px;
  min-height: 300px;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border-strong);
  border-radius: 3px;
  box-shadow: 0 14px 40px rgba(20, 32, 44, 0.16), 0 2px 6px rgba(20, 32, 44, 0.08);
  overflow: hidden;
  pointer-events: auto;
  animation: win-in 0.12s ease-out;
}
.os-window.active {
  border-color: color-mix(in srgb, var(--pos-accent) 55%, var(--pos-border-strong));
  box-shadow: 0 18px 50px rgba(20, 32, 44, 0.2), 0 2px 8px rgba(20, 32, 44, 0.1);
}
.os-window.minimized { display: none; }
.os-window.maximized { border-radius: 0; border-color: var(--pos-border); }
@keyframes win-in {
  from { opacity: 0; transform: translateY(4px) scale(0.995); }
  to { opacity: 1; transform: none; }
}
.win-titlebar {
  height: 38px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px 0 10px;
  background: var(--pos-panel);
  border-bottom: 1px solid var(--pos-border);
  user-select: none;
}
.os-window.active .win-titlebar { background: color-mix(in srgb, var(--pos-accent) 4%, var(--pos-panel)); }
.win-app-icon {
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--pos-border);
  border-radius: 2px;
  background: var(--pos-panel-2);
}
.win-title { font-size: 12.5px; font-weight: 600; white-space: nowrap; }
.win-group {
  font-size: 10px;
  color: var(--pos-muted);
  white-space: nowrap;
  border-left: 1px solid var(--pos-border);
  padding-left: 8px;
}
.spacer { flex: 1; }
.win-controls { display: flex; align-items: center; height: 100%; }
.win-btn {
  width: 38px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--pos-muted);
  cursor: pointer;
  border-radius: 2px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.win-btn:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.win-btn.close:hover { background: var(--pos-danger); color: #fff; }
.glyph { display: block; position: relative; width: 10px; height: 10px; }
.glyph.minimize::before { content: ''; position: absolute; left: 0; right: 0; bottom: 2px; height: 1.5px; background: currentColor; }
.glyph.maximize::before { content: ''; position: absolute; inset: 1px; border: 1.4px solid currentColor; border-radius: 1px; }
.glyph.restore::before, .glyph.restore::after { content: ''; position: absolute; border: 1.4px solid currentColor; border-radius: 1px; background: var(--pos-panel); }
.glyph.restore::before { left: 3px; top: 0; width: 6px; height: 6px; }
.glyph.restore::after { left: 0; top: 3px; width: 6px; height: 6px; }
.glyph.close::before, .glyph.close::after { content: ''; position: absolute; left: 0; top: 4.5px; width: 10px; height: 1.5px; background: currentColor; }
.glyph.close::before { transform: rotate(45deg); }
.glyph.close::after { transform: rotate(-45deg); }
.window-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 14px 16px 16px;
  user-select: text;
  display: flex;
  flex-direction: column;
}
.window-body.fill { overflow: hidden; padding: 10px 12px 12px; }
.window-body :deep(.files-page) { height: 100% !important; min-height: 0 !important; }
.window-body :deep(.page-header) { margin-bottom: 2px; }
.window-body :deep(.page-header h1) { display: none; }
.window-body :deep(.page-header p) { margin: 0; font-size: 12px; }
.window-body :deep(.AdminHeader h2) { display: none; }
.window-body :deep(.AdminHeader p) { margin: 0; font-size: 12px; }
.window-body :deep(.AdminHeader) { margin: 0; align-items: center; }
.window-body :deep(.settings-layout .side-nav) { top: 0; }
.window-body :deep(.pos-page) { gap: 12px; }
.window-body :deep(.AdminPage) { gap: 12px; }
.resize-handle { position: absolute; right: 0; bottom: 0; width: 15px; height: 15px; cursor: nwse-resize; z-index: 6; }
.resize-handle::after {
  content: '';
  position: absolute;
  right: 3px;
  bottom: 3px;
  width: 7px;
  height: 7px;
  border-right: 2px solid var(--pos-muted);
  border-bottom: 2px solid var(--pos-muted);
  opacity: 0.55;
}
body.os-dragging, body.os-resizing { user-select: none; }
@media (max-width: 900px) {
  .os-window { min-width: 0; min-height: 0; border-radius: 0; }
  .win-titlebar { height: 42px; }
  .win-group { display: none; }
  .window-body { padding: 12px; }
  .window-body.fill { padding: 8px 10px 10px; }
}
</style>
