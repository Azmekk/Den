<script lang="ts">
  import "../app.css";
  import { onMount, untrack } from "svelte";
  import { goto } from "$app/navigation";
  import { auth } from "$lib/stores/auth.svelte";
  import { voiceStore } from "$lib/stores/voice.svelte";
  import { websocket } from "$lib/stores/websocket.svelte";

  // biome-ignore lint/correctness/noUnusedVariables: used in template via {@render children()}
  let { children } = $props();
  let ready = $state(false);

  onMount(() => {
    auth.init().then(() => {
      ready = true;

      if (!auth.isLoggedIn) return;

      // Voice state listeners — must persist across page navigations
      websocket.on("voice_state_initial", voiceStore.handleVoiceStateInitial);
      websocket.on("voice_state_update", voiceStore.handleVoiceStateUpdate);

      // Connect WebSocket
      if (auth.accessToken) {
        websocket.connect(auth.accessToken);
      }
    });

    // Refresh token when tab becomes visible (handles sleep/background)
    function handleVisibilityChange() {
      if (document.visibilityState === "visible") {
        auth.refresh().then((ok) => {
          if (ok && auth.accessToken) {
            websocket.updateToken(auth.accessToken);
            if (!websocket.connected) {
              websocket.connect(auth.accessToken);
            }
          } else {
            goto("/login");
          }
        });
      }
    }
    document.addEventListener("visibilitychange", handleVisibilityChange);

    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      websocket.off("voice_state_initial", voiceStore.handleVoiceStateInitial);
      websocket.off("voice_state_update", voiceStore.handleVoiceStateUpdate);
      voiceStore.leave(true);
      websocket.disconnect();
    };
  });

  // Safety net: reconnect WS if logged in but not connected
  $effect(() => {
    const token = auth.accessToken;
    const isConnected = websocket.connected;
    const isReconnecting = websocket.reconnecting;
    if (auth.isLoggedIn && token && !isConnected && !isReconnecting) {
      untrack(() => {
        websocket.connect(token);
      });
    }
  });
</script>

<svelte:head>
  <title>Den</title>
</svelte:head>

{#if ready}
  {@render children()}
{:else}
  <div class="flex h-screen items-center justify-center">
    <div class="text-muted-foreground">Loading...</div>
  </div>
{/if}
