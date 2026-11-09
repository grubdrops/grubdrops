<script lang="ts">
	import type { DiscoverCampaign, Platform } from '$lib/api/types';
	import BoxArt from '$lib/components/BoxArt.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import PlatformMark from '$lib/components/PlatformMark.svelte';
	import {
		filterGroups,
		groupByGame,
		groupUpcoming,
		isUpcoming,
		minutesLeft,
		sortGroups,
		splitByStart,
		type DiscoverGroup,
		type DiscoverSort
	} from '$lib/discover';
	import { clock, formatInt, shortDate } from '$lib/format';
	import { linkHref } from '$lib/games';
	import { t, tn } from '$lib/i18n/index.svelte';
	import { prefs } from '$lib/prefs.svelte';
	import { fakeChosen, fakeDiscover, type VariantId } from './_data';

	let { variant, phone = false, skip = 0 }: { variant: VariantId; phone?: boolean; skip?: number } = $props();

	const base = Date.now();
	const data = fakeDiscover(base);

	let tick = $state(Date.now());
	$effect(() => {
		const id = setInterval(() => (tick = Date.now()), 30_000);
		return () => clearInterval(id);
	});
	const now = $derived(tick + skip);

	let q = $state('');
	let platform = $state<'all' | Platform>('all');
	let hideAdded = $state(true);
	let sort = $state<DiscoverSort>('name');
	let tab = $state<'live' | 'upcoming'>('live');
	let showUpcoming = $state(true);
	let added = $state<string[]>([]);
	let adding = $state<string | null>(null);

	const chosen = $derived([...fakeChosen, ...added.map((name) => ({ name, slug: '' }))]);
	const split = $derived(splitByStart(data.campaigns, data.upcoming, now));
	const filter = $derived({ q, platform, hideAdded });

	const liveGroups = $derived(groupByGame(split.live, chosen));
	const liveShown = $derived(sortGroups(filterGroups(liveGroups, filter), sort));
	const upGroups = $derived(groupUpcoming(split.upcoming, chosen));
	const upShown = $derived(filterGroups(upGroups, filter));

	/** Variant C: one list, live campaigns first in each game, then upcoming. */
	const mixedShown = $derived.by(() => {
		const all = showUpcoming ? [...split.live, ...split.upcoming] : split.live;
		return sortGroups(filterGroups(groupByGame(all, chosen), filter), sort).map((g) => ({
			...g,
			campaigns: [...g.campaigns.filter((c) => !isUpcoming(c, now)), ...g.campaigns.filter((c) => isUpcoming(c, now))]
		}));
	});

	const head = $derived(tn('discover.head', liveGroups.length));
	const upCount = $derived(split.upcoming.length);

	function add(name: string): void {
		adding = name;
		setTimeout(() => {
			added = [...added, name];
			adding = null;
		}, 400);
	}

	function left(iso: string | null): string {
		const m = minutesLeft(iso, now);
		if (m === null) return t('discover.no_end');
		if (m >= 2 * 1440) return t('discover.left_d', { n: Math.floor(m / 1440) });
		if (m >= 60) return t('discover.left_h', { n: Math.floor(m / 60) });
		return t('discover.left_m', { n: m });
	}

	/** "in 5h", "in 6d": time until the start. Proposed copy. */
	function until(iso: string | null): string {
		const m = minutesLeft(iso, now) ?? 0;
		if (m >= 2 * 1440) return `in ${Math.floor(m / 1440)}d`;
		if (m >= 60) return `in ${Math.floor(m / 60)}h`;
		return `in ${Math.max(1, m)}m`;
	}

	const dayKey = (ms: number) =>
		new Intl.DateTimeFormat('en-CA', { timeZone: prefs.zone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(
			new Date(ms)
		);

	/** "starts today 21:00", "starts tomorrow 17:00", "starts Nov 16", in the display time zone. Proposed copy. */
	function starts(iso: string | null): string {
		if (!iso) return '';
		const at = Date.parse(iso);
		const time = clock(iso, prefs.zone, prefs.fmt);
		if (dayKey(at) === dayKey(now)) return `starts today ${time}`;
		if (dayKey(at) === dayKey(now + 86_400_000)) return `starts tomorrow ${time}`;
		return `starts ${shortDate(iso, prefs.zone)}`;
	}

	const onlyUpcoming = (g: DiscoverGroup) => g.campaigns.every((c) => isUpcoming(c, now));
	const name = (p: Platform) => (p === 'twitch' ? 'Twitch' : 'Kick');
</script>

<div class="frame" class:phone>
	<div class="mock">
		<div class="head">
			<h1>Discover</h1>
			<span class="now"><span class="dot"></span>{head}{#if variant === 'a' && upCount > 0}<span class="sep">·</span
					>{upCount} upcoming{/if}</span
			>
			<button type="button" class="btn rf"><Icon name="refresh" />{t('discover.refresh')}</button>
		</div>

		<div class="filters">
			{#if variant === 'b'}
				<div class="seg" role="group" aria-label="Campaigns">
					<button type="button" aria-pressed={tab === 'live'} onclick={() => (tab = 'live')}
						>Live <span class="n">{split.live.length}</span></button
					>
					<button type="button" aria-pressed={tab === 'upcoming'} onclick={() => (tab = 'upcoming')}
						>Upcoming <span class="n">{upCount}</span><span class="newpill">New</span></button
					>
				</div>
			{/if}
			<input class="search" type="search" placeholder={t('discover.search_ph')} aria-label={t('discover.search_ph')} bind:value={q} />
			<div class="fgroup" role="group" aria-label={t('discover.filter_platform')}>
				<span>{t('discover.filter_platform')}</span>
				<button type="button" class="chip" class:on={platform === 'all'} aria-pressed={platform === 'all'} onclick={() => (platform = 'all')}
					>{t('discover.all')}</button
				>
				{#each ['twitch', 'kick'] as const as p (p)}
					<button type="button" class="chip" class:on={platform === p} aria-pressed={platform === p} onclick={() => (platform = p)}
						><PlatformMark platform={p} size="sm" />{name(p)}</button
					>
				{/each}
			</div>
			{#if !(variant === 'b' && tab === 'upcoming')}
				<div class="fgroup" role="group" aria-label={t('discover.sort')}>
					<span>{t('discover.sort')}</span>
					{#each [['name', 'discover.sort_name'], ['newest', 'discover.sort_newest'], ['ending', 'discover.sort_ending']] as const as [s, key] (s)}
						<button type="button" class="chip" class:on={sort === s} aria-pressed={sort === s} onclick={() => (sort = s)}>{t(key)}</button>
					{/each}
				</div>
			{/if}
			<label class="toggle">
				<input type="checkbox" bind:checked={hideAdded} />
				<span>{t('discover.hide_added')}</span>
			</label>
			{#if variant === 'c'}
				<label class="toggle">
					<input type="checkbox" bind:checked={showUpcoming} />
					<span>Show upcoming</span><span class="newpill">New</span>
				</label>
			{/if}
		</div>

		{#if variant === 'a'}
			{@render list(liveShown, false)}
			{#if upShown.length > 0}
				<div class="section">
					<h2>Upcoming<span class="newpill">New</span></h2>
					<span class="count">{upShown.reduce((n, g) => n + g.campaigns.length, 0)} campaigns</span>
				</div>
				{@render list(upShown, true)}
			{/if}
		{:else if variant === 'b'}
			{#if tab === 'live'}
				{@render list(liveShown, false)}
			{:else if upShown.length === 0}
				<p class="empty">No upcoming campaigns.</p>
			{:else}
				{@render list(upShown, true)}
			{/if}
		{:else}
			{@render list(mixedShown, false)}
		{/if}
	</div>
</div>

{#snippet list(groups: DiscoverGroup[], upcomingList: boolean)}
	{#if groups.length === 0}
		<p class="empty">{t('discover.no_match')}</p>
	{/if}
	<div class="list">
		{#each groups as g (g.key)}
			<article class="game">
				<div class="artw"><BoxArt src={g.box_art_url || null} name={g.name} size="lg" /></div>
				<div class="body">
					<div class="status">
						<h3 class="gname">{g.name}</h3>
						{#if !upcomingList && variant === 'c' && onlyUpcoming(g)}<span class="tag">Upcoming</span>{/if}
						<span class="pfs">{#each g.platforms as p (`${g.key}:${p}`)}<PlatformMark platform={p} size="sm" />{/each}</span>
						<button type="button" class="add-btn" disabled={g.added || adding !== null} onclick={() => add(g.name)}
							>{g.added ? t('discover.added') : adding === g.name ? 'Adding…' : t('discover.add')}</button
						>
					</div>
					<ul class="camps">
						{#each g.campaigns as c (`${c.platform}/${c.id}`)}
							{@render campaign(c, isUpcoming(c, now))}
						{/each}
					</ul>
				</div>
			</article>
		{/each}
	</div>
{/snippet}

{#snippet campaign(c: DiscoverCampaign, up: boolean)}
	{@const href = linkHref(c.link_url)}
	<li class:up>
		<div class="ch">
			<PlatformMark platform={c.platform} />
			<span class="cn"
				>{c.name}{#if up && variant === 'c'}<span class="tag soon">{starts(c.starts_at)}</span>{/if}</span
			>
			{#if up}
				<small class="when" title={c.ends_at ? t('discover.ends', { date: shortDate(c.ends_at, prefs.zone) }) : undefined}
					>{until(c.starts_at)}{#if variant !== 'c'}{' · '}<b>{starts(c.starts_at)}</b>{/if}</small
				>
			{:else}
				<small
					>{left(c.ends_at)}{#if c.ends_at}{' · '}{t('discover.ends', { date: shortDate(c.ends_at, prefs.zone) })}{/if}</small
				>
			{/if}
		</div>
		{#if c.needs_link}
			<p class="link">
				<Icon name="alert" size={14} /><span>{t('discover.needs_link')}</span>
				{#if href}<a {href} target="_blank" rel="noopener noreferrer">{t('discover.link')}</a>{/if}
			</p>
		{/if}
		{#if c.rewards.length > 0}
			<ul class="rewards">
				{#each c.rewards as r, i (`${i}:${r.name}`)}
					<li>
						<span class="img"><BoxArt src={r.image_url || null} name={r.name} size="sm" /></span>
						<span class="rn">{r.name}</span>
						{#if r.minutes > 0}<small>{t('discover.minutes', { n: formatInt(r.minutes) })}</small>{/if}
					</li>
				{/each}
			</ul>
		{:else}
			<p class="muted">{up ? 'Drops not listed yet' : t('discover.no_rewards')}</p>
		{/if}
	</li>
{/snippet}

<style>
	.frame {
		container-type: inline-size;
	}
	.frame.phone {
		width: 390px;
		max-width: 100%;
		margin: 0 auto;
		border: 1px solid var(--border);
		border-radius: 16px;
		overflow: hidden;
	}
	.mock {
		background: var(--bg);
		padding: 20px 24px 32px;
	}
	.phone .mock {
		padding: 16px 16px 28px;
	}
	.head {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 8px 12px;
		margin-bottom: 16px;
	}
	.head h1 {
		margin: 0;
		font-size: 20px;
		font-weight: 650;
		letter-spacing: -0.02em;
	}
	.now {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		color: var(--t2);
	}
	.now .sep {
		color: var(--t3);
	}
	.dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--green);
	}
	.rf {
		margin-left: auto;
	}
	.btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		font-weight: 500;
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 3px 9px;
		color: var(--text);
		white-space: nowrap;
		cursor: pointer;
	}
	.btn:focus-visible,
	.chip:focus-visible,
	.add-btn:focus-visible,
	.search:focus-visible,
	.seg button:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
	}
	.link a {
		color: var(--accent);
		font-weight: 500;
	}
	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px 16px;
		margin-bottom: 14px;
	}
	.search {
		flex: 1 1 100%;
		min-width: 0;
		max-width: 320px;
		font-size: 13px;
		color: var(--text);
		background: var(--surface-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 4px 10px;
	}
	.seg {
		flex: 0 0 100%;
		display: flex;
	}
	.seg button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font: inherit;
		font-size: 13px;
		font-weight: 500;
		padding: 5px 14px;
		border: 1px solid var(--border);
		background: var(--surface);
		color: var(--t2);
		cursor: pointer;
	}
	.seg button:first-child {
		border-radius: 8px 0 0 8px;
	}
	.seg button + button {
		border-left: 0;
		border-radius: 0 8px 8px 0;
	}
	.seg button[aria-pressed='true'] {
		background: var(--accent-soft);
		color: var(--text);
	}
	.seg .n {
		font: 12px var(--mono);
		color: var(--t3);
	}
	.fgroup {
		display: flex;
		align-items: center;
		gap: 4px;
		flex-wrap: wrap;
	}
	.fgroup > span {
		font-size: 12px;
		color: var(--t3);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		font-weight: 600;
		margin-right: 4px;
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--t2);
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 2px 10px;
		cursor: pointer;
	}
	.chip:hover {
		color: var(--text);
		border-color: var(--t3);
	}
	.chip.on {
		color: var(--text);
		background: var(--accent-soft);
		border-color: color-mix(in srgb, var(--accent) 50%, transparent);
	}
	.toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--t2);
		cursor: pointer;
	}
	.toggle input {
		accent-color: var(--accent);
		margin: 0;
	}
	.newpill {
		display: inline-block;
		margin-left: 6px;
		padding: 1px 6px;
		border-radius: 999px;
		font-size: 10.5px;
		font-weight: 650;
		line-height: 1.5;
		letter-spacing: 0.02em;
		color: var(--accent);
		background: var(--accent-soft);
	}
	.section {
		display: flex;
		align-items: baseline;
		gap: 10px;
		margin: 28px 0 10px;
	}
	.section h2 {
		margin: 0;
		font-size: 15px;
		font-weight: 650;
		letter-spacing: -0.01em;
	}
	.section .count {
		font-size: 12px;
		color: var(--t3);
	}
	.game {
		display: grid;
		grid-template-columns: 116px minmax(0, 1fr);
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--surface);
		overflow: hidden;
	}
	.game + .game {
		margin-top: 10px;
	}
	.artw {
		position: relative;
		overflow: hidden;
		display: grid;
		min-height: 112px;
	}
	.artw :global(.art) {
		min-height: 112px;
	}
	.body {
		padding: 14px 18px;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.status {
		display: flex;
		align-items: center;
		gap: 6px 8px;
		flex-wrap: wrap;
	}
	.gname {
		margin: 0 4px 0 0;
		font-size: 15px;
		font-weight: 600;
		letter-spacing: -0.01em;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		min-width: 0;
	}
	.pfs {
		display: inline-flex;
		gap: 4px;
	}
	.tag {
		display: inline-block;
		font-size: 11px;
		font-weight: 600;
		padding: 1px 7px;
		border-radius: 999px;
		color: var(--accent);
		background: var(--accent-soft);
		white-space: nowrap;
	}
	.tag.soon {
		margin-left: 8px;
		font-weight: 500;
		vertical-align: 1px;
	}
	.add-btn {
		margin-left: auto;
		font-size: 12.5px;
		font-weight: 500;
		padding: 4px 10px;
		border-radius: 6px;
		border: 1px solid var(--border);
		background: var(--surface);
		color: var(--text);
		cursor: pointer;
		white-space: nowrap;
	}
	.add-btn:hover:not(:disabled) {
		border-color: var(--accent);
		color: var(--accent);
	}
	.add-btn:disabled {
		color: var(--t2);
		background: none;
		border-color: transparent;
		cursor: default;
	}
	.camps,
	.rewards {
		margin: 0;
		padding: 0;
		list-style: none;
		display: grid;
	}
	.camps {
		gap: 12px;
	}
	.camps > li {
		display: grid;
		gap: 6px;
		min-width: 0;
	}
	.ch {
		display: grid;
		grid-template-columns: 14px minmax(0, 1fr) auto;
		gap: 10px;
		align-items: center;
		font-size: 13px;
	}
	.cn {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		font-weight: 500;
	}
	.ch small {
		color: var(--t3);
		font: 12px var(--mono);
		white-space: nowrap;
	}
	.ch small.when b {
		font-weight: 500;
		color: var(--accent);
		white-space: nowrap;
	}
	.link {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 4px 8px;
		margin: 0 0 0 24px;
		color: var(--amber);
		font-size: 12px;
	}
	.rewards {
		margin-left: 24px;
		grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
		gap: 6px 12px;
	}
	.rewards li {
		display: grid;
		grid-template-columns: 28px minmax(0, 1fr) auto;
		gap: 8px;
		align-items: center;
		font-size: 12px;
		min-width: 0;
	}
	.img {
		width: 28px;
		height: 28px;
		border-radius: 4px;
		overflow: hidden;
		display: block;
	}
	.img :global(.art) {
		min-height: 0;
	}
	.rn {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		color: var(--t2);
	}
	.rewards small {
		font: 12px var(--mono);
		color: var(--t3);
		white-space: nowrap;
	}
	.muted {
		margin: 0 0 0 24px;
		color: var(--t3);
		font-size: 12px;
	}
	.empty {
		padding: 28px 16px;
		text-align: center;
		color: var(--t3);
		font-size: 13px;
		border: 1px dashed var(--border);
		border-radius: 10px;
		margin: 0;
	}
	@container (max-width: 720px) {
		.search {
			font-size: 16px;
			max-width: none;
		}
		.game {
			grid-template-columns: 64px minmax(0, 1fr);
		}
		.artw,
		.artw :global(.art) {
			min-height: 0;
		}
		.body {
			padding: 12px 14px;
		}
		.ch {
			grid-template-columns: 14px minmax(0, 1fr);
		}
		.ch small {
			grid-column: 2;
			margin-top: -4px;
			white-space: normal;
		}
		.cn {
			white-space: normal;
		}
		.tag.soon {
			margin-left: 6px;
		}
		.rewards,
		.link,
		.muted {
			margin-left: 0;
		}
	}
</style>
