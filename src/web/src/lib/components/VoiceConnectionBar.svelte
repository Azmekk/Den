<script lang="ts">
  import { channelStore } from "$lib/stores/channels.svelte";
  import { voiceStore } from "$lib/stores/voice.svelte";
  import AudioSettingsPopover from "./AudioSettingsPopover.svelte";

  const currentVoiceChannel = $derived(
    channelStore.sortedVoiceChannels.find((c) => c.id === voiceStore.currentChannelId),
  );
</script>

{#if voiceStore.currentChannelId}
  <div class="border-t border-border bg-card/80 px-3 py-2">
    <div class="flex items-center gap-1.5 mb-1.5">
      <div class="h-2 w-2 rounded-full bg-green-500 shrink-0"></div>
      <span class="text-xs font-medium text-green-500 truncate"> Voice Connected </span>
    </div>
    <div class="flex items-center gap-1 text-xs text-muted-foreground mb-1.5 pl-3.5">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        class="shrink-0"
        ><path d="m2 2 20 20" /><path d="M18.89 13.23A7.12 7.12 0 0 0 19 12v-2" /><path
          d="M5 10v2a7 7 0 0 0 12 5"
        /><path d="M15 9.34V5a3 3 0 0 0-5.68-1.33" /><path d="M9 9v3a3 3 0 0 0 5.12 2.12" /><line
          x1="12"
          x2="12"
          y1="19"
          y2="22"
        /></svg
      >
      <span class="truncate">{currentVoiceChannel?.name ?? "Unknown"}</span>
    </div>
    <div class="flex items-center gap-0.5 pl-2">
      <!-- Mic toggle -->
      <button
        onclick={() => voiceStore.toggleMute()}
        class="rounded p-1.5 transition-colors {voiceStore.isMuted
          ? 'text-destructive hover:bg-destructive/10'
          : 'text-muted-foreground hover:bg-secondary hover:text-foreground'}"
        title={voiceStore.isMuted ? "Unmute" : "Mute"}
      >
        {#if voiceStore.isMuted}
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            ><path d="m2 2 20 20" /><path d="M18.89 13.23A7.12 7.12 0 0 0 19 12v-2" /><path
              d="M5 10v2a7 7 0 0 0 12 5"
            /><path d="M15 9.34V5a3 3 0 0 0-5.68-1.33" /><path d="M9 9v3a3 3 0 0 0 5.12 2.12" /><line
              x1="12"
              x2="12"
              y1="19"
              y2="22"
            /></svg
          >
        {:else}
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            ><path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z" /><path
              d="M19 10v2a7 7 0 0 1-14 0v-2"
            /><line x1="12" x2="12" y1="19" y2="22" /></svg
          >
        {/if}
      </button>

      <!-- Audio settings -->
      <AudioSettingsPopover />

      <!-- Disconnect -->
      <button
        onclick={() => voiceStore.leave()}
        class="rounded p-1.5 text-destructive hover:bg-destructive/10"
        title="Disconnect"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          ><path
            d="M10.68 13.31a16 16 0 0 0 3.41 2.6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7 2 2 0 0 1 1.72 2v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.42 19.42 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 2.59 3.4Z"
          /><line x1="2" x2="22" y1="2" y2="22" /></svg
        >
      </button>
    </div>
  </div>
{/if}
