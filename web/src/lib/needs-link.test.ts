import { describe, expect, it } from 'vitest';
import type { NeedsLinkDrop } from '$lib/api/types';
import { dismissKey, groupNeedsLink, needsLinkRows, openCount, type NeedsLinkRow } from './needs-link';

function drop(id: string, over: Partial<NeedsLinkDrop> = {}): NeedsLinkDrop {
	return {
		account_id: 'k1',
		account_name: 'Bob',
		platform: 'kick',
		benefit_id: id,
		benefit_name: 'Sticker ' + id,
		image_url: '',
		campaign_id: 'c-iem',
		campaign_name: 'IEM Beijing 2026 - Day 2',
		game: 'Counter-Strike 2',
		link_url: 'https://kick.com/connect',
		ends_at: '2026-11-02T12:00:00Z',
		expired: false,
		...over
	};
}

describe('groupNeedsLink', () => {
	it('groups one account and campaign together, in server order', () => {
		const g = groupNeedsLink([drop('r1'), drop('r2'), drop('r3')]);
		expect(g).toHaveLength(1);
		expect(g[0].drops.map((d) => d.benefit_id)).toEqual(['r1', 'r2', 'r3']);
		expect(g[0].href).toBe('https://kick.com/connect');
		expect(g[0].campaign_name).toBe('IEM Beijing 2026 - Day 2');
	});

	it('splits by account and puts closed groups last', () => {
		const g = groupNeedsLink([
			drop('old', { campaign_id: 'c-old', expired: true }),
			drop('r1'),
			drop('t1', { account_id: 't1', account_name: 'Tia', platform: 'twitch' })
		]);
		expect(g.map((x) => [x.account_id, x.campaign_id, x.expired])).toEqual([
			['k1', 'c-iem', false],
			['t1', 'c-iem', false],
			['k1', 'c-old', true]
		]);
		expect(openCount(g)).toBe(2);
	});

	it('splits a campaign with open and closed drops', () => {
		const g = groupNeedsLink([drop('r1'), drop('r2', { expired: true })]);
		expect(g.map((x) => x.expired)).toEqual([false, true]);
	});

	it('takes the first safe link and drops unsafe ones', () => {
		const g = groupNeedsLink([drop('r1', { link_url: 'javascript:alert(1)' }), drop('r2', { link_url: '' }), drop('r3', { link_url: 'https://ok.example/link' })]);
		expect(g[0].href).toBe('https://ok.example/link');
		expect(groupNeedsLink([drop('r1', { link_url: '' })])[0].href).toBe('');
	});

	it('is empty for no drops', () => {
		expect(groupNeedsLink([])).toEqual([]);
		expect(openCount([])).toBe(0);
	});
});

describe('needsLinkRows', () => {
	const kinds = (rows: NeedsLinkRow[]) => rows.map((r) => [r.kind, r.group.campaign_id, r.group.expired]);

	it('lists every group when none is dismissed', () => {
		const g = groupNeedsLink([drop('r1'), drop('d1', { campaign_id: 'c-day1' })]);
		expect(kinds(needsLinkRows(g, new Map()))).toEqual([
			['group', 'c-iem', false],
			['group', 'c-day1', false]
		]);
	});

	it('folds a dismissed group into one line where it stood', () => {
		const g = groupNeedsLink([drop('r1'), drop('d1', { campaign_id: 'c-day1' }), drop('d3', { campaign_id: 'c-day3' })]);
		const rows = needsLinkRows(g, new Map([[dismissKey(g[1]), g[1]]]));
		expect(kinds(rows)).toEqual([
			['group', 'c-iem', false],
			['dismissed', 'c-day1', false],
			['group', 'c-day3', false]
		]);
		expect(rows[1]).toMatchObject({ key: 'k1:c-day1' });
		const open = rows.flatMap((r) => (r.kind === 'group' ? [r.group] : []));
		expect(openCount(open)).toBe(2);
	});

	it('gives a campaign split into open and closed groups one line', () => {
		const g = groupNeedsLink([drop('r1'), drop('r2', { expired: true })]);
		const rows = needsLinkRows(g, new Map([[dismissKey(g[0]), g[0]]]));
		expect(kinds(rows)).toEqual([['dismissed', 'c-iem', false]]);
	});

	it('keeps the line of a group the server stopped listing, at the end', () => {
		const old = groupNeedsLink([drop('d1', { campaign_id: 'c-day1' })])[0];
		const now = groupNeedsLink([drop('r1')]);
		expect(kinds(needsLinkRows(now, new Map([[dismissKey(old), old]])))).toEqual([
			['group', 'c-iem', false],
			['dismissed', 'c-day1', false]
		]);
		expect(kinds(needsLinkRows([], new Map([[dismissKey(old), old]])))).toEqual([['dismissed', 'c-day1', false]]);
	});

	it('keys a dismiss by account and campaign', () => {
		expect(dismissKey({ account_id: 'k1', campaign_id: 'c-iem' })).toBe('k1:c-iem');
		expect(dismissKey({ account_id: 't1', campaign_id: 'c-iem' })).not.toBe('k1:c-iem');
	});
});
