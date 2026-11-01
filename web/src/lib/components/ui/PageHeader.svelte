<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    title: string;
    subtitle?: string;
    icon?: string;
    actions?: Snippet;
    children?: Snippet;
  }

  let { title, subtitle = '', icon = '', actions, children }: Props = $props();
</script>

<header class="page-header">
  <div class="page-header__main">
    <h1 class="page-header__title">
      {#if icon}
        <i class={icon} aria-hidden="true"></i>
      {/if}
      <span>{title}</span>
    </h1>
    {#if subtitle}
      <p class="page-header__sub">{subtitle}</p>
    {/if}
    {@render children?.()}
  </div>

  {#if actions}
    <div class="page-header__actions">
      {@render actions()}
    </div>
  {/if}
</header>
