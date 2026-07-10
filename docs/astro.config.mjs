// docs/astro.config.mjs
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { nebari } from '@nebari/starlight';
import rehypeMermaid from 'rehype-mermaid';
import remarkBaseLinks from './src/plugins/remark-base-links';

// BASE and SITE are set by CI when deploying under a subpath (e.g. packs.nebari.dev/nebari-landing/).
// Default '/' is the right thing for `astro dev` and local previews.
export default defineConfig({
  base: process.env.BASE || '/',
  site: process.env.SITE,
  integrations: [
    starlight({
      title: 'Nebari Landing',
      description:
        'The Launchpad for Nebari — service discovery and access portal. React SPA + Go webapi, deployed via the nebari-landing Helm chart and reconciled by the Nebari Operator.',
      // Shared Nebari identity (brand colors, fonts, logo, favicon, footer, GitHub link)
      // comes from the @nebari/starlight theme plugin. logoHref sets where the header logo
      // takes the reader when they click it — nebari.dev for the project's main site.
      plugins: [nebari({ logoHref: 'https://nebari.dev/' })],
      sidebar: [
        {
          label: 'Overview',
          items: [
            { label: 'Introduction', slug: 'index' },
          ],
        },
        {
          label: 'Development',
          items: [
            { label: 'Frontend Dev Quickstart', slug: 'dev-quickstart' },
          ],
        },
        // Additional groups (Reference / API, Architecture, Maintainers) will be
        // populated as the remaining markdown files under docs/ are migrated into
        // src/content/docs/. See CONTRIBUTING for the migration checklist.
      ],
    }),
  ],
  markdown: {
    // Turn Shiki off for mermaid so rehype-mermaid sees the raw graph source.
    syntaxHighlight: { type: 'shiki', excludeLangs: ['mermaid'] },
    remarkPlugins: [[remarkBaseLinks, { base: process.env.BASE || '/' }]],
    rehypePlugins: [[rehypeMermaid, { strategy: 'inline-svg' }]],
  },
});
