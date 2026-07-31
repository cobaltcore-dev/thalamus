import DefaultTheme from 'vitepress/theme'
import Layout from './Layout.vue'
import ImageCarousel from './components/ImageCarousel.vue'
import ChatTranscript from './components/ChatTranscript.vue'
import './custom.css'

export default {
  ...DefaultTheme,
  Layout,
  enhanceApp({ app }) {
    app.component('ImageCarousel', ImageCarousel)
    app.component('ChatTranscript', ChatTranscript)
  },
}
