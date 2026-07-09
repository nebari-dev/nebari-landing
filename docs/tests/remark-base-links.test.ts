import { describe, it, expect } from 'vitest';
import { remark } from 'remark';
import remarkBaseLinks, { prefixUrl } from '../src/plugins/remark-base-links';

describe('prefixUrl', () => {
  const cases: Array<{ name: string; url: string; base: string; want: string }> = [
    { name: 'base "/" leaves links unchanged', url: '/dev-quickstart/', base: '/', want: '/dev-quickstart/' },
    { name: 'sub-path base prefixes internal links', url: '/dev-quickstart/', base: '/nebari-landing/', want: '/nebari-landing/dev-quickstart/' },
    { name: 'prefixes image paths', url: '/screenshots/homepage-light.png', base: '/nebari-landing/', want: '/nebari-landing/screenshots/homepage-light.png' },
    { name: 'never rewrites external links', url: 'https://nebari.dev', base: '/nebari-landing/', want: 'https://nebari.dev' },
    { name: 'never rewrites protocol-relative links', url: '//example.com/x', base: '/nebari-landing/', want: '//example.com/x' },
    { name: 'never rewrites anchor-only links', url: '#section', base: '/nebari-landing/', want: '#section' },
    { name: 'preserves anchors on internal links', url: '/dev-quickstart/#step-1', base: '/nebari-landing/', want: '/nebari-landing/dev-quickstart/#step-1' },
    { name: 'idempotent on already-prefixed links', url: '/nebari-landing/dev-quickstart/', base: '/nebari-landing/', want: '/nebari-landing/dev-quickstart/' },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(prefixUrl(c.url, c.base)).toBe(c.want);
    });
  }
});

describe('remarkBaseLinks plugin', () => {
  it('rewrites link and image urls in a markdown document', async () => {
    const md = 'See [Quickstart](/dev-quickstart/) and ![img](/img/a.png) and [ext](https://nebari.dev)';
    const out = String(
      await remark().use(remarkBaseLinks, { base: '/nebari-landing/' }).process(md),
    );
    expect(out).toContain('(/nebari-landing/dev-quickstart/)');
    expect(out).toContain('(/nebari-landing/img/a.png)');
    expect(out).toContain('(https://nebari.dev)');
  });

  it('is a no-op when base is "/"', async () => {
    const md = '[Q](/dev-quickstart/)';
    const out = String(await remark().use(remarkBaseLinks, { base: '/' }).process(md));
    expect(out).toContain('(/dev-quickstart/)');
  });
});
