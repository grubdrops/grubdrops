<script lang="ts">
	import Theme from '../points-page/_Theme.svelte';
	import Mock from './_Mock.svelte';
	import { variants, type VariantId } from './_data';

	let { id }: { id: VariantId } = $props();
	const variant = $derived(variants.find((v) => v.id === id)!);

	let phone = $state(false);
	let skip = $state(0);
	const HOUR = 3_600_000;
</script>

<main>
	<div class="bar">
		<p>
			<a href="/sandbox/">Sandbox</a> / discover-upcoming /
			{#each variants as v, i (v.id)}{i ? ' · ' : ''}{#if v.id === id}<b>{v.id.toUpperCase()}</b>{:else}<a href={v.href}
						>{v.id.toUpperCase()}</a
					>{/if}{/each}
		</p>
		<div class="tools">
			<div class="seg" role="group" aria-label="Width">
				<button type="button" aria-pressed={!phone} onclick={() => (phone = false)}>Desktop</button>
				<button type="button" aria-pressed={phone} onclick={() => (phone = true)}>Phone 390</button>
			</div>
			<div class="seg" role="group" aria-label="Clock">
				<button type="button" onclick={() => (skip += 6 * HOUR)}>+6h</button>
				<button type="button" onclick={() => (skip += 24 * HOUR)}>+1d</button>
				<button type="button" disabled={skip === 0} onclick={() => (skip = 0)}>Now</button>
			</div>
			<Theme />
		</div>
	</div>
	<h1>{variant.name}</h1>
	<p class="trade">{variant.tradeoff}</p>
	<p class="note">
		{variant.note} Fake data: upcoming campaigns start later today, tomorrow and next week; Escape from Tarkov has no drops
		listed yet. The clock buttons move time so you can watch a campaign go live.
	</p>
	{#key id}
		<Mock variant={id} {phone} {skip} />
	{/key}
</main>

<style>
	main {
		padding: 24px 20px 48px;
		min-height: 100vh;
		max-width: 1100px;
		margin: 0 auto;
	}
	a {
		text-decoration: underline;
	}
	.bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		flex-wrap: wrap;
		gap: 12px;
	}
	.bar p {
		margin: 0;
	}
	.tools {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	.seg {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
	}
	.seg button {
		font: inherit;
		font-size: 12.5px;
		padding: 0 12px;
		height: 28px;
		border: 0;
		background: var(--surface);
		color: var(--t2);
		cursor: pointer;
	}
	.seg button + button {
		border-left: 1px solid var(--border);
	}
	.seg button[aria-pressed='true'] {
		background: var(--accent-soft);
		color: var(--accent);
	}
	.seg button:disabled {
		opacity: 0.45;
		cursor: default;
	}
	h1 {
		margin: 12px 0 4px;
		font-size: 22px;
	}
	.trade {
		margin: 0 0 4px;
		font-weight: 500;
		color: var(--accent);
		max-width: 80ch;
	}
	.note {
		color: var(--t2);
		margin: 0 0 16px;
		max-width: 80ch;
	}
	@media (max-width: 720px) {
		main {
			padding: 16px 16px 48px;
		}
	}
</style>
