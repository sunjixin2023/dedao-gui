import { defineStore } from 'pinia'
import { Logout, SessionStatus } from '../../wailsjs/go/backend/App'
import { services } from '../../wailsjs/go/models'

export const classifySessionErrorMessage = (message: string) => {
  const lower = message.toLowerCase()
  if (/\b496\b/.test(message)) {
    return 'verification'
  }

  const isExpiredAuth =
    /\b(401|403)\b/.test(message) ||
    lower.includes('invalid csrf token') ||
    lower.includes('missing csrf token') ||
    lower.includes('csrftoken') ||
    lower.includes('csrf token')

  if (isExpiredAuth) {
    return 'expired'
  }

  return ''
}

export const userStore = defineStore('userStore', {
  state: () => ({
    userList: [] as services.User[],
    user: null as services.User | null,
    loggedIn: false,
    sessionLoaded: false,
    recoveryMessage: '',
    recoveryBackupPath: '',
  }),
  actions: {
    async refreshSession() {
      try {
        const status = await SessionStatus()
        this.loggedIn = Boolean(status.loggedIn)
        this.user = status.user ? Object.assign(new services.User(), status.user) : null
        this.recoveryMessage = status.recovery?.message || ''
        this.recoveryBackupPath = status.recovery?.backupPath || ''
      } catch (error) {
        console.warn('Session refresh failed:', error)
        this.user = null
        this.loggedIn = false
        this.recoveryMessage = ''
        this.recoveryBackupPath = ''
      } finally {
        this.sessionLoaded = true
      }
    },
    acceptLogin(user: services.User | null) {
      this.user = user
      this.loggedIn = Boolean(user)
      this.sessionLoaded = true
      this.recoveryMessage = ''
      this.recoveryBackupPath = ''
    },
    clearSession() {
      this.user = null
      this.loggedIn = false
      this.sessionLoaded = true
      this.recoveryMessage = ''
      this.recoveryBackupPath = ''
      localStorage.removeItem('cookies')
    },
    async classifySessionError(error: unknown) {
      const message = String(error || '')
      const classification = classifySessionErrorMessage(message)
      if (classification === 'verification') {
        return '需要验证，请先在得到官网完成验证码后重试'
      }
      if (classification === 'expired') {
        try {
          await Logout()
        } catch (logoutError) {
          console.warn('Backend session cleanup failed:', logoutError)
        }
        this.clearSession()
        return '登录已失效，请重新扫码登录'
      }
      return ''
    },
    async logout() {
      await Logout()
      this.clearSession()
    },
  },
  persist: { pick: ['userList'] },
})
