<template>
  <div class="page-header">
    <div class="titles">
      <h1>{{ title }}</h1>
      <p v-if="subtitle">{{ subtitle }}</p>
    </div>
    <div class="actions">
      <slot />
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' }
})
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}
h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 650;
  letter-spacing: 0.3px;
}
p {
  margin: 6px 0 0;
  color: var(--pos-muted);
  font-size: 13px;
}
.actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
</style>
