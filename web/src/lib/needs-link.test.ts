import { describe, expect, it } from 'vitest';
import type { NeedsLinkDrop } from '$lib/api/types';
import { groupNeedsLink, openCount } from './needs-link';

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
