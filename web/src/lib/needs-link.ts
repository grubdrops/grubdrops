import type { NeedsLinkDrop, Platform } from '$lib/api/types';
import { linkHref } from '$lib/games';

/** The drops one account must link for in one campaign. */
export interface NeedsLinkGroup {
	key: string;
	account_id: string;
	account_name: string;
	platform: Platform;
	campaign_id: string;
	campaign_name: string;
	game: string;
	/** First safe http(s) link page of the group, '' when none. */
	href: string;
	/** Every drop's claim window has closed: no Connect or Check now. */
	expired: boolean;
	drops: NeedsLinkDrop[];
}

/** Groups drops by account and campaign, open groups first, each side in server order.
 * A campaign with open and closed drops splits into one group of each. */
export function groupNeedsLink(drops: NeedsLinkDrop[]): NeedsLinkGroup[] {
	const groups = new Map<string, NeedsLinkGroup>();
	for (const d of drops) {
		const key = `${d.account_id}:${d.platform}:${d.campaign_id}:${d.expired ? 'x' : 'o'}`;
		let g = groups.get(key);
		if (!g) {
			g = {
				key,
				account_id: d.account_id,
				account_name: d.account_name,
				platform: d.platform,
				campaign_id: d.campaign_id,
				campaign_name: d.campaign_name,
				game: d.game,
				href: '',
				expired: d.expired,
				drops: []
			};
			groups.set(key, g);
		}
		if (!g.href) g.href = linkHref(d.link_url);
		g.drops.push(d);
	}
	const all = [...groups.values()];
	return [...all.filter((g) => !g.expired), ...all.filter((g) => g.expired)];
}

/** The account and campaign a dismiss targets: open and closed groups of a campaign share it. */
export function dismissKey(g: { account_id: string; campaign_id: string }): string {
	return `${g.account_id}:${g.campaign_id}`;
}

/** One row of the section: a group, or the Undo line of a dismissed one. */
export type NeedsLinkRow = { kind: 'group'; group: NeedsLinkGroup } | { kind: 'dismissed'; key: string; group: NeedsLinkGroup };

/** The section's rows. A dismissed group folds into one Undo line where it stood; a dismissed
 * group the server no longer lists keeps its line at the end, so Undo stays until the page is left.
 * dismissed maps dismissKey to the group as it was when dismissed. */
export function needsLinkRows(groups: NeedsLinkGroup[], dismissed: ReadonlyMap<string, NeedsLinkGroup>): NeedsLinkRow[] {
	const rows: NeedsLinkRow[] = [];
	const seen = new Set<string>();
	for (const g of groups) {
		const key = dismissKey(g);
		if (!dismissed.has(key)) rows.push({ kind: 'group', group: g });
		else if (!seen.has(key)) {
			seen.add(key);
			rows.push({ kind: 'dismissed', key, group: dismissed.get(key)! });
		}
	}
	for (const [key, g] of dismissed) if (!seen.has(key)) rows.push({ kind: 'dismissed', key, group: g });
	return rows;
}

/** Open drops across groups: the count the section heading shows. */
export function openCount(groups: NeedsLinkGroup[]): number {
	return groups.reduce((n, g) => n + (g.expired ? 0 : g.drops.length), 0);
}
