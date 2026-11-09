import type { DiscoverCampaign, DiscoverReward, Platform } from '$lib/api/types';

export type VariantId = 'a' | 'b' | 'c';

export const variants: { id: VariantId; href: string; name: string; tradeoff: string; note: string }[] = [
	{
		id: 'a',
		href: '/sandbox/discover-upcoming/',
		name: 'A. Upcoming section after the live list',
		tradeoff: 'Live campaigns stay on top as today. Upcoming ones follow under their own heading, soonest first.',
		note: 'Search, platform and Hide added apply to both lists. Sort only orders the live list: upcoming is always soonest first.'
	},
	{
		id: 'b',
		href: '/sandbox/discover-upcoming/v2',
		name: 'B. Live / Upcoming switch',
		tradeoff: 'One list at a time. The switch sits first in the filter row with a count on each side.',
		note: 'Cleanest live list, but upcoming is one click away and easy to miss. Sort hides on Upcoming.'
	},
	{
		id: 'c',
		href: '/sandbox/discover-upcoming/v3',
		name: 'C. Mixed into each game',
		tradeoff: 'An upcoming campaign sits in its game card after the live ones, tagged with its start.',
		note: 'Best for "what is coming for Rust". A game with only upcoming campaigns gets an Upcoming tag on its name. Show upcoming turns them off.'
	}
];

const HOUR = 3_600_000;
const DAY = 24 * HOUR;

/** Later today: 21:00 local, or partway to midnight when 21:00 has passed. */
function laterToday(now: number): number {
	const d = new Date(now);
	d.setHours(21, 0, 0, 0);
	if (d.getTime() > now + HOUR) return d.getTime();
	d.setHours(24, 0, 0, 0);
	return now + Math.max(60_000, Math.floor((d.getTime() - now) / 2));
}

function atLocal(now: number, days: number, hour: number): number {
	const d = new Date(now + days * DAY);
	d.setHours(hour, 0, 0, 0);
	return d.getTime();
}

const iso = (ms: number) => new Date(ms).toISOString();
const art = (id: string) => `https://static-cdn.jtvnw.net/ttv-boxart/${id}-144x192.jpg`;
const rw = (name: string, minutes: number): DiscoverReward => ({ name, image_url: '', minutes });

function c(
	id: string,
	platform: Platform,
	game: string,
	name: string,
	box: string,
	start: number,
	end: number,
	rewards: DiscoverReward[],
	extra: Partial<DiscoverCampaign> = {}
): DiscoverCampaign {
	return {
		id,
		platform,
		game,
		box_art_url: box,
		name,
		starts_at: iso(start),
		ends_at: iso(end),
		link_url: '',
		needs_link: false,
		rewards,
		...extra
	};
}

/** Fake Discover answer around now: live campaigns plus upcoming ones today, tomorrow and next week. */
export function fakeDiscover(now: number): { campaigns: DiscoverCampaign[]; upcoming: DiscoverCampaign[] } {
	const campaigns = [
		c('t-rust-hz', 'twitch', 'Rust', 'Rust Twitch Drops: Hazmat', art('263490'), now - 2 * DAY, now + 5 * DAY, [
			rw('Hazmat Hoodie', 120),
			rw('Hazmat Pants', 240),
			rw('Hazmat Helmet', 360)
		]),
		c('k-rust-14', 'kick', 'Rust', 'Rust Kick Drops #14', art('263490'), now - DAY, now + 5 * DAY, [
			rw('Kick Garage Door', 120),
			rw('Kick AK-47', 360)
		]),
		c(
			't-mr-45',
			'twitch',
			'Marvel Rivals',
			'Season 4.5 Drops',
			art('1264310518'),
			now - 10 * DAY,
			now + 2 * DAY + 5 * HOUR,
			[rw('Spray: Web Swing', 30), rw('Nameplate: Midtown', 60), rw('Emote: Victory Pose', 120)],
			{ needs_link: true, link_url: 'https://example.com/link' }
		),
		c('t-wt', 'twitch', 'War Thunder', 'War Thunder Drops: Autumn', art('66366'), now - 4 * DAY, now + 12 * DAY, [
			rw('Decal pack', 60),
			rw('3 days Premium', 240)
		]),
		c('k-abi', 'kick', 'Arena Breakout: Infinite', 'ABI x Kick Drops', '', now - 3 * DAY, now + 9 * DAY, [
			rw('Kick Weapon Charm', 60),
			rw('Kick Backpack Skin', 180)
		])
	];
	const upcoming = [
		c('t-mr-5', 'twitch', 'Marvel Rivals', 'Season 5 Launch Drops', art('1264310518'), laterToday(now), now + 14 * DAY, [
			rw('Spray: Season 5', 30),
			rw('Nameplate: Ultron', 60),
			rw('Costume: Venom Classic', 240)
		]),
		c('k-rust-15', 'kick', 'Rust', 'Rust Kick Drops #15', art('263490'), atLocal(now, 1, 17), now + 8 * DAY, [
			rw('Kick Hoodie', 120),
			rw('Kick Garage Door II', 240)
		]),
		c('k-df', 'kick', 'Delta Force', 'Delta Force Kick Drops', '', atLocal(now, 6, 18), now + 20 * DAY, [
			rw('Kick Operator Skin', 120)
		]),
		c('t-eft', 'twitch', 'Escape from Tarkov', 'Arena Season 3 Drops', art('491931'), atLocal(now, 7, 12), now + 21 * DAY, [])
	].map((x) => ({ ...x, upcoming: true }));
	return { campaigns, upcoming };
}

/** Games already added in the fake install. */
export const fakeChosen = [{ name: 'War Thunder', slug: 'war-thunder' }];
