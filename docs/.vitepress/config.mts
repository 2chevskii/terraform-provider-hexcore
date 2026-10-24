import { defineConfig } from 'vitepress'

export default defineConfig({
  lang: 'en-US',
  title: 'HexCore Terraform Provider',
  description: 'Discover HexCore catalogs and manage standard or custom VDS with Terraform.',
  base: '/terraform-provider-hexcore/',
  lastUpdated: true,
  cleanUrls: false,
  rewrites: {
    'index.md': 'reference/provider.md',
    'home.md': 'index.md'
  },
  sitemap: { hostname: 'https://2chevskii.github.io/terraform-provider-hexcore/' },
  themeConfig: {
    siteTitle: 'HexCore Provider',
    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Reference', link: '/reference/provider' },
      { text: 'Development', link: '/guide/development' },
      { text: 'Status', link: '/guide/status' }
    ],
    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting started', link: '/guide/getting-started' },
          { text: 'Configuration & catalogs', link: '/guide/configuration' },
          { text: 'VDS lifecycle', link: '/guide/vds-lifecycle' },
          { text: 'Import & recovery', link: '/guide/import' },
          { text: 'Troubleshooting', link: '/guide/troubleshooting' }
        ]
      },
      {
        text: 'Schema reference',
        items: [
          { text: 'Provider', link: '/reference/provider' },
          { text: 'VDS resource', link: '/resources/vds' },
          { text: 'VDS data source', link: '/data-sources/vds' },
          { text: 'Regions', link: '/data-sources/regions' },
          { text: 'Sizes', link: '/data-sources/sizes' },
          { text: 'Images', link: '/data-sources/images' },
          { text: 'Applications', link: '/data-sources/applications' }
        ]
      },
      {
        text: 'Project',
        items: [
          { text: 'Development & CI', link: '/guide/development' },
          { text: 'Release process', link: '/guide/releasing' },
          { text: 'Verification & limits', link: '/guide/status' }
        ]
      }
    ],
    socialLinks: [{ icon: 'github', link: 'https://github.com/2chevskii/terraform-provider-hexcore' }],
    search: { provider: 'local' },
    outline: [2, 3],
    editLink: {
      pattern: ({ relativePath }) => relativePath.startsWith('guide/')
        ? `https://github.com/2chevskii/terraform-provider-hexcore/edit/develop/docs/${relativePath}`
        : 'https://github.com/2chevskii/terraform-provider-hexcore/blob/develop/CONTRIBUTING.md#documentation'
    },
    footer: {
      message: 'MIT licensed. Community provider; live lifecycle verification remains pending.',
      copyright: 'Copyright © 2026 2CHEVSKII'
    }
  }
})
