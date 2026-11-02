import { mount } from 'svelte'
import './app.css'
import './lib/theme.svelte.js' // before mounting: no flash of the wrong theme
import App from './App.svelte'

mount(App, { target: document.getElementById('app') })
