import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import enUs from 'element-plus/es/locale/lang/en'
import * as ElementPlusIcons from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'

import App from './App.vue'
import router from './router'
import i18n from './locales'
import { useUiStore } from './stores/ui'
import './styles/index.scss'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(i18n)
app.use(ElementPlus, {
  locale: (localStorage.getItem('pos_lang') || 'zh-CN') === 'en-US' ? enUs : zhCn
})

for (const [name, component] of Object.entries(ElementPlusIcons)) {
  app.component(name, component)
}

const ui = useUiStore(pinia)
ui.applyTheme()
i18n.global.locale.value = ui.lang
document.documentElement.setAttribute('lang', ui.lang === 'en-US' ? 'en' : 'zh-CN')

app.mount('#app')
