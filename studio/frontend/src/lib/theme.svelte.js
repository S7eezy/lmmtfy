// The colour theme. Studio follows the system until the user picks light or
// dark with the toggle, which is then remembered; toggling back to what the
// system shows follows the system again. app.css styles
// <html data-theme="dark">.
import { Environment, WindowSetDarkTheme, WindowSetLightTheme } from '../../wailsjs/runtime/runtime'

const KEY = 'lmmtfy.theme'
const media = window.matchMedia('(prefers-color-scheme: dark)')

function stored() {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : null
  } catch {
    return null
  }
}

let picked = stored() // null: follow the system
export const theme = $state({ dark: false })

let windows = false
try {
  Environment().then((env) => { windows = env.platform === 'windows'; apply() }).catch(() => {})
} catch {} // no Wails runtime (frontend alone)

function apply() {
  theme.dark = picked ? picked === 'dark' : media.matches
  document.documentElement.dataset.theme = theme.dark ? 'dark' : 'light'
  // The title bar on Windows has its own theme.
  if (windows) (theme.dark ? WindowSetDarkTheme : WindowSetLightTheme)()
}

export function toggleTheme() {
  const dark = !theme.dark
  picked = dark === media.matches ? null : dark ? 'dark' : 'light'
  try {
    if (picked) localStorage.setItem(KEY, picked)
    else localStorage.removeItem(KEY)
  } catch {}
  apply()
}

media.addEventListener('change', apply)
apply()
