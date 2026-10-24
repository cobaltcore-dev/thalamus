<template>
  <Layout>
    <template #layout-top>
      <VersionBanner />
    </template>
    <template #layout-bottom>
      <SiteFooter :class="{ 'has-sidebar': hasSidebar }" />
    </template>
  </Layout>
</template>

<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import { useSidebar } from 'vitepress/theme'
import mediumZoom from 'medium-zoom'
import { onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vitepress'
import SiteFooter from './components/Footer.vue'
import VersionBanner from './components/VersionBanner.vue'

const { Layout } = DefaultTheme
const { hasSidebar } = useSidebar()

const route = useRoute()

const initImageZoom = () => {
  mediumZoom('.main img:not(.carousel-zoom-image)', { background: 'var(--vp-c-bg)', margin: 80 })
}

onMounted(() => {
  initImageZoom()
})

watch(
  () => route.path,
  () =>
    nextTick(() => {
      initImageZoom()
    })
)
</script>
