<template>
  <div class="ebook-reader-page">
    <header class="reader-header">
      <div class="header-left">
        <el-button type="primary" link @click="goBack">← 返回书架</el-button>
        <el-button link @click="tocCollapsed = !tocCollapsed">
          {{ tocCollapsed ? '☰ 目录' : '✕ 收起' }}
        </el-button>
      </div>
      <div class="header-center">
        <span class="book-title">{{ displayTitle }}</span>
        <span class="chapter-info" v-if="chapters.length > 0">
          {{ activeDisplayIndex }} / {{ chapters.length }}
        </span>
      </div>
      <div class="header-right">
        <el-button link @click="noteDrawerVisible = true">笔记 {{ readerNotes.length }}</el-button>
        <el-button link :loading="loadingPages || loadingInfo" @click="reloadCurrentChapter">刷新</el-button>
        <el-button link @click="goPrevChapter" :disabled="activeDisplayIndex <= 1">上一章</el-button>
        <el-button link @click="goNextChapter" :disabled="activeDisplayIndex >= chapters.length">下一章</el-button>
      </div>
      <div class="progress-bar" :style="{ width: readProgress + '%' }"></div>
    </header>

    <section class="reader-layout" :class="{ 'toc-collapsed': tocCollapsed }">
      <aside class="chapter-panel" v-loading="loadingInfo">
        <div class="chapter-panel-header">
          <h3>目录</h3>
        </div>
        <div class="chapter-list">
          <button
            v-for="(chapter, index) in chapters"
            :key="chapter.chapterId"
            class="chapter-item"
            :class="{ 'active': chapter.chapterId === activeChapterId }"
            @click="jumpToChapter(chapter.chapterId)"
          >
            <span class="chapter-index">{{ index + 1 }}</span>
            <span class="chapter-title" :title="chapter.title">{{ chapter.title }}</span>
          </button>
        </div>
      </aside>

      <main
        ref="contentRef"
        class="reader-content"
        v-loading="loadingInfo"
        @scroll="onContentScroll"
        @mouseup="onReaderMouseUp"
        @touchend="onReaderMouseUp"
        @click="onMarkClick"
      >
        <div v-if="errorText && contentBlocks.length === 0" class="reader-error">
          <el-result icon="error" title="加载失败" :sub-title="errorText">
            <template #extra>
              <el-button type="primary" @click="reloadCurrentChapter">重试</el-button>
            </template>
          </el-result>
        </div>

        <div v-else-if="!loadingPages && contentBlocks.length === 0 && !loadingInfo" class="reader-empty">
          <el-empty description="当前章节暂无可阅读内容">
            <el-button type="primary" @click="reloadCurrentChapter">重新加载</el-button>
          </el-empty>
        </div>

        <!-- Continuous scrolling: all loaded chapters stacked vertically -->
        <div class="page-stream">
          <div
            v-for="block in contentBlocks"
            :key="block.chapterId"
            :id="'ch-' + block.chapterId"
            class="chapter-block"
          >
            <div class="chapter-divider">{{ block.title }}</div>
            <div class="html-reader" v-html="block.html"></div>
          </div>
        </div>

        <div
          v-if="selectionMenu.visible"
          class="selection-toolbar"
          :style="{ left: selectionMenu.x + 'px', top: selectionMenu.y + 'px' }"
          @mousedown.prevent
        >
          <el-button size="small" type="primary" @click="createNoteBySelection(false)">划线</el-button>
          <el-button size="small" @click="createNoteBySelection(true)">写笔记</el-button>
        </div>

        <div v-if="loadingPages" class="loading-hint">加载中…</div>

        <div v-if="contentBlocks.length > 0 && !loadingPages" class="chapter-end-hint">
          <span v-if="hasMoreChapters">— 继续滚动加载下一章 —</span>
          <span v-else>— 全书完 —</span>
        </div>
      </main>
    </section>

    <el-drawer
      v-model="noteDrawerVisible"
      title="电子书笔记"
      direction="rtl"
      size="360px"
      destroy-on-close
    >
      <div class="note-drawer-header">
        <span>共 {{ readerNotes.length }} 条</span>
        <div class="note-drawer-actions">
          <el-button
            link
            type="primary"
            :loading="publishingNoteId === '__first__'"
            :disabled="!!publishingNoteId && publishingNoteId !== '__first__'"
            @click="publishNoteToKnowledge()"
          >
            发布首条到城邦
          </el-button>
          <el-button link type="primary" @click="openPublishPage()">去发布页</el-button>
          <el-button link @click="copyAndGoKnowledge()">复制并去城邦</el-button>
        </div>
      </div>
      <div v-if="readerNotes.length === 0" class="note-empty">
        <el-empty description="先在正文里选中文字进行划线或写笔记" />
      </div>
      <div v-else class="note-list">
        <div
          v-for="note in readerNotes"
          :key="note.id"
          class="note-item"
          :class="{ active: note.id === activeNoteId }"
        >
          <div class="note-meta">
            <span class="note-chapter" :title="note.chapterTitle">{{ note.chapterTitle }}</span>
            <div class="note-meta-right">
              <span class="note-sync" v-if="!note.synced">仅本地</span>
              <span class="note-time">{{ formatNoteTime(note.createdAt) }}</span>
            </div>
          </div>
          <div class="note-quote">“{{ note.quote }}”</div>
          <div v-if="note.comment" class="note-comment">{{ note.comment }}</div>
          <div class="note-actions">
            <el-button link type="primary" @click="locateNote(note)">定位</el-button>
            <el-button link @click="copyNote(note)">复制</el-button>
            <el-button link type="primary" @click="openPublishPage(note)">去发布页</el-button>
            <el-button
              link
              type="primary"
              :loading="publishingNoteId === note.id"
              :disabled="!!publishingNoteId && publishingNoteId !== note.id"
              @click="publishNoteToKnowledge(note)"
            >
              发城邦
            </el-button>
            <el-button link type="danger" @click="removeNote(note)">删除</el-button>
          </div>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script lang="ts" setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import 'element-plus/es/components/message-box/style/css'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute } from 'vue-router'
