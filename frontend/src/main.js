import 'element-plus/theme-chalk/base.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import 'element-plus/theme-chalk/el-button.css'
import 'element-plus/theme-chalk/el-card.css'
import 'element-plus/theme-chalk/el-icon.css'
import 'element-plus/theme-chalk/el-message-box.css'
import 'element-plus/theme-chalk/el-overlay.css'
import 'element-plus/theme-chalk/el-popper.css'
import 'element-plus/theme-chalk/el-progress.css'
import 'element-plus/theme-chalk/el-tag.css'
import 'element-plus/theme-chalk/el-tooltip.css'
import './assets/main.css'

import { createApp } from 'vue'
import { ElButton, ElCard, ElProgress, ElTag, ElTooltip } from 'element-plus'
import App from './App.vue'

const app = createApp(App)

for (const component of [ElButton, ElCard, ElProgress, ElTag, ElTooltip]) {
  app.use(component)
}

app.mount('#app')
