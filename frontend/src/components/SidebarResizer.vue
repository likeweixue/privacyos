<template>
  <div
    class="sidebar-resizer"
    :class="side"
    role="separator"
    aria-orientation="vertical"
    title="拖动调整宽度"
    @pointerdown.prevent="start"
  ><span /></div>
</template>

<script setup>
const props = defineProps({
  width: { type: Number, required: true },
  side: { type: String, default: 'left' }
})
const emit = defineEmits(['resize'])
function start(event) {
  const startX = event.clientX
  const startW = props.width
  const onMove = (e) => {
    const delta = e.clientX - startX
    emit('resize', props.side === 'left' ? startW + delta : startW - delta)
  }
  const onUp = () => {
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    document.body.classList.remove('sidebar-resizing')
  }
  document.body.classList.add('sidebar-resizing')
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}
</script>

<style scoped>
.sidebar-resizer {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 7px;
  cursor: col-resize;
  z-index: 5;
  background: transparent;
}
.sidebar-resizer.left { right: -4px; }
.sidebar-resizer.right { left: -4px; }
.sidebar-resizer span {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 3px;
  width: 1px;
  background: transparent;
  transition: background 0.12s ease;
}
.sidebar-resizer:hover span { background: var(--pos-accent); }
body.sidebar-resizing { user-select: none; cursor: col-resize; }
</style>
