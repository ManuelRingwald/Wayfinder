import { createApp } from 'vue'
import { createPinia } from 'pinia'

// Design System v1 (ADR 0015): bundled webfonts + design tokens.
// Fonts are self-hosted via @fontsource (offline, no runtime CDN) so an
// air-gapped ATC deployment makes no external font call. Roboto carries the
// UI; Roboto Mono carries tabular numeric readouts. Weights match the type
// scale in design/tokens/typography.css. Only the latin + latin-ext subsets
// are imported (the UI is German — umlauts live in latin-ext); this drops the
// cyrillic/greek/vietnamese/math subsets from the embedded bundle.
import '@fontsource/roboto/latin-300.css'
import '@fontsource/roboto/latin-400.css'
import '@fontsource/roboto/latin-500.css'
import '@fontsource/roboto/latin-700.css'
import '@fontsource/roboto/latin-ext-400.css'
import '@fontsource/roboto/latin-ext-500.css'
import '@fontsource/roboto-mono/latin-400.css'
import '@fontsource/roboto-mono/latin-500.css'
import '@fontsource/roboto-mono/latin-600.css'
import '@fontsource/roboto-mono/latin-ext-400.css'
import './design/tokens/index.css'
import './design/base.css'

import vuetify from './plugins/vuetify.js'
import router from './router/index.js'
import App from './App.vue'

const app = createApp(App)
app.use(createPinia())
app.use(vuetify)
app.use(router)
app.mount('#app')
