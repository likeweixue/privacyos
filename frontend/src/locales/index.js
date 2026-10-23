import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import enUS from './en-US'

const saved = localStorage.getItem('pos_lang') || 'zh-CN'

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: saved === 'en-US' ? 'en-US' : 'zh-CN',
  fallbackLocale: 'zh-CN',
  missingWarn: false,
  fallbackWarn: false,
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS
  }
})

export default i18n
