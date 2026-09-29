import { computed, onMounted, ref } from 'vue'

export interface VersionManifest {
  latest?: string
  versions?: string[]
}

let cached: Promise<VersionManifest | null> | null = null

function loadManifest(root: string): Promise<VersionManifest | null> {
  if (!cached) {
    cached = fetch(`${root}versions.json`, { cache: 'no-store' })
      .then((res) => (res.ok ? res.json() : null))
      .catch(() => null)
  }
  return cached
}

export function useVersions(root: string) {
  const manifest = ref<VersionManifest | null>(null)

  onMounted(() => {
    if (root) {
      loadManifest(root).then((value) => {
        manifest.value = value
      })
    }
  })

  const latest = computed(() => manifest.value?.latest ?? '')
  const versions = computed(() => manifest.value?.versions ?? [])

  return { manifest, latest, versions }
}
