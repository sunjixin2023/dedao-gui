import { defineStore } from 'pinia'

export type LastArticleSession = {
  path: string
  title: string
  progress: number
  updatedAt: number
}

export const learningStore = defineStore('learningStore', {
  state: () => ({
    lastArticle: null as LastArticleSession | null,
  }),
  getters: {
    hasLastArticle: (state) => Boolean(state.lastArticle?.path),
  },
  actions: {
    saveLastArticle(session: LastArticleSession) {
      this.lastArticle = {
        path: String(session.path || ''),
        title: String(session.title || '继续学习'),
        progress: Math.max(0, Math.min(100, Number(session.progress) || 0)),
        updatedAt: Number(session.updatedAt) || Date.now(),
      }
    },
    clearLastArticle() {
      this.lastArticle = null
    },
  },
  persist: true,
})

