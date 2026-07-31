<template>
  <div class="image-carousel" :style="{ '--aspect-ratio': aspectRatio }">
    <div class="image-carousel-viewport">
      <Transition name="carousel-slide" mode="out-in">
        <div :key="currentIndex" class="image-carousel-slide">
          <img :src="images[currentIndex].src" :alt="images[currentIndex].alt || ''" />
        </div>
      </Transition>

      <button
        type="button"
        class="image-carousel-button image-carousel-prev"
        aria-label="Previous image"
        @click="prev"
      >
        <span class="image-carousel-arrow" aria-hidden="true">&#8249;</span>
      </button>
      <button
        type="button"
        class="image-carousel-button image-carousel-next"
        aria-label="Next image"
        @click="next"
      >
        <span class="image-carousel-arrow" aria-hidden="true">&#8250;</span>
      </button>

      <div class="image-carousel-dots" role="tablist" :aria-label="`${images.length} images`">
        <button
          v-for="(_, index) in images"
          :key="index"
          type="button"
          role="tab"
          :aria-selected="index === currentIndex"
          :aria-label="`Go to image ${index + 1}`"
          class="image-carousel-dot"
          :class="{ active: index === currentIndex }"
          @click="goTo(index)"
        />
      </div>
    </div>

    <div v-if="caption" class="image-carousel-caption">
      {{ caption }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'

interface CarouselImage {
  src: string
  alt?: string
}

const props = withDefaults(
  defineProps<{
    images: CarouselImage[]
    caption?: string
    interval?: number
    aspectRatio?: string
  }>(),
  {
    interval: 5000,
    aspectRatio: '16 / 9',
  }
)

const currentIndex = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const next = () => {
  currentIndex.value = (currentIndex.value + 1) % props.images.length
}

const prev = () => {
  currentIndex.value = (currentIndex.value - 1 + props.images.length) % props.images.length
}

const goTo = (index: number) => {
  currentIndex.value = index
}

const startAutoPlay = () => {
  if (props.interval <= 0 || props.images.length <= 1) return
  stopAutoPlay()
  timer = setInterval(next, props.interval)
}

const stopAutoPlay = () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(startAutoPlay)
onUnmounted(stopAutoPlay)

watch(() => props.images, startAutoPlay)
</script>

<style scoped>
.image-carousel {
  margin: 1.5rem 0;
}

.image-carousel-viewport {
  position: relative;
  overflow: hidden;
  border-radius: 8px;
  background: var(--vp-c-bg-soft);
  aspect-ratio: var(--aspect-ratio);
}

.image-carousel-slide {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.image-carousel-slide img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.image-carousel-button {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.35);
  color: #fff;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.2s ease, background 0.2s ease;
}

.image-carousel-viewport:hover .image-carousel-button,
.image-carousel-button:focus-visible {
  opacity: 1;
}

.image-carousel-button:hover {
  background: rgba(0, 0, 0, 0.55);
}

.image-carousel-prev {
  left: 0.5rem;
}

.image-carousel-next {
  right: 0.5rem;
}

.image-carousel-arrow {
  font-size: 1.5rem;
  line-height: 1;
}

.image-carousel-dots {
  position: absolute;
  bottom: 0.75rem;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 0.5rem;
}

.image-carousel-dot {
  width: 0.625rem;
  height: 0.625rem;
  border: none;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.5);
  cursor: pointer;
  transition: background 0.2s ease;
}

.image-carousel-dot.active,
.image-carousel-dot:hover {
  background: #fff;
}

.image-carousel-caption {
  margin-top: 0.5rem;
  text-align: center;
  font-size: 0.875rem;
  color: var(--vp-c-text-2);
}

.carousel-slide-enter-active,
.carousel-slide-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.carousel-slide-enter-from {
  opacity: 0;
  transform: scale(1.02);
}

.carousel-slide-leave-to {
  opacity: 0;
  transform: scale(0.98);
}
</style>
