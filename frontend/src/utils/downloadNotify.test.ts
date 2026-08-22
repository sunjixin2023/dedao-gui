import assert from 'node:assert/strict'
import test from 'node:test'

import { downloadNotifyMessage, notifyDownloadEnd, trySystemNotification } from './downloadNotify.ts'

test('completed downloads use a finite success copy', () => {
  assert.deepEqual(downloadNotifyMessage('completed', '得到·课程'), {
    title: '下载完成',
    message: '得到·课程',
    type: 'success',
  })
})

test('failed downloads keep the sanitized detail', () => {
  assert.equal(downloadNotifyMessage('failed', '下载失败，请检查网络连接后重试').type, 'error')
})

test('notifyDownloadEnd emits only terminal states', () => {
  const calls: unknown[] = []
  notifyDownloadEnd('downloading', 'x', (payload) => calls.push(payload))
  notifyDownloadEnd('completed', '得到·课程', (payload) => calls.push(payload))
  notifyDownloadEnd('failed', '下载失败，请检查网络连接后重试', (payload) => calls.push(payload))
  assert.equal(calls.length, 2)
})

test('notifyDownloadEnd does not throw when emit is a no-op', () => {
  assert.doesNotThrow(() => notifyDownloadEnd('cancelled', '已取消，可稍后继续下载', () => undefined))
})

test('trySystemNotification is a no-op without a constructor', () => {
  assert.doesNotThrow(() => trySystemNotification({ title: '下载完成', message: 'ok', type: 'success' }, undefined))
})
