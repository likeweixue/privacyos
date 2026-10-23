<template>
  <div
    class="os-window"
    :class="{ active: isActive, maximized: win.maximized, minimized: win.minimized }"
    :style="windowStyle"
    @pointerdown.capture="activate"
  >
    <header class="titlebar" @pointerdown="startDrag" @dblclick="onTitleDoubleClick">
      <span class="app-icon" :style="{ background: `${meta.color}22`, color: meta.color }">
        <el-icon :size="15"><component :is="meta.icon" /></el-icon>
      </span>
      <span class="title">{{ t(meta.titleKey) }}</span>
      <span class="title-sub">{{ t('app.name') }} {{ auth.editionName }}</span>
      <div class="spacer" />
      <div class="win-controls" @pointerdown.stop>
        <button type="button" class="win-btn" :title="t('desktop.minimize')" @click="store.minimize(win.id)">
          <span class="glyph minimize" />
        </button>
        <button
          type="button"
          class="win-btn"
          :title="win.maximized ? t('desktop.restore') : t('desktop.maximize')"
          @click="store.toggleMaximize(win.id)"
        >
          <span class="glyph" :class="win.maximized ? 'restore' : 'maximize'" />
        </button>
        <button type="button" class="win-btn close" :title="t('desktop.close')" @click="$emit('close', win)">
          <span class="glyph close" />
        </button>
      </div>
    </header>

    <div class="window-content" :class="{ fill: meta.key === 'files' }">
      <component :is="meta.component" />
    </div>

    <div v-if="!win.maximized" class="resize-handle" @pointerdown.stop.prevent="startResize" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useDesktopStore } from '../stores/desktop'
import { useAuthStore } from '../stores/auth'
import { apps } from '../desktop/apps'

const props = defineProps({
  win: { type: Object, required: true }
})
defineEmits(['close'])

const { t } = useI18n()
const store = useDesktopStore()
const auth = useAuthStore()
const router = useRouter()

const meta = computed(() => apps[props.win.app] || {})
const isActive = computed(() => store.activeId === props.win.id)

const windowStyle = computed(() => {
  if (props.win.maximized) {
    return { left: '0px', top: '0px', width: '100%', height: '100%', zIndex: props.win.z }
  }
  return {
    left: `${props.win.x}px`,
    top: `${props.win.y}px`,
    width: `${props.win.w}px`,
    height: `${props.win.h}px`,
    zIndex: props.win.z
  }
})

function activate() {
  if (store.activeId !== props.win.id) {
    store.focus(props.win.id)
    if (meta.value.routeName && router.currentRoute.value.name !== meta.value.routeName) {
      router.replace({ name: meta.value.routeName }).catch(() => {})
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
  const onMove = (e) => {
    store.move(props.win.id, originX + e.clientX - startX, originY + e.clientY - startY)
  }
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
  const onMove = (e) => {
    store.resize(props.win.id, originW + e.clientX - startX, originH + e.clientY - startY)
  }
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
  min-width: 460px;
  min-height: 320px;
  border-radius: 12px;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  box-shadow: 0 28px 80px rgba(0, 0, 0, 0.55), 0 2px 10px rgba(0, 0, 0, 0.35);
  overflow: hidden;
  pointer-events: auto;
  animation: window-in 0.16s ease-out;
  transition: box-shadow 0.15s ease, border-color 0.15s ease;
}
.os-window.active {
  border-color: rgba(45, 212, 191, 0.4);
  box-shadow: 0 34px 100px rgba(0, 0, 0, 0.62), 0 0 0 1px rgba(45, 212, 191, 0.18);
}
.os-window.minimized {
  display: none;
}
.os-window.maximized {
  border-radius: 0;
  border-color: transparent;
}
@keyframes window-in {
  from { opacity: 0; transform: scale(0.985) translateY(6px); }
  to { opacity: 1; transform: none; }
}
.titlebar {
  height: 40px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 0 6px 0 12px;
  background: color-mix(in srgb, var(--pos-panel) 80%, var(--pos-bg));
  border-bottom: 1px solid var(--pos-border);
  user-select: none;
  cursor: default;
}
.os-window.active .titlebar {
  background: color-mix(in srgb, var(--pos-panel) 62%, var(--pos-accent) 6%);
}
.app-icon {
  width: 23px;
  height: 23px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.title {
  font-size: 12.5px;
  font-weight: 600;
  letter-spacing: 0.2px;
  white-space: nowrap;
}
.title-sub {
  font-size: 10.5px;
  color: var(--pos-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.spacer { flex: 1; }
.win-controls {
  display: flex;
  align-items: center;
  height: 100%;
}
.win-btn {
  width: 42px;
  height: 30px;
  border: none;
  background: transparent;
  color: var(--pos-text);
  cursor: pointer;
  border-radius: 5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: background 0.12s ease, color 0.12s ease;
}
.win-btn:hover { background: rgba(139, 161, 180, 0.16); }
.win-btn.close:hover { background: #e5484d; color: #fff; }
.glyph {
  display: block;
  position: relative;
  width: 11px;
  height: 11px;
}
.glyph.minimize::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 2px;
  height: 1.5px;
  background: currentColor;
}
.glyph.maximize::before {
  content: '';
  position: absolute;
  inset: 1px;
  border: 1.4px solid currentColor;
  border-radius: 1px;
}
.glyph.restore::before,
.glyph.restore::after {
  content: '';
  position: absolute;
  border: 1.4px solid currentColor;
  border-radius: 1px;
  background: var(--pos-panel);
}
.glyph.restore::before { left: 3px; top: 0; width: 7px; height: 7px; }
.glyph.restore::after { left: 0; top: 3px; width: 7px; height: 7px; }
.glyph.close::before,
.glyph.close::after {
  content: '';
  position: absolute;
  left: 0;
  top: 5px;
  width: 11px;
  height: 1.5px;
  background: currentColor;
}
.glyph.close::before { transform: rotate(45deg); }
.glyph.close::after { transform: rotate(-45deg); }
.window-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
  user-select: text;
  padding: 16px 18px 18px;
  display: flex;
  flex-direction: column;
}
.window-content.fill {
  overflow: hidden;
  padding: 12px 14px;
}
.window-content :deep(.pos-page) {
  gap: 14px;
}
.window-content :deep(.page-header) {
  margin-bottom: 0;
  align-items: center;
}
.window-content :deep(.page-header h1) {
  display: none;
}
.window-content :deep(.page-header p) {
  margin: 0;
  font-size: 12px;
}
.window-content :deep(.page-header .actions) {
  margin-left: auto;
}
.window-content :deep(.settings-layout .side-nav) {
  top: 0;
}
.resize-handle {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 16px;
  height: 16px;
  cursor: nwse-resize;
  z-index: 5;
}
.resize-handle::after {
  content: '';
  position: absolute;
  right: 3px;
  bottom: 3px;
  width: 8px;
  height: 8px;
  border-right: 2px solid var(--pos-muted);
  border-bottom: 2px solid var(--pos-muted);
  opacity: 0.6;
}
@media (max-width: 900px) {
  .os-window { min-width: 0; border-radius: 0; }
  .titlebar { height: 44px; }
  .title-sub { display: none; }
  .window-content { padding: 12px; }
}
</style>
