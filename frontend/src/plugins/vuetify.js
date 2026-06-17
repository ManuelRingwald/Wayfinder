import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'
import * as components from 'vuetify/components'
import * as directives from 'vuetify/directives'

// ASD-007: Command-center color scheme derived from ASD mockup (2026-06-17).
// Documented in docs/design/color-tokens.md.
// Cyan primary (#23d3e6) against near-black surfaces maximises contrast between
// the WebGL map, track symbols, and UI chrome — the same principle used on
// real radar scopes. All tokens are stored here even if not yet wired up in a
// component, so future APs can reference them without touching this file.
const asdDarkTheme = {
  dark: true,
  colors: {
    // Background hierarchy (darkest → lightest surface level)
    background: '#070b12',
    surface: '#0e1622',
    'surface-variant': '#16202e',
    'surface-bright': '#1c2c3e',

    // Primary accent — cyan, aerospace/command-center convention
    primary: '#23d3e6',
    'primary-darken-1': '#0e8a9c',
    'on-primary': '#04141a',

    // Secondary — muted steel blue for inactive nav icons and secondary actions
    secondary: '#5b7a9d',
    'on-secondary': '#dce6f0',

    // Text on dark surfaces
    'on-background': '#dce6f0',
    'on-surface': '#dce6f0',
    'on-surface-variant': '#8a9bb0',

    // Semantic status colours — ATC-conventional, clearly distinguishable
    error: '#ff4338',
    warning: '#ffb02e',
    success: '#3ecf6b',
    info: '#3d9be0',
    'on-error': '#ffffff',
    'on-warning': '#1a0a00',
    'on-success': '#021a0a',
    'on-info': '#ffffff',
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
