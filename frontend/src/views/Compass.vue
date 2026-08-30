<template>
  <div class="compass-container">
    <section class="compass-hero">
      <div class="hero-main">
        <p class="hero-kicker">Compass</p>
        <h1 class="hero-title">锦囊精选</h1>
        <p class="hero-subtitle">聚合实用问题与建议，支持快速筛选与一键下载。</p>
      </div>
      <div class="hero-stats">
        <article class="stat-card">
          <span>内容总量</span>
          <strong>{{ total }}</strong>
        </article>
        <article class="stat-card">
          <span>当前页</span>
          <strong>{{ page }}</strong>
        </article>
        <article class="stat-card">
          <span>每页数量</span>
          <strong>{{ pageSize }}</strong>
        </article>
      </div>
    </section>

    <div class="compass-grid" v-loading="loading">
      <div v-for="item in tableData.list" :key="item.id" class="compass-card">
        <div class="card-inner">
          <div class="card-cover" v-if="item.icon">
            <el-image :src="item.icon" loading="lazy" fit="cover">
              <template #placeholder>
                <div class="image-placeholder">
                  <el-icon><Picture /></el-icon>
                </div>
              </template>
            </el-image>
            <button class="download-fab" @click.stop="openDownloadDialog(item)">
              <el-icon><DownloadIcon /></el-icon>
            </button>
          </div>
          <div class="card-info">
            <h3 class="card-title" :title="item.title">{{ item.title }}</h3>
            <div class="card-meta">
              <span class="replier" v-if="item.ext_info && item.ext_info[0]">
                <el-icon><User /></el-icon>
                {{ item.ext_info[0].replier_name }}
              </span>
              <span class="replier" v-else>
                <el-icon><User /></el-icon>
                匿名用户
              </span>
            </div>
            <div class="card-intro" v-if="item.intro">
              <p>{{ item.intro }}</p>
            </div>
            <div class="card-actions">
              <el-button type="primary" plain size="small" @click.stop="openDownloadDialog(item)">
                下载
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </div>
    
    <div class="pagination-container">
      <Pagination :total="total" @pageChange="handleChangePage"></Pagination>
    </div>

    <!-- Download Dialog -->
    <el-dialog v-model="dialogDownloadVisible" title="下载内容" width="30%" align-center class="custom-dialog">
      <div class="dialog-content">
        <el-form label-position="top">
          <el-form-item label="选择格式">
            <el-select v-model="downloadType" placeholder="请选择下载格式" style="width: 100%">
              <el-option
                v-for="item in downloadTypeOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="closeDownloadDialog">取消</el-button>
          <el-button type="primary" @click="download(downloadId, downloadType)">
            开始下载
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { CourseList, CourseCategory, CourseDownload } from '../../wailsjs/go/backend/App'
import { services } from '../../wailsjs/go/models'
import { useRouter } from 'vue-router'
import { userStore } from '../stores/user';
import Pagination from '../components/Pagination.vue'
import { Local } from '../utils/storage';
import { Download as DownloadIcon, User, Picture } from '@element-plus/icons-vue'

const store = userStore()
const router = useRouter()

const loading = ref(true)
const page = ref(1)
const total = ref(0)
const pageSize = ref(15)
const dialogVisible = ref(false)


const dialogDownloadVisible = ref(false)
const downloadType = ref(1)
const downloadId = ref(0)

const downloadTypeOptions = [
    { value: 1, label: "MP3" }, { value: 2, label: "PDF" }, { value: 3, label: "Markdown" }
]
let tableData = reactive(new services.CourseList)

onMounted(() => {
    CourseCategory().then(result => {
        result.forEach((item, key) => {
            if (item.category == "compass") {
                total.value = item.count
            }
        })
    }).catch((error) => {
        if (error == '401 Unauthorized') {
            store.user = null
            router.push("/user/login")
        }
        Local.remove("cookies")
        Local.remove("userStore")
    })
})

// 分页
const handleChangePage = (item: any) => {
    page.value = item.page
    pageSize.value = item.pageSize
    getTableData()
}


const getTableData = async () => {
    await CourseList("compass", "study", "all", page.value, pageSize.value).then((table) => {
        loading.value = false
        Object.assign(tableData, table)
        console.log(tableData)
    }).catch((error) => {
        loading.value = false
        ElMessage({
            message: error,
            type: 'warning'
        })
    })
}

getTableData()


const openDialog = () => {
    dialogVisible.value = true
}
const closeDialog = () => {
    //   initForm()
    dialogVisible.value = false
}

const openDownloadDialog = (row: any) => {
    downloadId.value = row.id
    dialogDownloadVisible.value = true
}
const closeDownloadDialog = () => {
    //   initForm()
    downloadType.value = 1
    dialogDownloadVisible.value = false
}

