<template>
  <div v-if="show" class="docs-version-banner" role="note">
    <p>
      <template v-if="isMain">
        You are viewing the <strong>main (dev)</strong> documentation.
      </template>
      <template v-else>
        You are viewing the <strong>{{ version }}</strong> documentation.
      </template>
      <a class="vp-raw" :href="latestLink">Latest release ({{ latest }})</a>
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { inBrowser, useData } from 'vitepress'
import { useVersions } from '../composables/useVersions'

const { theme } = useData()

const config = computed(() => theme.value.versionBanner ?? {})
const version = computed(() => config.value.version ?? '')
const root = computed(() => config.value.root ?? '')
const baseUrl = computed(() => config.value.baseUrl ?? '')

const { latest } = useVersions(root.value)

const isMain = computed(() => version.value === 'main')

const show = computed(
  () => !!version.value && !!latest.value && latest.value !== version.value,
)

const latestLink = computed(
  () =>
    baseUrl.value
      ? `${baseUrl.value}${root.value}${latest.value}/`
      : `${root.value}${latest.value}/`,
)

watch(
  show,
  (value) => {
    if (!inBrowser) return
    const height = value
      ? getComputedStyle(document.documentElement).getPropertyValue('--vp-docs-banner-height') || '2.5rem'
      : '0px'
    document.documentElement.style.setProperty('--vp-layout-top-height', height)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (inBrowser) {
    document.documentElement.style.setProperty('--vp-layout-top-height', '0px')
  }
})
</script>

<style scoped>
.docs-version-banner {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: calc(var(--vp-z-index-nav, 40) + 10);
  height: var(--vp-docs-banner-height);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 16px;
  background-color: var(--vp-c-bg-soft);
  border-bottom: 1px solid var(--vp-c-divider);
  font-size: 13px;
  line-height: 1.5;
  color: var(--vp-c-text-2);
  text-align: center;
}

.docs-version-banner p {
  margin: 0;
}

.docs-version-banner a {
  color: var(--vp-c-brand-1);
  text-decoration: none;
}

.docs-version-banner a:hover {
  text-decoration: underline;
}
</style>
