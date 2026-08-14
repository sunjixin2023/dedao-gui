<template>
    <el-dialog
        v-model="dialogVisible"
        title="下载选项"
        align-center
        center
        width="420px"
        :before-close="closeDialog"
        class="custom-download-dialog"
    >
        <div class="download-container">
            <div class="format-selector">
                <div class="section-label">选择导出格式</div>
                <el-radio-group v-model="downloadType" class="format-radio-group" :disabled="isForegroundActive">
                    <el-radio-button
                        v-for="item in props.downloadTypeOptions"
                        :key="item.value"
                        :label="item.value"
                    >
                        {{ item.label }}
                    </el-radio-button>
                </el-radio-group>
            </div>

            <div v-if="showStatus" class="download-status" :class="`is-${state}`">
                <div class="status-header">
                    <span class="status-text">{{ content }}</span>
                    <span class="status-percent">{{ percentage }}%</span>
                </div>
                <el-progress
                    :percentage="percentage"
                    :stroke-width="8"
                    :show-text="false"
                    :status="progressStatus"
                    class="custom-progress"
                />
                <div class="status-meta">{{ stateLabel }}</div>
                <div v-if="errorDetail" class="status-detail">{{ errorDetail }}</div>
            </div>
        </div>

        <template #footer>
            <div class="dialog-footer">
                <el-button @click="handleSecondaryAction" :disabled="cancelPending || isForegroundActive">
                    {{ secondaryActionLabel }}
                </el-button>
                <el-button
                    v-if="showPrimaryAction"
                    type="primary"
                    @click="download"
                    :loading="isStarting"
                    :disabled="isForegroundActive || cancelPending"
                >
                    {{ primaryActionLabel }}
                </el-button>
                <el-button
                    v-if="isForegroundActive || cancelPending"
                    type="danger"
                    plain
                    @click="cancelDownload"
                    :loading="cancelPending"
                >
                    取消下载
                </el-button>
            </div>
        </template>
    </el-dialog>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, PropType, ref, watch } from "vue";
import { CancelDownload, CourseDownload, EbookDownload, OdobDownload } from "../../wailsjs/go/backend/App";
import { ElMessage } from "element-plus";
import { EventsOn } from "../../wailsjs/runtime/runtime";

type DownloadState = 'queued' | 'downloading' | 'verifying' | 'completed' | 'failed' | 'cancelled'

type DownloadProgressEvent = {
    id?: number | string
    pct?: number
    value?: string
    state?: DownloadState
    detail?: string
}

const percentage = ref(0)
const content = ref('')
const errorDetail = ref('')
const state = ref<DownloadState>('queued')
const dialogVisible = ref(false)
const downloadType = ref(1)
const isStarting = ref(false)
const cancelPending = ref(false)
const deferredExternalHide = ref(false)

let removeEventListener: (() => void) | null = null
let activeEventName = ''

const props = defineProps({
    downloadId: {
        type: Number,
        required: true,
        default: 0,
    },
    enId: {
        type: String,
        default: '',
    },
    prodType: {
        type: Number,
        required: true,
        default: 0,
    },
    articleId: {
        type: Number,
        default: 0,
    },
    dialogVisible: {
        type: Boolean,
        default: false,
    },
    downloadTypeOptions: {
        type: Array as PropType<Array<{ value: number; label: string }>>,
        required: true,
        default: () => []
    },
    downloadData: {
        type: Object,
        default: () => ({})
    }
});

const emits = defineEmits(["close"]);

const isForegroundActive = computed(() => isStarting.value || cancelPending.value)

const showStatus = computed(() => {
    return content.value !== '' || percentage.value > 0 || state.value !== 'queued' || errorDetail.value !== ''
})

const stateLabel = computed(() => {
    switch (state.value) {
        case 'queued':
            return '等待开始'
        case 'downloading':
            return '下载中'
        case 'verifying':
            return '处理中'
        case 'completed':
            return '已完成'
        case 'failed':
            return '下载失败'
        case 'cancelled':
            return '已取消'
    }
})

