<script lang="ts">
	import Icon from '$lib/components/Icon.svelte';
	import NewPill from '$lib/components/NewPill.svelte';
	import { SECTION_LOOK } from '$lib/settings-look';
	import type { Settings, SettingsPatch } from '$lib/api/types';
	import { t } from '$lib/i18n/index.svelte';
	import { shareCatalogToggle } from '$lib/settings';
	import SectionHead from './SectionHead.svelte';

	let {
		s,
		save,
		errs
	}: {
		s: Settings;
		save: (key: string, p: SettingsPatch) => Promise<boolean>;
		errs: Record<string, boolean>;
	} = $props();

	const share = $derived(shareCatalogToggle(s));
</script>

<section class="set hue-{SECTION_LOOK.experimental.hue}" id="experimental" aria-labelledby="h-exp">
	<SectionHead sec="experimental" id="h-exp" blurb={t('settings.experimental_sub')} />
	<div class="panel">
		<div class="row" class:na={share.disabled}>
			<div class="lbl">
				<b id="share-l">{t('settings.exp_share_catalog')}<NewPill since="2.3" /></b>
				<small>{t(share.hint)}</small>
				{#if errs.exp_share_catalog}<small class="bad">{t('settings.save_failed')}</small>{/if}
			</div>
			{#if share.disabled}
				<span class="chip off">{t('settings.unavailable')}</span>
			{:else}
				<button
					type="button"
					class="sw"
					role="switch"
					aria-checked={s.exp_share_catalog}
					aria-labelledby="share-l"
					onclick={() => save('exp_share_catalog', { exp_share_catalog: !s.exp_share_catalog })}
				></button>
			{/if}
		</div>
		<div class="row">
			<div class="lbl">
				<b id="daily-l">{t('settings.kick_daily_reward')}<NewPill since="2.3" /></b>
				<small>{t('settings.kick_daily_reward_sub')}</small>
				<p class="warn" role="note"><Icon name="alert" />{t('settings.kick_daily_reward_warn')}</p>
				{#if errs.kick_daily_reward}<small class="bad">{t('settings.save_failed')}</small>{/if}
			</div>
			<button
				type="button"
				class="sw"
				role="switch"
				aria-checked={s.kick_daily_reward}
				aria-labelledby="daily-l"
				onclick={() => save('kick_daily_reward', { kick_daily_reward: !s.kick_daily_reward })}
			></button>
		</div>
	</div>
</section>

<style>
	.bad {
		color: var(--red);
	}
	.na b {
		opacity: 0.5;
	}
	.warn {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		margin: 6px 0 0;
		padding: 7px 10px;
		border-radius: 9px;
		font-size: 12.5px;
		line-height: 1.45;
		color: var(--amber);
		background: var(--amber-soft);
	}
	.warn :global(svg) {
		flex: none;
		margin-top: 2px;
	}
</style>
