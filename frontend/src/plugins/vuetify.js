import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'

// MD3 dark theme matching ASD radar scope aesthetics.
// Surface colors are dark (near-black) to avoid washing out the WebGL map.
const asdDarkTheme = {
  dark: true,
  colors: {
    background: '#0b0f14',
    surface: '#111720',
    'surface-variant': '#1a2030',
    primary: '#5b8fd6',
    'primary-darken-1': '#3a6baf',
    secondary: '#607d8b',
    'secondary-darken-1': '#455a64',
    error: '#b42318',
    info: '#4a90d9',
    success: '#1a7f37',
    warning: '#e8a030',
    'on-background': '#e8eef5',
    'on-surface': '#e8eef5',
    'on-primary': '#ffffff',
    'on-secondary': '#ffffff',
  },
}

export default createVuetify({
  components,
  directives,
  theme: {
    defaultTheme: 'asdDarkTheme',
    themes: { asdDarkTheme },
  },
  defaults: {
    VBtn: { variant: 'text' },
    VSwitch: { color: 'primary', density: 'compact', hideDetails: true },
    VTextField: {
      variant: 'outlined',
      density: 'compact',
      color: 'primary',
      hideDetails: true,
    },
  },
})
