import type { Blocked, Platform, Runner, RunnerState, Target } from '$lib/api/types';
import { progressRows, type ProgressRow } from '$lib/inventory';

export type VariantId = 'a' | 'b' | 'c';
export type Variant = { id: VariantId; path: string; name: string; note: string };

export const variants: Variant[] = [
	{
		id: 'a',
		path: '/sandbox/inventory-all-progress/',
		name: 'A Flat list, Mining marker',
		note: "Today's In progress rows, one list for all accounts. The drop being mined has a Mining tag and a purple bar with its ETA. Waiting drops get a grey bar and say why they wait: Up next, Needs link, Needs earlier drop or Skipped."
	},
	{
		id: 'b',
		path: '/sandbox/inventory-all-progress/v2',
		name: 'B Grouped per account',
		note: "One block per account, headed by its name and state. The current drop sits first on a tinted row with its ETA; the account's other started drops follow in plain rows, furthest along first."
	},
	{
		id: 'c',
		path: '/sandbox/inventory-all-progress/v3',
		name: 'C Current drop big, others compact',
		note: 'Each account gets one large row for the drop it mines now, with box art, a thick bar and the ETA. Its other started drops sit under it as one-line rows with a thin bar. An account that mines nothing right now shows only the compact rows.'
	}
];

const ART = {
	wt: 'https://static-cdn.jtvnw.net/ttv-boxart/66366_IGDB-144x192.jpg',
	rust: 'https://static-cdn.jtvnw.net/ttv-boxart/263490_IGDB-144x192.jpg',
	rivals: 'https://static-cdn.jtvnw.net/ttv-boxart/516575_IGDB-144x192.jpg'
};

let n = 0;
function drop(
	game: string,
	campaign: string,
	name: string,
	minutes: number,
	req: number,
	platform: Platform = 'twitch',
	art = ''
): Target {
	n += 1;
	return {
		game,
		platform,
		campaign_id: `camp-${game.toLowerCase().replace(/\W+/g, '-')}`,
		campaign_name: campaign,
		benefit_id: `ben-${n}`,
		benefit_name: name,
		image_url: art,
		required_minutes: req,
		minutes,
		ends_at: null,
		channels: [],
		channel_count: 0
	};
}

function runner(
	id: string,
	name: string,
	platform: Platform,
	state: RunnerState,
	current: Target | null,
	up_next: Target[],
	blocked: Blocked[] = []
): Runner {
	return {
		account_id: id,
		account_name: name,
		platform,
		avatar_url: '',
		state,
		current,
		channel: current ? 'some_channel' : '',
		viewer_count: 1200,
		minutes: current?.minutes ?? 0,
		started_at: null,
		last_scan_at: null,
		up_next,
		blocked,
		order_custom: false
	};
}

const wtTier2 = drop('War Thunder', 'War Thunder Twitch Drops: Season 14', 'Pz.Kpfw. IV Ausf. J decal set', 12, 60, 'twitch', ART.wt);

/** Fake /runners payload: four mining accounts and a signed-out one the list skips. */
export const runners: Runner[] = [
	runner(
		'acc-hjune',
		'hJune',
		'twitch',
		'watching',
		drop('War Thunder', 'War Thunder Twitch Drops: Season 14', 'M18 Hellcat camo', 42, 60, 'twitch', ART.wt),
		[
			drop('Rust', 'Rust Twitch Drops Round 41', 'Garage door skin', 25, 120, 'twitch', ART.rust),
			drop('Rust', 'Rust Twitch Drops Round 41', 'Hazmat suit', 0, 180, 'twitch', ART.rust)
		],
		[
			{
				target: drop('Marvel Rivals', 'Marvel Rivals Season 5 Viewer Rewards', 'Spray: Jeff the Land Shark', 30, 60, 'twitch', ART.rivals),
				reason: 'not_linked'
			},
			{
				target: drop('Marvel Rivals', 'Marvel Rivals Season 5 Viewer Rewards', 'Costume: Midnight Moon Knight', 0, 240, 'twitch', ART.rivals),
				reason: 'precondition'
			}
		]
	),
	runner(
		'acc-alt',
		'alt_farm_account_with_a_long_name',
		'twitch',
		'watching',
		drop('Rust', 'Rust Twitch Drops Round 41', 'Garage door skin', 18, 120, 'twitch', ART.rust),
		[wtTier2, drop('War Thunder', 'War Thunder Twitch Drops: Season 14', 'M18 Hellcat camo', 60, 60, 'twitch', ART.wt)]
	),
	runner(
		'acc-kick',
		'grubby_kick',
		'kick',
		'watching',
		drop(
			'Escape from Tarkov: Arena',
			'Arena Kick Drops Week 3 with Partnered Streamers Only',
			'Exclusive Legendary Weapon Skin Bundle: Season of the Ancient Dragon',
			95,
			120,
			'kick'
		),
		[drop('Escape from Tarkov: Arena', 'Arena Kick Drops Week 3 with Partnered Streamers Only', 'Dogtag', 8, 240, 'kick')],
		[
			{
				target: drop('Rust', 'Rust x Kick Creator Drops', 'Kick logo sign', 46, 90, 'kick'),
				reason: 'skipped'
			}
		]
	),
	runner(
		'acc-sleepy',
		'night_owl',
		'twitch',
		'sleep_window',
		null,
		[
			drop('War Thunder', 'War Thunder Twitch Drops: Season 14', 'M18 Hellcat camo', 51, 60, 'twitch', ART.wt),
			drop('Rust', 'Rust Twitch Drops Round 41', 'Garage door skin', 3, 120, 'twitch', ART.rust)
		]
	),
	runner('acc-out', 'signed_out', 'kick', 'needs_login', null, [drop('Rust', 'Rust x Kick Creator Drops', 'Kick logo sign', 20, 90, 'kick')])
];

export const rows: ProgressRow[] = progressRows(runners);

/** Short label for a waiting drop. Copy reused from existing UI keys. */
export function waitLabel(r: ProgressRow): string {
	switch (r.reason) {
		case 'not_linked':
			return 'Needs link';
		case 'precondition':
			return 'Needs earlier drop';
		case 'skipped':
			return 'Skipped';
		default:
			return 'Up next';
	}
}

export type Group = { runner: Runner; current: ProgressRow | null; rest: ProgressRow[] };

/** Rows per account, accounts in list order of their first row. */
export function groups(list: ProgressRow[]): Group[] {
	const out: Group[] = [];
	const by = new Map<string, Group>();
	for (const r of list) {
		let g = by.get(r.runner.account_id);
		if (!g) {
			g = { runner: r.runner, current: null, rest: [] };
			by.set(r.runner.account_id, g);
			out.push(g);
		}
		if (r.current) g.current = r;
		else g.rest.push(r);
	}
	return out;
}

/** "Mining: War Thunder" style state line for a group header. */
export function stateLine(g: Group): string {
	if (g.runner.state === 'sleep_window') return 'Sleep window';
	if (g.current) return `Mining: ${g.current.target.game}`;
	return 'Waiting';
}
