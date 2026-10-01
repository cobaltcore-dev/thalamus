import { withMermaid } from 'vitepress-plugin-mermaid'
import markdownItFootnote from 'markdown-it-footnote'

const missing: string[] = []
function requiredEnv(name: string): string {
  const value = process.env[name]
  if (!value) {
    missing.push(name)
    return ''
  }
  return value
}

const docsVersion = requiredEnv('DOCS_VERSION')
const chartVersion = requiredEnv('CHART_VERSION')
const base = requiredEnv('DOCS_BASE')
const docsSha = requiredEnv('DOCS_SHA')
const docsRepo = requiredEnv('DOCS_REPO')

if (missing.length > 0) {
  throw new Error(
    `[docs] Missing required environment: ${missing.join(', ')}. ` +
    `Run "npm run docs:dev" in website/.`,
  )
}

export default withMermaid({
  title: 'Thalamus',
  description: 'Vendor-neutral, Kubernetes-native LLM inference service.',

  base,
  cleanUrls: true,
  lastUpdated: true,

  vite: {
    define: {
      __DOCS_SHA__: JSON.stringify(docsSha),
      __DOCS_REPO__: JSON.stringify(docsRepo),
      __DOCS_LABEL__: JSON.stringify(docsVersion),
      __GUIDES_WORKFLOW_FILE__: JSON.stringify('e2e-guides.yaml'),
    },
  },

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
      { text: 'Community', link: '/ipcei-cis-workshop-2026/' },
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
      pattern: 'https://github.com/cobaltcore-dev/thalamus/edit/main/website/:path',
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
