<script lang="ts">
	import Mock from './_Mock.svelte';
	import Theme from '../points-page/_Theme.svelte';
	import { variants, type VariantId } from './_data';

	let { id }: { id: VariantId } = $props();
	const v = $derived(variants.find((x) => x.id === id)!);
	let phone = $state(false);
	let mock = $state<ReturnType<typeof Mock>>();
</script>

<main>
	<div class="bar">
		<p>
			<a href="/sandbox/">Sandbox</a> / needs-link-dismiss /
			{#each variants as x, i (x.id)}{i ? ' · ' : ''}{#if x.id === v.id}<b>{x.id}</b>{:else}<a href={x.path}>{x.id}</a>{/if}{/each}
		</p>
		<span class="tools">
			<span class="seg" role="group" aria-label="Width">
				<button type="button" aria-pressed={!phone} onclick={() => (phone = false)}>Desktop</button>
				<button type="button" aria-pressed={phone} onclick={() => (phone = true)}>390 px</button>
			</span>
			<button type="button" class="reset" onclick={() => mock?.reset()}>Reset</button>
			<Theme />
		</span>
	</div>
	<h1>Needs link: dismiss a group you'll never claim</h1>
	<p class="sub">
		GRUB-74. TTik3r mined Escape from Tarkov without owning it, so 20 drops sit in Needs link for 30 days. Dismiss hides
		one account and campaign. Backend is built: <code>POST /api/needs-link/dismiss</code> and
		<code>/restore</code> with <code>{'{account_id, campaign_id}'}</code>; hidden drops stay hidden when refused again
		and leave <code>GET /api/needs-link</code>. Fake data; Check now is faked too.
	</p>
	<h2>{v.name}</h2>
	<p class="note">{v.note}</p>
	<div class="frame" class:phone>
		{#key v.id}
			<Mock id={v.id} bind:this={mock} />
		{/key}
	</div>
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
		display: inline-flex;
		gap: 8px;
		flex-wrap: wrap;
	}
	.seg {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
	}
	.seg button,
	.reset {
		font: inherit;
		font-size: 12.5px;
		padding: 0 12px;
		height: 28px;
		border: 0;
		background: var(--surface);
		color: var(--t2);
		cursor: pointer;
	}
	.reset {
		border: 1px solid var(--border);
		border-radius: 8px;
	}
	.reset:hover {
		color: var(--text);
	}
	.seg button + button {
		border-left: 1px solid var(--border);
	}
	.seg button[aria-pressed='true'] {
		background: var(--surface-3);
		color: var(--text);
	}
	h1 {
		margin: 12px 0 4px;
		font-size: 22px;
	}
	h2 {
		margin: 20px 0 2px;
		font-size: 16px;
	}
	.sub,
	.note {
		color: var(--t2);
		margin: 0 0 12px;
		max-width: 80ch;
	}
	code {
		font-family: var(--mono);
		font-size: 0.92em;
	}
	.frame {
		padding: 20px 16px 24px;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: var(--bg);
	}
	.frame.phone {
		max-width: 390px;
	}
	@media (max-width: 720px) {
		main {
			padding: 16px 16px 48px;
		}
		.frame {
			padding: 16px 0 20px;
			border: 0;
			background: none;
		}
	}
</style>
