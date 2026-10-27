<script>
  import Modal from './Modal.svelte'
  import { Settings2 } from '@lucide/svelte'
  import SettingsForm from './SettingsForm.svelte'
  let { mapping = $bindable(), targetPath = $bindable(), sources = [], ontarget, onclose } = $props()
</script>

<Modal title="Mapping settings" icon={Settings2} {onclose} wide>
  <SettingsForm bind:mapping bind:targetPath {sources} {ontarget} />
  {#snippet footer()}<button class="primary" onclick={onclose}>Done</button>{/snippet}
</Modal>
