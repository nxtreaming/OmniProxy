import 'element-plus/dist/index.css'
import './assets/main.css'

import { createApp } from 'vue'
import { ElButton, ElCard, ElProgress, ElTag, ElTooltip } from 'element-plus'
import App from './App.vue'

const app = createApp(App)

for (const component of [ElButton, ElCard, ElProgress, ElTag, ElTooltip]) {
  app.use(component)
}

app.mount('#app')
