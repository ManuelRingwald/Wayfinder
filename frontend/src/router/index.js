import { createRouter, createWebHistory } from 'vue-router'
import AsdView from '@/views/AsdView.vue'

// WF2-32: clean history-mode routing (no hash). The backend serves the SPA shell
// for unknown paths (webui.Handler fallback), so deep links like /admin survive a
// hard reload. The ASD scope and the admin dashboard are distinct route
// components: navigating to /admin unmounts AsdView (and with it the MapLibre
// canvas + WebSocket), freeing GPU/CPU — a deliberate full component swap, not an
// overlay. AdminView is lazily imported so the admin bundle never weighs on the
// operational ASD load.
const routes = [
  { path: '/', name: 'asd', component: AsdView },
  { path: '/admin', name: 'admin', component: () => import('@/views/AdminView.vue') },
]

export default createRouter({
  history: createWebHistory(),
  routes,
})