const download = async (id: number, dType: number) => {
    await CourseDownload(id, 0, dType,'').then((info) => {
        console.log(info)
        ElMessage({
            message: '任务已添加到下载队列',
            type: 'success'
        })
    }).catch((error) => {
        ElMessage({
            message: error,
            type: 'warning'
        })
    })
    closeDownloadDialog()
    return
}
</script>
  
<style scoped>
.compass-container {
  min-height: calc(100vh - 60px);
  display: flex;
  flex-direction: column;
  gap: 14px;
  background-color: var(--fill-color-light);
  padding: 14px;
  box-sizing: border-box;
}

.compass-hero {
  display: grid;
  grid-template-columns: 1fr 320px;
  gap: 12px;
  padding: 16px;
  border-radius: 14px;
  border: 1px solid var(--border-soft);
  background:
    radial-gradient(320px 180px at 14% 0%, color-mix(in srgb, var(--primary-color) 16%, transparent) 0%, transparent 72%),
    color-mix(in srgb, var(--card-bg) 92%, transparent);
}

.hero-kicker {
  margin: 0;
  color: var(--accent-color);
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-weight: 700;
}

.hero-title {
  margin: 8px 0 0;
  font-size: 28px;
  color: var(--text-primary);
  font-family: var(--font-family-display);
}

.hero-subtitle {
  margin: 10px 0 0;
  color: var(--text-secondary);
  font-size: 14px;
}

.hero-stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  align-content: start;
}

.stat-card {
  border: 1px solid var(--border-soft);
  border-radius: 10px;
  padding: 10px;
  background: color-mix(in srgb, var(--fill-color-light) 76%, transparent);
}

.stat-card span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.stat-card strong {
  display: block;
  margin-top: 4px;
  font-size: 18px;
  color: var(--text-primary);
}

.compass-grid {
  flex: 1;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 14px;
  overflow-y: auto;
  padding: 2px;
  align-content: start;
  
  /* 隐藏滚动条但保留功能 - 清新风格 */
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE 10+ */
}

.compass-grid::-webkit-scrollbar {
  display: none; /* Chrome/Safari */
}

.compass-card {
  background: var(--card-bg);
  border-radius: 14px;
  box-shadow: var(--shadow-soft);
  transition: transform 0.24s ease, box-shadow 0.24s ease, border-color 0.24s ease;
  overflow: hidden;
  height: 100%;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border-soft);
}

.compass-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-medium);
  border-color: color-mix(in srgb, var(--primary-color) 32%, transparent);
}

.card-inner {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.card-cover {
  position: relative;
  height: 180px;
  background: var(--fill-color);
  overflow: hidden;
}

.card-cover .el-image {
  width: 100%;
  height: 100%;
  transition: transform 0.4s ease;
}

.compass-card:hover .card-cover .el-image {
  transform: scale(1.03);
}

.image-placeholder {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  color: var(--text-secondary);
  font-size: 24px;
}

.download-fab {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 32px;
  height: 32px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.28);
  background: rgba(229, 81, 0, 0.92);
  color: #fff;
  display: flex;
  justify-content: center;
  align-items: center;
  cursor: pointer;
  transition: transform 0.2s ease, background-color 0.2s ease;
  backdrop-filter: blur(4px);
}

.download-fab:hover {
  transform: translateY(-1px);
  background: #d94800;
}

.card-info {
  padding: 14px;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.card-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 64px;
}

.card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: var(--text-secondary);
}

.replier {
  display: flex;
  align-items: center;
  gap: 4px;
}

.card-intro {
  margin-top: auto;
  font-size: 13px;
  color: var(--text-tertiary);
  line-height: 1.5;
}

.card-intro p {
  margin: 0;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-actions {
  margin-top: 8px;
  display: flex;
  justify-content: flex-end;
}

.pagination-container {
  display: flex;
  justify-content: center;
  background: var(--card-bg);
  padding: 12px;
  border-radius: 8px;
  box-shadow: var(--shadow-soft);
}

@media (max-width: 1180px) {
  .compass-hero {
    grid-template-columns: 1fr;
  }

  .hero-stats {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .compass-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 860px) {
  .compass-container {
    padding: 10px;
  }

  .hero-title {
    font-size: 24px;
  }

  .hero-stats {
    grid-template-columns: 1fr 1fr;
  }

  .compass-grid {
    grid-template-columns: 1fr;
  }

  .card-title {
    font-size: 20px;
    min-height: auto;
  }
}

/* Dark mode adjustments */
.theme-dark .compass-card {
  background: var(--card-bg);
  border-color: var(--border-soft);
}

.theme-dark .card-title {
  color: var(--text-primary);
}

.theme-dark .card-meta {
  color: var(--text-secondary);
}

.theme-dark .card-intro {
  color: var(--text-tertiary);
}

/* Custom Dialog */
.custom-dialog {
  border-radius: 12px;
  overflow: hidden;
}

.dialog-content {
  padding: 10px 0;
}
</style>
