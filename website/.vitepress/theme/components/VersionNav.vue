<template>
  <VPFlyout :button="label" :items="items" />
</template>

<script setup lang="ts">
import type { DefaultTheme } from 'vitepress/theme'
import { computed } from 'vue'
import VPFlyout from 'vitepress/dist/client/theme-default/components/VPFlyout.vue'
import { useVersions } from '../composables/useVersions'

const props = defineProps<{
  version: string
  root: string
  baseUrl?: string
}>()

const { versions } = useVersions(props.root)

const label = computed(
  () => (props.version === 'main' ? 'main (dev)' : props.version || 'version'),
)

const url = (v: string) =>
  props.baseUrl ? `${props.baseUrl}${props.root}${v}/` : `${props.root}${v}/`

const items = computed<DefaultTheme.NavItem[]>(() => {
  const list = versions.value.map((v): DefaultTheme.NavItemWithLink => ({
    text: v,
    link: url(v),
    target: '_self',
    noIcon: true,
  }))
  if (props.version !== 'main') {
    list.push({ text: 'main (dev)', link: url('main'), target: '_self', noIcon: true })
  }
  return list
})
</script>
