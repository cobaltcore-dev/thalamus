<template>
  <!--
    VerifiedBadge: shows whether this guide currently passes e2e on the
    commit being viewed. No generated files: at view time it asks the GitHub
    Actions API for runs of the guides workflow at __DOCS_SHA__ and matches
    the matrix job named "Guide (<page>)" (see .github/workflows/e2e-guides.yaml).
    __DOCS_SHA__ / __DOCS_REPO__ / __DOCS_LABEL__ are injected at VitePress
    build time (website/.vitepress/config.mts); for versioned docs the SHA is
    static (tag commit), for latest it is the built HEAD. Everything degrades
    to Unverified/Unknown when offline, rate-limited, or unpublished.
  -->
  <span v-if="state === 'verified'" class="verified-badge verified-pass">
    ✓ Verified · {{ label }} · {{ shortDate }} ·
    <a :href="runUrl" target="_blank" rel="noopener">run</a>
  </span>
  <span v-else-if="state === 'running'" class="verified-badge verified-running">● Verifying…</span>
  <span v-else-if="state === 'failed'" class="verified-badge verified-failed">
    ✗ Verification failing · <a :href="runUrl" target="_blank" rel="noopener">run</a>
  </span>
  <span v-else-if="state === 'unknown'" class="verified-badge verified-missing">○ Status unknown</span>
  <span v-else class="verified-badge verified-missing">○ Unverified</span>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

declare const __DOCS_SHA__: string | null
declare const __DOCS_REPO__: string | null
declare const __DOCS_LABEL__: string | null
declare const __GUIDES_WORKFLOW_FILE__: string

const props = defineProps<{
  page: string
}>()

type State = 'verified' | 'running' | 'failed' | 'unverified' | 'unknown'

const state = ref<State>('unverified')
const runUrl = ref('')
const runDate = ref('')

const label = computed(() => __DOCS_LABEL__ ?? 'docs')
const shortDate = computed(() => runDate.value.slice(0, 10))

interface WorkflowRun {
  id: number
  status: string
  conclusion: string | null
  updated_at: string
  html_url: string
  jobs_url: string
}

interface WorkflowJob {
  name: string
  status: string
  conclusion: string | null
  html_url: string
}

async function getJson(url: string): Promise<{ status: number; body: unknown }> {
  const res = await fetch(url, { headers: { Accept: 'application/vnd.github+json' } })
  let body: unknown = null
  try {
    body = await res.json()
  } catch {
    body = null
  }
  return { status: res.status, body }
}

onMounted(async () => {
  try {
    if (!__DOCS_SHA__ || !__DOCS_REPO__) {
      state.value = 'unverified'
      return
    }
    const runsUrl =
      `https://api.github.com/repos/${__DOCS_REPO__}` +
      `/actions/workflows/${__GUIDES_WORKFLOW_FILE__}/runs` +
      `?head_sha=${__DOCS_SHA__}&per_page=5`
    const runsRes = await getJson(runsUrl)
    if (runsRes.status === 404) {
      state.value = 'unverified'
      return
    }
    if (runsRes.status !== 200 || typeof runsRes.body !== 'object' || runsRes.body === null) {
      state.value = 'unknown'
      return
    }
    const runs = (runsRes.body as { workflow_runs?: WorkflowRun[] }).workflow_runs ?? []
    if (runs.length === 0) {
      state.value = 'unverified'
      return
    }
    // Prefer the newest completed run; otherwise surface an in-progress one.
    const run = runs.find((r) => r.status === 'completed') ?? runs[0]
    const jobsRes = await getJson(`${run.jobs_url}?per_page=100`)
    if (jobsRes.status !== 200 || typeof jobsRes.body !== 'object' || jobsRes.body === null) {
      state.value = 'unknown'
      return
    }
    const jobs = (jobsRes.body as { jobs?: WorkflowJob[] }).jobs ?? []
    const job = jobs.find((j) => j.name === `Guide (${props.page})`)
    if (!job) {
      state.value = 'unverified'
      return
    }
    runUrl.value = job.html_url || run.html_url
    runDate.value = run.updated_at
    if (job.status !== 'completed') {
      state.value = 'running'
    } else if (job.conclusion === 'success') {
      state.value = 'verified'
    } else {
      state.value = 'failed'
    }
  } catch {
    state.value = 'unknown'
  }
})
</script>

<style scoped>
.verified-badge {
  display: inline-block;
  flex-shrink: 0;
  font-size: 0.72rem;
  font-weight: 500;
  line-height: 1.35;
  padding: 0.55rem 0.55rem;
  border-radius: 0.75rem;
  border: 1px solid transparent;
  transition:
    background-color 0.4s ease,
    color 0.4s ease,
    border-color 0.4s ease;
}
.verified-badge a {
  color: inherit;
}
.verified-pass {
  color: #15803d;
  background: #f0fdf4;
  border-color: #bbf7d0;
}
.verified-running {
  color: #1d4ed8;
  background: #eff6ff;
  border-color: #bfdbfe;
}
.verified-failed {
  color: #b91c1c;
  background: #fef2f2;
  border-color: #fecaca;
}
.verified-missing {
  color: #c2410c;
  background: #fff7ed;
  border-color: #fed7aa;
}
</style>