const progressStatus = computed(() => {
    return state.value === 'failed' ? 'exception' : state.value === 'completed' ? 'success' : undefined
})

const primaryActionLabel = computed(() => {
    if (state.value === 'failed') {
        return '重试下载'
    }
    if (state.value === 'cancelled') {
        return '重新开始'
    }
    return '开始下载'
})

const secondaryActionLabel = computed(() => '取消')

const showPrimaryAction = computed(() => state.value !== 'completed')

const openDialog = () => {
    dialogVisible.value = props.dialogVisible
    if (props.downloadTypeOptions.length > 0) {
        downloadType.value = props.downloadTypeOptions[0].value
    }
    resetDialogState()
}

const resetDialogState = () => {
    percentage.value = 0
    content.value = ''
    errorDetail.value = ''
    state.value = 'queued'
    isStarting.value = false
    cancelPending.value = false
    deferredExternalHide.value = false
}

const eventNameForProduct = () => {
    switch (props.prodType) {
        case 2:
            return 'ebookDownload'
        case 66:
            return 'courseDownload'
        case 3:
            return 'odobDownload'
        default:
            return ''
    }
}

const normalizeProgressId = (value: unknown) => {
    if (typeof value === 'number' && Number.isFinite(value)) {
        return String(value)
    }
    if (typeof value === 'string') {
        return value.trim()
    }
    return ''
}

const activeDownloadId = computed(() => normalizeProgressId(props.downloadId))

const matchesActiveDownload = (data?: DownloadProgressEvent) => {
    const eventId = normalizeProgressId(data?.id)
    const currentId = activeDownloadId.value

    if (!currentId) {
        return true
    }
    if (!eventId) {
        return false
    }
    return eventId === currentId
}

const detachProgressListener = () => {
    if (removeEventListener) {
        removeEventListener()
        removeEventListener = null
    }
    activeEventName = ''
}

const applyProgressEvent = (data?: DownloadProgressEvent) => {
    if (!data || !matchesActiveDownload(data)) {
        return
    }

    if (typeof data.pct === 'number') {
        percentage.value = Number(data.pct)
    }
    if (typeof data.detail === 'string') {
        errorDetail.value = data.detail
    }

    const nextState = data.state ?? state.value
    state.value = nextState

    if (typeof data.value === 'string' && data.value.trim() !== '') {
        if (nextState === 'downloading') {
            content.value = `${data.value} 下载中...`
        } else {
            content.value = data.value
        }
    } else {
        switch (nextState) {
            case 'queued':
                content.value = '准备下载...'
                break
            case 'downloading':
                content.value = '下载中...'
                break
            case 'verifying':
                content.value = '正在处理下载文件...'
                break
            case 'completed':
                content.value = '下载完成'
                break
            case 'failed':
                content.value = '下载失败'
                break
            case 'cancelled':
                content.value = '已取消，可稍后继续下载'
                break
        }
    }

    if (nextState === 'completed') {
        percentage.value = 100
    }
}

const safeFailureDetail = () => {
    return errorDetail.value || '下载失败，请检查下载目录和网络后重试'
}

const safeCancelFailureDetail = () => {
    return '取消下载失败，请稍后重试'
}

const attachProgressListener = () => {
    detachProgressListener()
    activeEventName = eventNameForProduct()
    if (!activeEventName) {
        return
    }
    removeEventListener = EventsOn(activeEventName, (data: DownloadProgressEvent) => {
        applyProgressEvent(data)
    })
}

const finalizeSuccessfulDownload = () => {
    applyProgressEvent({
        pct: 100,
        state: 'completed',
        value: content.value || '下载完成',
    })
    isStarting.value = false
    deferredExternalHide.value = false
    closeDialog()
}

const closeDialog = () => {
    if (isForegroundActive.value) {
        return
    }
    detachProgressListener()
    resetDialogState()
    emits("close")
}

const handleSecondaryAction = () => {
    if (isForegroundActive.value) {
        return
    }
    closeDialog()
}

const normalizeError = (error: unknown) => {
    if (error instanceof Error) {
        return error.message
    }
    return String(error)
}

