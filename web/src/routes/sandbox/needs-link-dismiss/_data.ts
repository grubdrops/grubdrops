import type { NeedsLinkDrop, Platform } from '$lib/api/types';

export type VariantId = 'a' | 'b' | 'c';
export type Variant = { id: VariantId; path: string; name: string; note: string };

export const variants: Variant[] = [
	{
		id: 'a',
		path: '/sandbox/needs-link-dismiss/',
		name: 'A Dismiss next to Check now',
		note: 'A quiet Dismiss text button sits after Check now, and on closed groups too. Clicking folds the group into one line with Undo, in place. The line goes on the next load.'
	},
	{
		id: 'b',
		path: '/sandbox/needs-link-dismiss/v2',
		name: 'B x on the group, Undo toast',
		note: 'An x at the top right of each group header. Clicking removes the group at once and a toast offers Undo for 6 seconds. No confirm step: Undo covers a slip.'
	},
	{
		id: 'c',
		path: '/sandbox/needs-link-dismiss/v3',
		name: 'C Hide, with a Hidden list at the bottom',
		note: 'Hide sits next to Check now. A hidden group leaves the list, and a footer row "2 hidden · 20 drops" opens them dimmed with Show again. Nothing is lost for good. Needs GET /api/needs-link to return hidden drops flagged.'
	}
];

const TTIK3R_TWITCH = 'acc_ttik3r_tw';
const TTIK3R_KICK = 'acc_ttik3r_kick';

function drops(
	account: string,
	platform: Platform,
	game: string,
	campaignID: string,
	campaign: string,
	names: string[],
	link: string,
	expired = false
): NeedsLinkDrop[] {
	return names.map((n, i) => ({
		account_id: account,
		account_name: 'TTik3r',
		platform,
		benefit_id: `${campaignID}-${i}`,
		benefit_name: n,
		image_url: '',
		campaign_id: campaignID,
		campaign_name: campaign,
		game,
		link_url: link,
		ends_at: null,
		expired
	}));
}

const EFT_LINK = 'https://www.escapefromtarkov.com/twitch';

/** Fake rows: two Tarkov campaigns the account can't claim (no game) and one CS2 group. */
export const sample: NeedsLinkDrop[] = [
	...drops(
		TTIK3R_TWITCH,
		'twitch',
		'Escape from Tarkov',
		'eft-hw-d1',
		'Echoes of Halloween day 1',
		[
			'Pumpkin balaclava',
			'Jack-o-lantern tactical mask',
			'Halloween dogtag',
			'Spooky sticker pack',
			'Bag of candy',
			'Witch hat',
			'Skull patch',
			'Bone knife skin',
			'Ghost armband',
			'Raven poster'
		],
		EFT_LINK
	),
	...drops(
		TTIK3R_TWITCH,
		'twitch',
		'Escape from Tarkov',
		'eft-hw-d3',
		'Echoes of Halloween day 3',
		[
			'Graveyard flag',
			'Candle set',
			'Black cat figurine',
			'Cursed dogtag',
			'Lantern',
			'Spider web scarf',
			'Moon patch',
			'Haunted radio',
			'Scarecrow mask',
			'Bat armband'
		],
		EFT_LINK
	),
	...drops(
		TTIK3R_KICK,
		'kick',
		'Counter-Strike 2',
		'iem-beijing-d2',
		'IEM Beijing 2026 - Day 2',
		['Sticker IEM Beijing gold', 'Sticker IEM Beijing holo', 'Graffiti Beijing'],
		'https://kick.com/settings/connections',
		true
	)
];

/** The key a dismissal targets: one account and campaign, open and closed drops alike. */
export function groupID(d: { account_id: string; campaign_id: string }): string {
	return `${d.account_id}:${d.campaign_id}`;
}
