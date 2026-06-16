import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import './styles/tailwind.css'
import './styles/element-override.css'

/** 创建Vue应用实例 */
const app = createApp(App)

/** 注册Pinia状态管理 */
app.use(createPinia())

/** 注册路由 */
app.use(router)

/** 注册Element Plus组件库 */
app.use(ElementPlus)

/** 挂载应用 */
app.mount('#app')
