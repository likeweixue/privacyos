<template>
  <el-config-provider :locale="epLocale">
    <router-view />
  </el-config-provider>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import enUs from 'element-plus/es/locale/lang/en'
import { useUiStore } from './stores/ui'
import { useAuthStore } from './stores/auth'
import { useI18n } from 'vue-i18n'

const ui = useUiStore()
const auth = useAuthStore()
const router = useRouter()
const { t } = useI18n()

const epLocale = computed(() => (ui.lang === 'en-US' ? enUs : zhCn))

onMounted(() => {
  ui.applyTheme()
  window.addEventListener('pos:logout', () => {
    auth.user = null
    if (router.currentRoute.value.name !== 'login') {
      ElMessage.warning(t('errors.unauthorized'))
      router.replace({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
    }
  })
})
</script>
