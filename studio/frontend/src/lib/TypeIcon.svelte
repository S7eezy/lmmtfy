<script>
  // One icon per column type, used wherever a type is shown.
  import { Type, Hash, Sigma, ToggleLeft, Calendar, CalendarClock, Clock } from '@lucide/svelte'
  let { type, size = 14 } = $props()
  const icons = { string: Type, integer: Hash, decimal: Sigma, boolean: ToggleLeft, date: Calendar, datetime: CalendarClock, time: Clock }
  const Icon = $derived(icons[type] ?? Type)
</script>

<span class="type-icon" title={type}><Icon {size} /></span>

<style>
  .type-icon { display: inline-flex; vertical-align: -2px; color: var(--muted); }
</style>
