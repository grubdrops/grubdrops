<script lang="ts">
	import BoxArt from '$lib/components/BoxArt.svelte';
	import Pill from './_Pill.svelte';
	import PlatformMark from '$lib/components/PlatformMark.svelte';
	import { etaShort } from '$lib/format';
	import type { ProgressRow } from '$lib/inventory';
	import { groups, rows, stateLine, waitLabel, type VariantId } from './_data';

	let { id }: { id: VariantId } = $props();

	const gs = groups(rows);
	let opened = $state<ProgressRow | null>(null);
	const key = (r: ProgressRow) => `${r.runner.account_id}:${r.target.benefit_id}`;
</script>

{#snippet num(r: ProgressRow)}
	<span class="num"><span>{r.target.minutes} / {r.target.required_minutes} min</span><span>{r.pct}%</span></span>
{/snippet}

{#snippet bar(r: ProgressRow, thick = false)}
	<span class="bar" class:wait={!r.current} class:thick><i style="width:{r.pct}%"></i></span>
{/snippet}

{#snippet side(r: ProgressRow, account = true)}
	{#if r.current}
		<span class="eta">~{etaShort(r.left)}{#if account}<small>on {r.runner.account_name}</small>{/if}</span>
	{:else}
		<span class="eta w" class:link={r.reason === 'not_linked'}>{waitLabel(r)}</span>
	{/if}
{/snippet}

<div class="frame">
	<section aria-label="In progress">
		<h2>
			<svg viewBox="0 0 16 16" fill="none" stroke="var(--accent)" stroke-width="1.6" aria-hidden="true"
				><circle cx="8" cy="8" r="6" stroke="var(--surface-3)" /><path d="M8 2a6 6 0 0 1 6 6" stroke-linecap="round" /></svg
			>In progress <span class="n">{rows.length}</span><Pill />
		</h2>

		{#if id === 'a'}
			<div class="panel">
				{#each rows as r (key(r))}
					<button type="button" class="ra" onclick={() => (opened = r)}>
						<span class="what">
							<span class="t"
								>{#if r.current}<span class="mine">Mining</span>{/if}<span class="tn">{r.target.benefit_name}</span></span
							>
							<span class="c">
								<span class="gc">{r.target.game} · {r.target.campaign_name}</span>
								<span class="acct"><PlatformMark platform={r.runner.platform} size="sm" /><span class="an">{r.runner.account_name}</span></span>
							</span>
						</span>
						<span class="bw">{@render num(r)}{@render bar(r)}</span>
						{@render side(r)}
					</button>
				{/each}
			</div>
		{:else if id === 'b'}
			<div class="stack">
				{#each gs as g (g.runner.account_id)}
					<div class="panel">
						<div class="gh">
							<span class="acct big"><PlatformMark platform={g.runner.platform} size="sm" /><span class="an">{g.runner.account_name}</span></span>
							<span class="st" class:on={!!g.current}>{stateLine(g)}</span>
						</div>
						{#each g.current ? [g.current, ...g.rest] : g.rest as r (key(r))}
							<button type="button" class="rb" class:cur={r.current} onclick={() => (opened = r)}>
								<span class="what">
									<span class="t"><span class="tn">{r.target.benefit_name}</span></span>
									<span class="c"><span class="gc">{r.target.game} · {r.target.campaign_name}</span></span>
								</span>
								<span class="bw">{@render num(r)}{@render bar(r)}</span>
								{@render side(r, false)}
							</button>
						{/each}
					</div>
				{/each}
			</div>
		{:else}
			<div class="panel">
				{#each gs as g (g.runner.account_id)}
					<div class="gc-block">
						{#if g.current}
							{@const r = g.current}
							<button type="button" class="hero" onclick={() => (opened = r)}>
								<span class="art"><BoxArt src={r.target.image_url || null} name={r.target.game} size="sm" /></span>
								<span class="what">
									<span class="t"><span class="tn">{r.target.benefit_name}</span></span>
									<span class="c">
										<span class="gc">{r.target.game} · {r.target.campaign_name}</span>
										<span class="acct"><PlatformMark platform={r.runner.platform} size="sm" /><span class="an">{r.runner.account_name}</span></span>
									</span>
									<span class="hb">{@render bar(r, true)}{@render num(r)}</span>
								</span>
								<span class="eta big">~{etaShort(r.left)}<small>left</small></span>
							</button>
						{:else}
							<div class="hero-none">
								<span class="acct"><PlatformMark platform={g.runner.platform} size="sm" /><span class="an">{g.runner.account_name}</span></span>
								<span class="st">{stateLine(g)}</span>
							</div>
						{/if}
						{#if g.rest.length > 0}
							<ul class="mini" class:solo={!g.current}>
								{#each g.rest as r (key(r))}
									<li>
										<button type="button" class="rc" onclick={() => (opened = r)}>
											<span class="mn">{r.target.benefit_name}</span>
											<span class="ml" class:link={r.reason === 'not_linked'}>{waitLabel(r)}</span>
											{@render bar(r)}
											<span class="mp">{r.pct}%</span>
										</button>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</section>

	<p class="opened" aria-live="polite">
		{#if opened}Opens the campaign drawer: <b>{opened.target.campaign_name}</b>, with <b>{opened.target.benefit_name}</b> highlighted.{:else}Click a row: it opens the campaign drawer.{/if}
	</p>

	<section class="claimed" aria-label="Claimed">
		<h2>Claimed <span class="n">128</span></h2>
		<p>Claimed rewards list, unchanged.</p>
	</section>
</div>

<style>
	.frame {
		container-type: inline-size;
		color: var(--text);
		font-size: 13px;
	}
	h2 {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 0 0 10px;
		font-size: 14px;
		font-weight: 600;
	}
	h2 svg {
		width: 16px;
		height: 16px;
	}
	.n {
		font: 400 12px var(--mono);
		color: var(--t2);
		background: var(--surface-3);
		border-radius: 10px;
		padding: 0 6px;
	}
	.panel {
		background: var(--surface);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius);
		overflow: hidden;
	}
	.panel > :global(* + *) {
		border-top: 1px solid var(--border-soft);
	}
	.stack {
		display: grid;
		gap: 10px;
	}
	button {
		font: inherit;
		color: inherit;
		text-align: left;
		background: none;
		border: 0;
		width: 100%;
		cursor: pointer;
	}
	button:hover {
		background: var(--surface-2);
	}
	button:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.what {
		min-width: 0;
	}
	.t {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		font-weight: 500;
		min-width: 0;
	}
	.tn {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		min-width: 0;
	}
	.c {
		font-size: 12px;
		color: var(--t2);
		display: flex;
		align-items: center;
		gap: 8px;
		margin-top: 3px;
		flex-wrap: wrap;
		min-width: 0;
	}
	.gc {
		min-width: 0;
		overflow-wrap: anywhere;
	}
	.acct {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--text);
		background: var(--surface-3);
		border-radius: 4px;
		padding: 1px 6px 1px 5px;
		max-width: 100%;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.an {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.acct.big {
		font-size: 13px;
		font-weight: 500;
		background: none;
		padding: 0;
	}
	.mine {
		flex: none;
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: 11px;
		font-weight: 600;
		color: var(--accent);
		background: var(--accent-soft);
		border-radius: 4px;
		padding: 1px 6px;
	}
	.mine::before {
		content: '';
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: currentColor;
	}
	.num {
		font: 12px var(--mono);
		font-variant-numeric: tabular-nums;
		color: var(--t3);
		display: flex;
		justify-content: space-between;
		gap: 8px;
		margin-bottom: 5px;
	}
	.bar {
		display: block;
		height: 4px;
		border-radius: 2px;
		background: var(--surface-3);
		overflow: hidden;
	}
	.bar.thick {
		height: 6px;
		border-radius: 3px;
	}
	.bar i {
		display: block;
		height: 100%;
		background: var(--accent);
		border-radius: inherit;
	}
	.bar.wait i {
		background: var(--t3);
		opacity: 0.6;
	}
	.eta {
		text-align: right;
		font-size: 13px;
		font-weight: 500;
		font-variant-numeric: tabular-nums;
	}
	.eta small {
		display: block;
		font-size: 12px;
		color: var(--t3);
		font-weight: 400;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.eta.w,
	.ml {
		font-weight: 400;
		font-size: 12px;
		color: var(--t3);
	}
	.eta.link,
	.ml.link {
		color: var(--amber);
	}

	/* A: flat list */
	.ra,
	.rb {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 240px 110px;
		gap: 16px;
		align-items: center;
		padding: 12px 14px;
	}

	/* B: grouped per account */
	.gh {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 10px 14px;
		background: var(--surface-2);
	}
	.gh .acct {
		min-width: 0;
	}
	.st {
		flex: none;
		font-size: 12px;
		color: var(--t3);
	}
	.st.on {
		color: var(--accent);
		font-weight: 500;
	}
	.rb {
		grid-template-columns: minmax(0, 1fr) 220px 90px;
		padding: 10px 14px;
	}
	.rb.cur {
		background: color-mix(in srgb, var(--accent) 9%, var(--surface));
	}
	.rb.cur:hover {
		background: color-mix(in srgb, var(--accent) 14%, var(--surface));
	}
	.rb:not(.cur) .t {
		font-weight: 400;
		color: var(--t2);
	}

	/* C: current drop big, others compact */
	.hero {
		display: grid;
		grid-template-columns: 46px minmax(0, 1fr) auto;
		gap: 14px;
		align-items: center;
		padding: 14px;
	}
	.art {
		width: 46px;
		height: 61px;
		border-radius: 4px;
		overflow: hidden;
	}
	.hero .t {
		font-size: 14px;
		font-weight: 600;
	}
	.hb {
		display: block;
		margin-top: 10px;
		max-width: 420px;
	}
	.hb .num {
		margin: 5px 0 0;
	}
	.eta.big {
		font-size: 18px;
		font-weight: 600;
		align-self: start;
	}
	.hero-none {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 10px 14px;
	}
	.mini {
		list-style: none;
		margin: 0;
		padding: 0 0 6px;
	}
	.mini.solo {
		padding-top: 0;
	}
	.rc {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto 80px 36px;
		gap: 12px;
		align-items: center;
		padding: 6px 14px 6px 74px;
		font-size: 12px;
	}
	.mini.solo .rc {
		padding-left: 14px;
	}
	.mn {
		color: var(--t2);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.mp {
		font: 12px var(--mono);
		font-variant-numeric: tabular-nums;
		color: var(--t3);
		text-align: right;
	}

	.opened {
		margin: 12px 0 20px;
		font-size: 12px;
		color: var(--t3);
	}
	.opened b {
		color: var(--t2);
		font-weight: 500;
	}
	.claimed p {
		color: var(--t3);
		margin: 0;
	}

	@container (max-width: 600px) {
		.ra,
		.rb {
			grid-template-columns: minmax(0, 1fr) auto;
			grid-template-areas: 'what eta' 'bar bar';
			row-gap: 10px;
		}
		.ra .what,
		.rb .what {
			grid-area: what;
		}
		.ra .eta,
		.rb .eta {
			grid-area: eta;
			align-self: start;
		}
		.ra .bw,
		.rb .bw {
			grid-area: bar;
		}
		.ra .eta small {
			max-width: 110px;
		}
		.hero {
			grid-template-columns: 40px minmax(0, 1fr) auto;
			gap: 12px;
			padding: 12px;
		}
		.art {
			width: 40px;
			height: 53px;
		}
		.eta.big {
			font-size: 15px;
		}
		.rc {
			grid-template-columns: minmax(0, 1fr) 56px 34px;
			grid-template-areas: 'mn bar mp' 'ml ml ml';
			row-gap: 2px;
			column-gap: 10px;
			padding: 6px 12px 6px 64px;
		}
		.mini.solo .rc {
			padding-left: 12px;
		}
		.rc .mn {
			grid-area: mn;
		}
		.rc .ml {
			grid-area: ml;
		}
		.rc .bar {
			grid-area: bar;
		}
		.rc .mp {
			grid-area: mp;
		}
	}
</style>
