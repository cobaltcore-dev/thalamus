import { withMermaid } from 'vitepress-plugin-mermaid'
import markdownItFootnote from 'markdown-it-footnote'

const docsVersion = process.env.DOCS_VERSION || 'main'
const chartVersion = process.env.CHART_VERSION || '0.0.0-main'
const pagesBase = process.env.PAGES_BASE
const base = docsVersion && pagesBase
  ? `/${pagesBase}/${docsVersion}/`
  : docsVersion
    ? `/${docsVersion}/`
    : pagesBase
      ? `/${pagesBase}/`
      : '/'

// Root of all versioned builds, e.g. "/thalamus/" for base "/thalamus/v0.1.0/".
const baseRoot = base.endsWith(`${docsVersion}/`)
  ? base.slice(0, base.length - docsVersion.length - 1)
  : base
const docsBaseUrl = process.env.DOCS_BASE_URL || ''

const repoUrl =
  process.env.DOCUMENTATION_REPOSITORY_URL || 'https://github.com/cobaltcore-dev/thalamus'

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

    // Consumed by the VersionBanner and VersionNav theme components. Both
    // fetch <root>/versions.json at runtime, so frozen releases always know
    // the current versions without being rebuilt.
    versionBanner: {
      version: docsVersion,
      root: baseRoot,
      baseUrl: docsBaseUrl,
    },

    nav: [
      { text: 'Getting Started', link: '/getting-started' },
      { text: 'Demo', link: '/demo' },
      { text: 'Concepts', link: '/concepts/architecture' },
      { text: 'Reference', link: '/reference/model-crd-api' },
      { text: 'Community', link: '/ipcei-cis-workshop-2026/' },
      {
        component: 'VersionNav',
        props: { version: docsVersion, root: baseRoot, baseUrl: docsBaseUrl },
      },
    ],

    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Overview', link: '/getting-started' },
          { text: 'Open WebUI', link: '/open-webui' },
          { text: 'Perses Dashboards', link: '/perses-dashboards' },
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
        text: 'Backends',
        collapsed: false,
        items: [
          {
            text: 'Native',
            collapsed: false,
            items: [
              { text: 'Overview', link: '/concepts/backends/native/' },
              { text: 'Request flow', link: '/concepts/backends/native/request-flow' },
            ],
          },
        ],
      },
      {
        text: 'Reference',
        collapsed: false,
        items: [
          { text: 'Model CRD API', link: '/reference/model-crd-api' },
        ],
      },
      {
        text: 'Community',
        collapsed: false,
        items: [
          { text: 'IPCEI-CIS Hackathon July 2026', link: '/ipcei-cis-workshop-2026/' },
          { text: 'Naira Integration', link: '/ipcei-cis-workshop-2026/naira-integration' },
          { text: 'OCM Packaging of Thalamus', link: '/ipcei-cis-workshop-2026/ocm-packaging' },
          { text: 'OCM Model Weights', link: '/ipcei-cis-workshop-2026/ocm-model-weights' },
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
      pattern: `${repoUrl}/edit/${docsVersion}/website/:path`,
      text: 'Edit this page on GitHub',
    },

    outline: [2, 3, 4, 5],
  },

  markdown: {
    config: (md) => {
      md.core.ruler.before('normalize', 'replace-docs-version', (state) => {
        state.src = state.src
          .replace(/@@DOCS_VERSION@@/g, docsVersion)
          .replace(/@@CHART_VERSION@@/g, chartVersion)
      })
      md.use(markdownItFootnote)
    },
  },
})