import { invokeBackend } from '../utils/backend'
import { useAppRouter } from '../composables/useRouter'
import { Local } from '../utils/storage'
import { ROUTE_NAMES } from '../router/routes'

type EbookReadInfoResp = {
  bookInfo?: {
    orders?: Array<{ chapterId: string; pathInEpub: string }>
    toc?: Array<{ href: string; level: number; playOrder: number; offset: number; text: string }>
  }
}
type EbookSyncedNote = {
  note_id?: number
  note_id_hazy?: string
  note?: string
  note_line?: string
  create_time?: number
  update_time?: number
  extra?: {
    book_section?: string
    location?: string
  }
}
type ReaderChapter = { chapterId: string; title: string }
type ContentBlock = { chapterId: string; title: string; html: string }
type ReaderNote = {
  id: string
  remoteId: string
  synced: boolean
  chapterId: string
  chapterTitle: string
  quote: string
  comment: string
  createdAt: number
}

const route = useRoute()
const { pushEbookList, pushByName } = useAppRouter()

const loadingInfo = ref(false)
const loadingPages = ref(false)
const errorText = ref('')
const tocCollapsed = ref(false)
const readProgress = ref(0)

const chapters = ref<ReaderChapter[]>([])
const activeChapterId = ref('')
const contentBlocks = ref<ContentBlock[]>([])
const contentRef = ref<HTMLElement | null>(null)
const noteDrawerVisible = ref(false)
const readerNotes = ref<ReaderNote[]>([])
const activeNoteId = ref('')
const publishingNoteId = ref('')
const selectionMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  quote: '',
  chapterId: '',
})

const htmlCache = new Map<string, string>()
const htmlPending = new Map<string, Promise<string>>()
const loadedIds = new Set<string>()
let autoNextLocked = false
let backgroundPrefetchVersion = 0

const enid = computed(() => String(route.params.id ?? '').trim())
const queryTitle = computed(() => String(route.query.title ?? '').trim())
const displayTitle = computed(() => queryTitle.value || `电子书 ${enid.value}`)
const notesStorageKey = computed(() => `ebook_reader_notes_v1_${enid.value}`)
const activeDisplayIndex = computed(() => {
  const idx = chapters.value.findIndex(c => c.chapterId === activeChapterId.value)
  return idx >= 0 ? idx + 1 : 0
})
const hasMoreChapters = computed(() => {
  if (contentBlocks.value.length === 0) return false
  const lastId = contentBlocks.value[contentBlocks.value.length - 1].chapterId
  const lastIdx = chapters.value.findIndex(c => c.chapterId === lastId)
  return lastIdx < chapters.value.length - 1
})

