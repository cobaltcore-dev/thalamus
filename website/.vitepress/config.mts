import { withMermaid } from 'vitepress-plugin-mermaid'
import markdownItFootnote from 'markdown-it-footnote'

const docsVersion = process.env.DOCS_VERSION
const pagesBase = process.env.PAGES_BASE
const base = docsVersion && pagesBase
  ? `/${pagesBase}/${docsVersion}/`
  : docsVersion
    ? `/${docsVersion}/`
    : pagesBase
      ? `/${pagesBase}/`
      : '/'

export default withMermaid({
  title: 'Thalamus',
  description: 'Vendor-neutral, Kubernetes-native LLM inference service.',

  base,
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', href: `${base}favicon.svg`, type: 'image/svg+xml' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Thalamus',

    nav: [
      { text: 'Getting Started', link: '/getting-started' },
      { text: 'Demo', link: '/demo' },
      { text: 'Concepts', link: '/concepts/architecture' },
      { text: 'Reference', link: '/reference/model-crd-api' },
    ],

    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Overview', link: '/getting-started' },
        ],
      },
      {
        text: 'Demo',
        items: [
          { text: 'Demo', link: '/demo' },
        ],
      },
      {
        text: 'Concepts',
        collapsed: false,
        items: [
          { text: 'Architecture', link: '/concepts/architecture' },
        ],
      },
      {
        text: 'Reference',
        collapsed: false,
        items: [
          { text: 'Model CRD API', link: '/reference/model-crd-api' },
        ],
      },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/cobaltcore-dev/thalamus' },
    ],

    search: {
      provider: 'local',
    },

    editLink: {
      pattern: 'https://github.com/cobaltcore-dev/thalamus/edit/main/website/:path',
      text: 'Edit this page on GitHub',
    },

    outline: [2, 3, 4, 5],
  },

  markdown: {
    config: (md) => {
      md.use(markdownItFootnote)
    },
  },
})
