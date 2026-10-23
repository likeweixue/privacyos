<template>
  <div class="empty-state">
    <div class="icon-wrap">
      <el-icon :size="30"><component :is="icon" /></el-icon>
    </div>
    <h3>{{ title }}</h3>
    <p v-if="description">{{ description }}</p>
    <div class="actions"><slot /></div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  description: { type: String, default: '' },
  icon: { type: String, default: 'Box' }
})
</script>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 64px 20px;
  text-align: center;
  color: var(--pos-muted);
}
.icon-wrap {
  width: 64px;
  height: 64px;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
  margin-bottom: 6px;
}
h3 {
  margin: 0;
  color: var(--pos-text);
  font-size: 16px;
  font-weight: 600;
}
p {
  margin: 0;
  max-width: 460px;
  font-size: 13px;
  line-height: 1.7;
}
.actions { margin-top: 10px; }
</style>