const getChapterKey = (raw: string) => String(raw || '').split('#')[0].trim()
const getLastKey = (id: string) => `ebook_reader_last_chapter_${id}`
const normalizeText = (text: string) => String(text || '').replace(/\s+/g, ' ').trim()
const chapterTitleById = (id: string) => chapters.value.find((c) => c.chapterId === id)?.title || id
const makeNoteId = () => `n_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
const formatNoteTime = (ts: number) => {
  const d = new Date(ts)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
const cssEscape = (v: string) => {
  if (typeof (window as any).CSS !== 'undefined' && typeof (window as any).CSS.escape === 'function') {
    return (window as any).CSS.escape(v)
  }
  return String(v).replace(/["\\]/g, '\\$&')
}

const loadReaderNotes = () => {
  const data = Local.get(notesStorageKey.value)
  const list = Array.isArray(data) ? data : []
  readerNotes.value = list
    .map((item: any) => ({
      id: String(item?.id || ''),
      remoteId: String(item?.remoteId || item?.id || ''),
      synced: Boolean(item?.synced ?? item?.remoteId),
      chapterId: String(item?.chapterId || ''),
      chapterTitle: String(item?.chapterTitle || ''),
      quote: String(item?.quote || ''),
      comment: String(item?.comment || ''),
      createdAt: Number(item?.createdAt || Date.now()),
    }))
    .filter((item: ReaderNote) => item.id && item.chapterId && item.quote)
}

const saveReaderNotes = () => {
  Local.set(notesStorageKey.value, readerNotes.value)
}

const fromServerSeconds = (value: any) => {
  const n = Number(value || 0)
  if (!n) return Date.now()
  return n > 1e12 ? n : n * 1000
}

const mapSyncedNote = (item: EbookSyncedNote): ReaderNote | null => {
  const remoteId = String(item?.note_id_hazy || '').trim()
  const quote = normalizeText(String(item?.note_line || item?.note || ''))
  if (!quote) return null
  const chapterId = getChapterKey(String(item?.extra?.book_section || item?.extra?.location || ''))
  return {
    id: remoteId || makeNoteId(),
    remoteId,
    synced: Boolean(remoteId),
    chapterId,
    chapterTitle: chapterId ? chapterTitleById(chapterId) : '未知章节',
    quote,
    comment: normalizeText(String(item?.note || '')),
    createdAt: fromServerSeconds(item?.update_time || item?.create_time),
  }
}

const loadSyncedNotes = async (localCache: ReaderNote[]) => {
  try {
    const result = await invokeBackend<{ list?: EbookSyncedNote[] }>('EbookSyncedNotes', enid.value)
    const rawList = Array.isArray(result?.list) ? result!.list! : []
    if (rawList.length === 0) return false

    const remoteNotes = rawList
      .map(mapSyncedNote)
      .filter((item): item is ReaderNote => Boolean(item))

    const unsyncedLocal = (localCache || []).filter((item) => !item.synced)
    const remoteIdSet = new Set(remoteNotes.map((item) => item.id))
    for (const local of unsyncedLocal) {
      if (!remoteIdSet.has(local.id)) {
        remoteNotes.unshift(local)
      }
    }
    remoteNotes.sort((a, b) => b.createdAt - a.createdAt)
    readerNotes.value = remoteNotes
    saveReaderNotes()
    return true
  } catch {
    return false
  }
}

const copyText = async (text: string) => {
  const value = String(text || '').trim()
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    return
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.style.position = 'fixed'
    textarea.style.left = '-9999px'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
}

const clearSelection = () => {
  const s = window.getSelection()
  if (s) s.removeAllRanges()
}

const hideSelectionMenu = () => {
  selectionMenu.value.visible = false
}

const getChapterIdFromNode = (node: Node | null): string => {
  const el = node instanceof Element ? node : node?.parentElement
  if (!el) return ''
  const chapterEl = el.closest('.chapter-block')
  const id = String(chapterEl?.id || '')
  return id.startsWith('ch-') ? id.slice(3) : ''
}

const getSelectionContext = () => {
  const host = contentRef.value
  const sel = window.getSelection()
  if (!host || !sel || sel.rangeCount === 0 || sel.isCollapsed) return null
  const range = sel.getRangeAt(0)
  if (!host.contains(range.commonAncestorContainer)) return null
  const chapterId = getChapterIdFromNode(range.commonAncestorContainer)
  if (!chapterId) return null
  const quote = normalizeText(sel.toString())
  if (!quote || quote.length < 2) return null
  const rect = range.getBoundingClientRect()
  if (!rect || (rect.width === 0 && rect.height === 0)) return null
  return { range: range.cloneRange(), chapterId, quote, rect }
}

const updateSelectionMenu = () => {
  const ctx = getSelectionContext()
  if (!ctx) {
    hideSelectionMenu()
    return
  }
  selectionMenu.value.visible = true
  selectionMenu.value.quote = ctx.quote
  selectionMenu.value.chapterId = ctx.chapterId
  selectionMenu.value.x = Math.max(80, Math.min(window.innerWidth - 80, ctx.rect.left + ctx.rect.width / 2))
  selectionMenu.value.y = Math.max(16, ctx.rect.top - 46)
}

const onReaderMouseUp = () => {
  window.setTimeout(updateSelectionMenu, 0)
}

const unwrapNoteMark = (noteId: string) => {
  const marks = document.querySelectorAll(`mark.reader-note-highlight[data-note-id="${cssEscape(noteId)}"]`)
  marks.forEach((mark) => {
    const parent = mark.parentNode
    if (!parent) return
    while (mark.firstChild) parent.insertBefore(mark.firstChild, mark)
    parent.removeChild(mark)
  })
}

const renameMarkId = (fromId: string, toId: string) => {
  if (!fromId || !toId || fromId === toId) return
  const marks = document.querySelectorAll(`mark.reader-note-highlight[data-note-id="${cssEscape(fromId)}"]`)
  marks.forEach((mark) => mark.setAttribute('data-note-id', toId))
}

const setActiveMark = (noteId: string) => {
  const all = document.querySelectorAll('mark.reader-note-highlight.active')
  all.forEach((item) => item.classList.remove('active'))
  const curr = document.querySelectorAll(`mark.reader-note-highlight[data-note-id="${cssEscape(noteId)}"]`)
  curr.forEach((item) => item.classList.add('active'))
}

const flashMark = (noteId: string) => {
  const curr = document.querySelectorAll(`mark.reader-note-highlight[data-note-id="${cssEscape(noteId)}"]`)
  curr.forEach((item) => {
    item.classList.add('flash')
    window.setTimeout(() => item.classList.remove('flash'), 1200)
  })
}

const wrapRangeWithMark = (range: Range, noteId: string) => {
  try {
    const mark = document.createElement('mark')
    mark.className = 'reader-note-highlight'
    mark.setAttribute('data-note-id', noteId)
    const fragment = range.extractContents()
    if (!normalizeText(fragment.textContent || '')) return false
    mark.appendChild(fragment)
    range.insertNode(mark)
    return true
  } catch {
    return false
  }
}

const locateQuoteInRoot = (root: HTMLElement, quote: string, noteId: string) => {
  const target = String(quote || '')
  if (!target) return false

  const walker = document.createTreeWalker(
    root,
    NodeFilter.SHOW_TEXT,
    {
      acceptNode(node) {
        const text = String(node.nodeValue || '')
        if (!normalizeText(text)) return NodeFilter.FILTER_REJECT
        const parent = (node as Text).parentElement
        if (parent?.closest('mark.reader-note-highlight')) return NodeFilter.FILTER_REJECT
        return NodeFilter.FILTER_ACCEPT
      },
    } as any,
  )

  const entries: Array<{ node: Text; start: number; end: number }> = []
  let full = ''
  let n = walker.nextNode() as Text | null
  while (n) {
    const text = String(n.nodeValue || '')
    entries.push({ node: n, start: full.length, end: full.length + text.length })
    full += text
    n = walker.nextNode() as Text | null
  }
  if (!entries.length) return false

  const idx = full.indexOf(target)
  if (idx < 0) return false
  const endIdx = idx + target.length
  const start = entries.find((item) => idx >= item.start && idx < item.end)
  const end = entries.find((item) => endIdx > item.start && endIdx <= item.end)
  if (!start || !end) return false

  const range = document.createRange()
  range.setStart(start.node, idx - start.start)
  range.setEnd(end.node, endIdx - end.start)
  return wrapRangeWithMark(range, noteId)
}

const restoreNotesForChapter = async (chapterId: string) => {
  await nextTick()
  if (!chapterId) return
  const chapterEl = document.getElementById(`ch-${chapterId}`)
  const root = chapterEl?.querySelector('.html-reader') as HTMLElement | null
  if (!root) return
  const notes = readerNotes.value.filter((item) => item.chapterId === chapterId)
  for (const note of notes) {
    const exists = root.querySelector(`mark.reader-note-highlight[data-note-id="${cssEscape(note.id)}"]`)
    if (exists) continue
    locateQuoteInRoot(root, note.quote, note.id)
  }
}

const createNoteBySelection = async (withComment: boolean) => {
  const ctx = getSelectionContext()
  if (!ctx) {
    hideSelectionMenu()
    return
  }

  let comment = ''
  if (withComment) {
    try {
      const { value } = await ElMessageBox.prompt('给这段划线写点笔记', '写笔记', {
        inputType: 'textarea',
        inputPlaceholder: '输入你的想法（可留空）',
        confirmButtonText: '保存',
        cancelButtonText: '取消',
      })
      comment = normalizeText(String(value || ''))
    } catch {
      return
    }
  }

  const localId = makeNoteId()
  const ok = wrapRangeWithMark(ctx.range, localId)
  if (!ok) {
    ElMessage({ message: '这段文本暂不支持直接划线，请缩短选择范围重试', type: 'warning' })
    hideSelectionMenu()
    clearSelection()
    return
  }

  const note: ReaderNote = {
    id: localId,
    remoteId: '',
    synced: false,
    chapterId: ctx.chapterId,
    chapterTitle: chapterTitleById(ctx.chapterId),
    quote: ctx.quote,
    comment,
    createdAt: Date.now(),
  }
  readerNotes.value.unshift(note)
  saveReaderNotes()
  activeNoteId.value = localId
  setActiveMark(localId)
  clearSelection()
  hideSelectionMenu()

  try {
    const synced = await invokeBackend<{ note_id_hazy?: string }>(
      'EbookSyncSave',
      enid.value,
      ctx.chapterId,
      ctx.quote,
      comment,
      '',
    )
    const remoteId = String(synced?.note_id_hazy || '').trim()
    if (remoteId) {
      renameMarkId(localId, remoteId)
      note.id = remoteId
      note.remoteId = remoteId
      note.synced = true
      if (activeNoteId.value === localId) {
        activeNoteId.value = remoteId
      }
      setActiveMark(remoteId)
      saveReaderNotes()
      ElMessage({ message: withComment ? '笔记已保存并同步' : '划线已保存并同步', type: 'success' })
      return
    }
  } catch {
    // Keep local note if sync fails.
  }

  saveReaderNotes()
  ElMessage({ message: '已仅本地保存（官方同步失败）', type: 'warning' })
}

const onMarkClick = (event: MouseEvent) => {
  const target = event.target as HTMLElement | null
  const mark = target?.closest('mark.reader-note-highlight') as HTMLElement | null
  if (!mark) return
  const noteId = String(mark.getAttribute('data-note-id') || '')
  if (!noteId) return
  activeNoteId.value = noteId
  setActiveMark(noteId)
  noteDrawerVisible.value = true
}

const locateNote = async (note: ReaderNote) => {
  if (!note?.id || !note?.chapterId) return
  if (!loadedIds.has(note.chapterId)) {
    await loadFromChapter(note.chapterId)
  }
  await nextTick()
  const mark = document.querySelector(`mark.reader-note-highlight[data-note-id="${cssEscape(note.id)}"]`) as HTMLElement | null
  if (mark) {
    mark.scrollIntoView({ behavior: 'smooth', block: 'center' })
    activeNoteId.value = note.id
    setActiveMark(note.id)
    flashMark(note.id)
  }
}

const removeNote = async (note: ReaderNote) => {
  try {
    await ElMessageBox.confirm('删除后不可恢复，确认删除这条笔记吗？', '删除笔记', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }

  const remoteId = String(note.remoteId || note.id || '').trim()
  if (note.synced && remoteId) {
    try {
      await invokeBackend('EbookSyncDelete', remoteId)
    } catch (error) {
      const msg = String((error as any)?.message || error || '删除失败')
      ElMessage({ message: `官方删除失败：${msg}`, type: 'warning', duration: 4200 })
      return
    }
  }

  readerNotes.value = readerNotes.value.filter((item) => item.id !== note.id)
  saveReaderNotes()
  unwrapNoteMark(note.id)
  if (activeNoteId.value === note.id) activeNoteId.value = ''
}

const copyNote = async (note: ReaderNote) => {
  const text = `《${displayTitle.value}》\n${note.chapterTitle}\n\n${note.quote}${note.comment ? `\n\n笔记：${note.comment}` : ''}`
  await copyText(text)
  ElMessage({ message: '笔记内容已复制', type: 'success' })
}

const buildKnowledgeNoteContent = (note: ReaderNote) => {
  const lines: string[] = []
  lines.push(`《${displayTitle.value}》${note.chapterTitle ? ` · ${note.chapterTitle}` : ''}`)
  lines.push('')
  lines.push(`“${note.quote}”`)
  if (note.comment) {
    lines.push('')
    lines.push(`笔记：${note.comment}`)
  }
  return lines.join('\n').trim()
}

const publishNoteToKnowledge = async (note?: ReaderNote) => {
  const target = note || readerNotes.value[0]
  if (!target) {
    ElMessage({ message: '还没有笔记可发布', type: 'warning' })
    return
  }
  if (publishingNoteId.value) return

  const content = buildKnowledgeNoteContent(target)
  if (content.length < 2) {
    ElMessage({ message: '笔记内容太短，无法发布', type: 'warning' })
    return
  }

  publishingNoteId.value = note ? target.id : '__first__'
  try {
    await invokeBackend('KnowledgeCreateNote', content, '')
    ElMessage({ message: '已发布到知识城邦', type: 'success' })
  } catch (error) {
    const msg = String((error as any)?.message || error || '发布失败')
    ElMessage({ message: `发布失败：${msg}`, type: 'warning', duration: 4200 })
  } finally {
    publishingNoteId.value = ''
  }
}

const openPublishPage = (note?: ReaderNote) => {
  const target = note || readerNotes.value[0]
  if (!target) {
    ElMessage({ message: '还没有笔记可发布', type: 'warning' })
    return
  }
  const content = buildKnowledgeNoteContent(target)
  pushByName(ROUTE_NAMES.KNOWLEDGE_PUBLISH, {}, { content })
}

const copyAndGoKnowledge = async (note?: ReaderNote) => {
  const target = note || readerNotes.value[0]
  if (!target) {
    ElMessage({ message: '还没有笔记可发布', type: 'warning' })
    return
  }
  await copyNote(target)
  pushByName(ROUTE_NAMES.KNOWLEDGE)
}



const buildChapters = (info: EbookReadInfoResp) => {
  const orders = Array.isArray(info?.bookInfo?.orders) ? info.bookInfo!.orders! : []
  const toc = Array.isArray(info?.bookInfo?.toc) ? info.bookInfo!.toc! : []
  const tocMap = new Map<string, string>()
  for (const item of toc) {
    const key = getChapterKey(item?.href || '')
    if (key && !tocMap.has(key)) tocMap.set(key, String(item?.text || '').trim() || key)
  }
  const list: ReaderChapter[] = []
  const seen = new Set<string>()
  for (const item of orders) {
    const id = String(item?.chapterId || '').trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    list.push({ chapterId: id, title: tocMap.get(id) || id })
  }
  return list
}

/** Fetch HTML content for a chapter (with cache) */
const fetchHtml = async (chapterId: string): Promise<string> => {
  if (htmlCache.has(chapterId)) return htmlCache.get(chapterId)!
  const pending = htmlPending.get(chapterId)
  if (pending) return pending

  const task = invokeBackend<string>('EbookChapterHtml', enid.value, chapterId)
    .then((content) => {
      const htmlStr = typeof content === 'string' ? content : ''
      htmlCache.set(chapterId, htmlStr)
      return htmlStr
    })
    .finally(() => {
      htmlPending.delete(chapterId)
    })
  htmlPending.set(chapterId, task)
  return task
}

const sleep = (ms: number) => new Promise<void>((resolve) => window.setTimeout(resolve, ms))

/**
 * Background prefetch chapters after current one.
 * This keeps scrolling continuous without waiting on network at chapter boundary.
 */
const startBackgroundPrefetch = (currentChapterId: string) => {
  const currentIdx = chapters.value.findIndex(c => c.chapterId === currentChapterId)
  if (currentIdx < 0) return
  backgroundPrefetchVersion += 1
  const version = backgroundPrefetchVersion

  const run = async () => {
    for (let i = currentIdx + 1; i < chapters.value.length; i++) {
      if (version !== backgroundPrefetchVersion) return
      const nextId = chapters.value[i].chapterId
      if (!nextId || htmlCache.has(nextId)) continue
      try {
        await fetchHtml(nextId)
      } catch {
        // Ignore background prefetch errors; foreground loading will retry when needed.
      }
      await sleep(80)
    }
  }

  void run()
}

/** Append a chapter to the content stream without clearing existing content */
const appendChapter = async (chapterId: string) => {
  if (loadedIds.has(chapterId) || loadingPages.value) return
  const chapter = chapters.value.find(c => c.chapterId === chapterId)
  if (!chapter) return

  loadingPages.value = true
  errorText.value = ''
  try {
    const html = await fetchHtml(chapterId)
    if (html.trim().length > 0) {
      contentBlocks.value.push({ chapterId, title: chapter.title, html })
      loadedIds.add(chapterId)
      activeChapterId.value = chapterId
      Local.set(getLastKey(enid.value), chapterId)
      await restoreNotesForChapter(chapterId)
    }
  } catch (error) {
    errorText.value = String((error as any)?.message || error || '章节加载失败')
    ElMessage({ message: errorText.value, type: 'warning' })
  } finally {
    loadingPages.value = false
    autoNextLocked = false
  }
}

/** Load a chapter as starting point (clears stream) */
const loadFromChapter = async (chapterId: string) => {
  contentBlocks.value = []
  loadedIds.clear()
  activeChapterId.value = chapterId
  await appendChapter(chapterId)
  await nextTick()
  if (contentRef.value) contentRef.value.scrollTop = 0
  startBackgroundPrefetch(chapterId)
}

const fetchReaderInfo = async () => {
  if (!enid.value) { errorText.value = '缺少电子书 enid'; return }
  loadReaderNotes()
  const localCache = [...readerNotes.value]
  loadingInfo.value = true
  errorText.value = ''
  try {
    const info = await invokeBackend<EbookReadInfoResp>('EbookReadInfo', enid.value)
    const list = buildChapters(info)
    chapters.value = list
    if (list.length > 0) {
      await loadSyncedNotes(localCache)
      readerNotes.value = readerNotes.value.map((item) => ({
        ...item,
        chapterTitle: item.chapterId ? chapterTitleById(item.chapterId) : item.chapterTitle,
      }))
      saveReaderNotes()
    }
    if (list.length === 0) return
    const last = String(Local.get(getLastKey(enid.value)) || '').trim()
    const start = list.some(c => c.chapterId === last) ? last : list[0].chapterId
    await loadFromChapter(start)
  } catch (error) {
    errorText.value = String((error as any)?.message || error || '阅读信息加载失败')
    ElMessage({ message: errorText.value, type: 'warning' })
  } finally {
    loadingInfo.value = false
  }
}

/** Jump to a chapter from TOC */
const jumpToChapter = async (id: string) => {
  if (loadedIds.has(id)) {
    // Already in stream — smooth scroll to it
    const el = document.getElementById('ch-' + id)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' })
      activeChapterId.value = id
      startBackgroundPrefetch(id)
      return
    }
  }
  await loadFromChapter(id)
}

const goPrevChapter = () => {
  const idx = chapters.value.findIndex(c => c.chapterId === activeChapterId.value)
  if (idx > 0) jumpToChapter(chapters.value[idx - 1].chapterId)
}
const goNextChapter = () => {
  const idx = chapters.value.findIndex(c => c.chapterId === activeChapterId.value)
  if (idx >= 0 && idx < chapters.value.length - 1) jumpToChapter(chapters.value[idx + 1].chapterId)
}
const reloadCurrentChapter = async () => {
  if (!activeChapterId.value) { await fetchReaderInfo(); return }
  htmlCache.delete(activeChapterId.value)
  await loadFromChapter(activeChapterId.value)
}
const goBack = () => pushEbookList()

/** Detect visible chapter + auto-load next on scroll bottom */
const onContentScroll = () => {
  const el = contentRef.value
  if (!el) return
  hideSelectionMenu()

  // Progress bar
  const maxScroll = el.scrollHeight - el.clientHeight
  readProgress.value = maxScroll > 0 ? Math.round((el.scrollTop / maxScroll) * 100) : 0

  // Detect which chapter block is topmost
  for (const block of contentBlocks.value) {
    const div = document.getElementById('ch-' + block.chapterId)
    if (div && div.offsetTop <= el.scrollTop + 100) {
      activeChapterId.value = block.chapterId
    }
  }

  // Auto-append next chapter near bottom
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 300 && !autoNextLocked && !loadingPages.value) {
    autoNextLocked = true
    const lastBlock = contentBlocks.value[contentBlocks.value.length - 1]
    if (lastBlock) {
      const lastIdx = chapters.value.findIndex(c => c.chapterId === lastBlock.chapterId)
      if (lastIdx >= 0 && lastIdx < chapters.value.length - 1) {
        appendChapter(chapters.value[lastIdx + 1].chapterId)
      }
    }
  }
}

const handleKeydown = (e: KeyboardEvent) => {
  if ((e.target as HTMLElement)?.tagName === 'INPUT' || (e.target as HTMLElement)?.tagName === 'TEXTAREA') return
  if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(e.key)) e.preventDefault()
  if (e.key === 'ArrowLeft') goPrevChapter()
  else if (e.key === 'ArrowRight') goNextChapter()
  else if (e.key === 'ArrowUp') contentRef.value?.scrollBy({ top: -window.innerHeight * 0.8, behavior: 'smooth' })
  else if (e.key === 'ArrowDown') contentRef.value?.scrollBy({ top: window.innerHeight * 0.8, behavior: 'smooth' })
}

onMounted(async () => {
  await fetchReaderInfo()
  window.addEventListener('keydown', handleKeydown)
})
onUnmounted(() => {
  backgroundPrefetchVersion += 1
  hideSelectionMenu()
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<style scoped>
.ebook-reader-page {
  padding: 0;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-color);
}

.reader-header {
  position: relative;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 16px;
  height: 46px;
  background: color-mix(in srgb, var(--card-bg) 96%, transparent);
  border-bottom: 1px solid var(--border-soft);
  flex-shrink: 0;
  z-index: 10;
}
.header-left, .header-right { display: flex; align-items: center; gap: 4px; }
.header-center { display: flex; align-items: center; gap: 10px; font-size: 14px; color: var(--text-primary); font-weight: 500; }
.book-title { max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.chapter-info { font-size: 12px; color: var(--text-tertiary); background: color-mix(in srgb, var(--fill-color-light) 60%, transparent); padding: 2px 8px; border-radius: 10px; }

.progress-bar {
  position: absolute; bottom: -1px; left: 0; height: 2px;
  background: linear-gradient(90deg, var(--accent-color), var(--primary-color));
  transition: width 0.15s ease-out; border-radius: 0 1px 1px 0; z-index: 20;
}

.reader-layout {
  flex: 1; display: grid; grid-template-columns: 240px 1fr;
  gap: 0; height: calc(100vh - 46px); overflow: hidden;
  transition: grid-template-columns 0.25s ease;
}
.reader-layout.toc-collapsed { grid-template-columns: 0 1fr; }

.chapter-panel {
  background: color-mix(in srgb, var(--card-bg) 92%, transparent);
  border-right: 1px solid var(--border-soft);
  display: flex; flex-direction: column; min-height: 0; overflow: hidden;
}
.chapter-panel-header { padding: 12px 14px 8px; border-bottom: 1px solid color-mix(in srgb, var(--border-soft) 60%, transparent); flex-shrink: 0; }
.chapter-panel-header h3 { margin: 0; color: var(--text-primary); font-size: 14px; font-weight: 600; }
.chapter-list { flex: 1; min-height: 0; overflow-y: auto; padding: 6px; display: flex; flex-direction: column; gap: 2px; }

.chapter-item {
  width: 100%; border: 1px solid transparent; border-radius: 8px; background: transparent;
  padding: 6px 8px; display: grid; grid-template-columns: 22px 1fr; gap: 6px;
  align-items: center; text-align: left; cursor: pointer;
  color: var(--text-secondary); font-size: 13px; transition: all 0.15s ease;
}
.chapter-item:hover { background: color-mix(in srgb, var(--fill-color-light) 72%, transparent); }
.chapter-item.active { background: color-mix(in srgb, var(--accent-color) 12%, transparent); color: var(--accent-color); font-weight: 500; }
.chapter-index { color: var(--text-tertiary); font-size: 11px; text-align: center; }
.chapter-title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* ── Reader Content ── */
.reader-content {
  background: #ffffff;
  overflow-y: auto;
  overflow-x: hidden;
  scroll-behavior: auto;
}

.page-stream {
  width: min(calc(100% - clamp(12px, 2vw, 44px)), clamp(980px, 92vw, 1680px));
  max-width: none;
  margin: 0 auto;
  padding: 0;
}

.chapter-block {
  margin-bottom: 2px;
}

.chapter-divider {
  text-align: center;
  font-size: 12px;
  color: var(--text-tertiary);
  padding: 16px 0 8px;
  letter-spacing: 1px;
}

.page-img {
  width: 100%;
  height: auto;
  display: block;
}

.html-reader {
  font-size: 17px;
  line-height: 1.8;
  color: var(--text-primary);
  padding: 0 clamp(12px, 1.8vw, 28px);
}

.html-reader :deep(p) {
  margin-bottom: 1em;
  text-align: left;
}

.html-reader :deep(img) {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 1em auto;
  border-radius: 4px;
}

.html-reader :deep(sup) {
  color: var(--accent-color);
  font-size: 0.8em;
}

.reader-empty, .reader-error {
  min-height: 420px; display: flex; align-items: center; justify-content: center;
}

.loading-hint {
  text-align: center; padding: 24px 0; color: var(--text-tertiary); font-size: 14px;
}

.chapter-end-hint {
  text-align: center; padding: 24px 0 48px; color: var(--text-tertiary); font-size: 13px; letter-spacing: 0.5px;
}

.selection-toolbar {
  position: fixed;
  z-index: 2200;
  transform: translate(-50%, -100%);
  display: inline-flex;
  gap: 6px;
  padding: 6px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 84%, transparent);
  background: color-mix(in srgb, var(--card-bg) 96%, transparent);
  box-shadow: 0 8px 20px rgba(12, 18, 28, 0.18);
}

.note-drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
  color: var(--text-secondary);
  font-size: 12px;
}

.note-drawer-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.note-empty {
  margin-top: 24px;
}

.note-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.note-item {
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  background: color-mix(in srgb, var(--fill-color-light) 52%, transparent);
  border-radius: 10px;
  padding: 10px;
}

.note-item.active {
  border-color: color-mix(in srgb, var(--accent-color) 42%, transparent);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent-color) 26%, transparent) inset;
}

.note-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.note-meta-right {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.note-chapter {
  font-size: 12px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.note-time {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--text-tertiary);
}

.note-sync {
  flex-shrink: 0;
  font-size: 11px;
  line-height: 1;
  color: var(--el-color-warning);
  border: 1px solid color-mix(in srgb, var(--el-color-warning) 45%, transparent);
  border-radius: 10px;
  padding: 2px 6px;
}

.note-quote {
  margin-top: 8px;
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.6;
}

.note-comment {
  margin-top: 8px;
  padding: 8px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--card-bg) 80%, transparent);
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.note-actions {
  margin-top: 8px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.html-reader :deep(mark.reader-note-highlight) {
  background: color-mix(in srgb, #ffd15c 55%, transparent);
  color: inherit;
  border-radius: 3px;
  padding: 0 1px;
  cursor: pointer;
}

.html-reader :deep(mark.reader-note-highlight.active) {
  background: color-mix(in srgb, var(--accent-color) 30%, #fff1a8 70%);
}

.html-reader :deep(mark.reader-note-highlight.flash) {
  animation: noteFlash 1.2s ease;
}

@keyframes noteFlash {
  0% {
    background: color-mix(in srgb, #ffba1f 75%, transparent);
  }
  100% {
    background: color-mix(in srgb, #ffd15c 55%, transparent);
  }
}

@media (max-width: 860px) {
  .reader-layout { grid-template-columns: 1fr; }
  .chapter-panel { display: none; }
}
</style>
