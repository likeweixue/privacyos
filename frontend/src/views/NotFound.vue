<template>
  <div class="nf-page">
    <div class="orb orb-a" />
    <div class="orb orb-b" />

    <main class="nf-inner">
      <div class="nf-brand">
        <img src="/logo.svg" :alt="t('app.name')" />
        <span>{{ t('app.name') }}</span>
      </div>
      <div class="nf-code" aria-hidden="true">404</div>
      <h1>{{ t('errors.notFound') }}</h1>
      <p class="pos-hint">{{ t('app.tagline') }}</p>
      <div class="nf-actions">
        <el-button type="primary" size="large" :icon="HomeFilled" @click="go('/')">{{ t('common.back') }}</el-button>
        <el-button size="large" :icon="FolderOpened" @click="go('/files')">{{ t('nav.files') }}</el-button>
      </div>
    </main>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { HomeFilled, FolderOpened } from '@element-plus/icons-vue'

const { t } = useI18n()
const router = useRouter()

function go(path) {
  router.push(path)
}

onMounted(() => {
  document.title = `${t('errors.notFound')} · ${t('app.name')}`
})
</script>

<style scoped>
.nf-page {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  overflow: hidden;
  background:
    radial-gradient(820px 500px at 50% -10%, rgba(45, 212, 191, 0.13), transparent 62%),
    radial-gradient(680px 460px at 100% 100%, rgba(96, 165, 250, 0.09), transparent 60%),
    var(--pos-bg);
}
.orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.45;
  pointer-events: none;
}
.orb-a {
  width: 380px;
  height: 380px;
  background: rgba(45, 212, 191, 0.15);
  top: -150px;
  right: -90px;
}
.orb-b {
  width: 320px;
  height: 320px;
  background: rgba(96, 165, 250, 0.11);
  bottom: -150px;
  left: -100px;
}
.nf-inner {
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  max-width: 480px;
  text-align: center;
}
.nf-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
  color: var(--pos-text);
  font-size: 13.5px;
  letter-spacing: 0.8px;
}
.nf-brand img {
  width: 32px;
  height: 32px;
}
.nf-code {
  font-size: clamp(100px, 22vw, 148px);
  line-height: 0.95;
  font-weight: 800;
  letter-spacing: 4px;
  user-select: none;
  background: linear-gradient(150deg, var(--pos-accent) 10%, var(--pos-blue) 90%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.nf-inner h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 650;
}
.nf-inner p {
  margin: 0;
  max-width: 380px;
}
.nf-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  justify-content: center;
  margin-top: 14px;
}
.nf-actions :deep(.el-button) {
  margin-left: 0;
  min-width: 132px;
}

@media (max-width: 520px) {
  .nf-actions {
    flex-direction: column;
    width: 100%;
  }
  .nf-actions :deep(.el-button) {
    width: 100%;
  }
}
</style>
