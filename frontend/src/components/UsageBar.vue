<template>
  <div class="usage">
    <div class="usage-head">
      <span>{{ label }}</span>
      <span class="usage-text">
        {{ formatBytes(used) }} / {{ quota > 0 ? formatBytes(quota) : t('common.unlimited') }}
      </span>
    </div>
    <div class="bar">
      <div class="fill" :class="{ danger: percent >= 90 }" :style="{ width: `${percent}%` }" />
    </div>
    <div class="usage-foot">
      <span>{{ t('common.used') }} {{ percent.toFixed(1) }}%</span>
      <span v-if="quota > 0">{{ t('dashboard.free') }} {{ formatBytes(Math.max(0, quota - used)) }}</span>
      <span v-else>{{ t('common.unlimited') }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatBytes } from '../utils/format'

const props = defineProps({
  used: { type: Number, default: 0 },
  quota: { type: Number, default: 0 },
  label: { type: String, default: 'Storage' }
})
const { t } = useI18n()
const percent = computed(() => {
  if (props.quota > 0) return Math.min(100, (props.used / props.quota) * 100)
  return 0
})
</script>

<style scoped>
.usage { display: flex; flex-direction: column; gap: 8px; }
.usage-head, .usage-foot {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--pos-muted);
}
.usage-text { font-variant-numeric: tabular-nums; }
.bar { height: 8px; border-radius: 5px; background: rgba(139,161,180,.18); overflow: hidden; }
.fill { height: 100%; border-radius: 5px; background: var(--pos-accent); transition: width .5s ease; }
.fill.danger { background: var(--pos-danger); }
</style>
