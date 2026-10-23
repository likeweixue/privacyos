<template>
  <div class="stat-card pos-panel">
    <div class="stat-head">
      <span class="label">{{ label }}</span>
      <span class="icon" :style="{ background: softColor, color }">
        <el-icon :size="18"><component :is="icon" /></el-icon>
      </span>
    </div>
    <div class="value-row">
      <span class="value">{{ value }}</span>
      <span v-if="suffix" class="suffix">{{ suffix }}</span>
    </div>
    <div v-if="percent !== null && percent !== undefined" class="meter">
      <div class="meter-fill" :style="{ width: `${Math.min(100, percent)}%`, background: color }" />
    </div>
    <div v-if="hint" class="hint">{{ hint }}</div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], required: true },
  suffix: { type: String, default: '' },
  icon: { type: String, default: 'DataLine' },
  color: { type: String, default: '#2dd4bf' },
  percent: { type: Number, default: null },
  hint: { type: String, default: '' }
})

const softColor = computed(() => `${props.color}22`)
</script>

<style scoped>
.stat-card {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 126px;
}
.stat-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.label {
  font-size: 13px;
  color: var(--pos-muted);
  letter-spacing: 0.3px;
}
.icon {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.value-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
}
.value {
  font-size: 28px;
  font-weight: 650;
  letter-spacing: -0.5px;
  font-variant-numeric: tabular-nums;
}
.suffix {
  color: var(--pos-muted);
  font-size: 13px;
}
.meter {
  height: 6px;
  border-radius: 4px;
  background: rgba(139, 161, 180, 0.18);
  overflow: hidden;
}
.meter-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.6s ease;
}
.hint {
  font-size: 12px;
  color: var(--pos-muted);
}
</style>