const cancelDownload = async () => {
    if (!isForegroundActive.value || cancelPending.value) {
        return
    }

    cancelPending.value = true
    state.value = 'cancelled'
    content.value = '已取消，可稍后继续下载'

    try {
        await CancelDownload()
    } catch (error) {
        const rawMessage = normalizeError(error)
        console.warn('CancelDownload failed:', rawMessage)
        const message = safeCancelFailureDetail()
        state.value = 'failed'
        content.value = '取消下载失败'
        errorDetail.value = message
        ElMessage({
            message,
            type: 'warning'
        })
    } finally {
        cancelPending.value = false
    }
}

const download = async () => {
    attachProgressListener()
    state.value = 'queued'
    content.value = '准备下载...'
    errorDetail.value = ''
    percentage.value = 0
    isStarting.value = true

    try {
        switch (props.prodType) {
            case 2:
                await EbookDownload(props.downloadId, downloadType.value, props.enId)
                break
            case 66:
                await CourseDownload(props.downloadId, props.articleId, downloadType.value, props.enId)
                break
            case 3:
                await OdobDownload(props.downloadId, downloadType.value, props.downloadData as any)
                break
            default:
                throw new Error('不支持的下载类型')
        }

        finalizeSuccessfulDownload()
        return
    } catch (error) {
        const rawMessage = normalizeError(error)
        console.warn('Download failed:', rawMessage)

        const currentState = state.value as DownloadState
        if (currentState === 'cancelled') {
            errorDetail.value = errorDetail.value || '已取消，可稍后继续下载'
            content.value = content.value || '已取消，可稍后继续下载'
            return
        }

        state.value = 'failed'
        content.value = '下载失败'
        const message = safeFailureDetail()
        errorDetail.value = message
        ElMessage({
            message,
            type: 'warning'
        })
    } finally {
        isStarting.value = false
    }
}

watch(() => props.dialogVisible, (visible) => {
    if (visible) {
        deferredExternalHide.value = false
        dialogVisible.value = true
        return
    }

    if (isForegroundActive.value) {
        deferredExternalHide.value = true
        dialogVisible.value = true
        return
    }

    if (deferredExternalHide.value && (state.value === 'failed' || state.value === 'cancelled')) {
        dialogVisible.value = true
        return
    }

    dialogVisible.value = false
    detachProgressListener()
    resetDialogState()
})

onMounted(() => {
    openDialog()
})

onBeforeUnmount(() => {
    detachProgressListener()
})
</script>

<style scoped>
.download-container {
    padding: 10px 20px;
}

.format-selector {
    margin-bottom: 24px;
}

.section-label {
    font-size: 14px;
    color: var(--text-secondary, #606266);
    margin-bottom: 12px;
    font-weight: 500;
}

.format-radio-group {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
}

.download-status {
    margin-top: 20px;
    background: var(--fill-color-lighter, #fafafa);
    padding: 16px;
    border-radius: 8px;
    border: 1px solid var(--border-color-lighter, #ebeef5);
}

.download-status.is-failed {
    border-color: var(--el-color-danger-light-5);
    background: var(--el-color-danger-light-9);
}

.download-status.is-cancelled {
    border-color: var(--el-color-warning-light-5);
    background: var(--el-color-warning-light-9);
}

.download-status.is-completed {
    border-color: var(--el-color-success-light-5);
    background: var(--el-color-success-light-9);
}

.status-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 8px;
    font-size: 13px;
    gap: 12px;
}

.status-text {
    color: var(--text-regular, #606266);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 80%;
}

.status-percent {
    color: var(--primary-color, #409eff);
    font-weight: 600;
}

.status-meta {
    margin-top: 10px;
    font-size: 12px;
    color: var(--text-secondary, #909399);
}

.status-detail {
    margin-top: 8px;
    font-size: 12px;
    line-height: 1.5;
    color: var(--el-color-danger);
    word-break: break-word;
}

.dialog-footer {
    display: flex;
    justify-content: center;
    gap: 16px;
    padding-bottom: 8px;
}
</style>
