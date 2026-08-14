<template>
  <div class="home-container-wrapper">
    <div class="home-ambient home-ambient-left"></div>
    <div class="home-ambient home-ambient-right"></div>

    <section v-if="loading && !hasHomeContent" class="home-feedback-shell">
      <div class="home-feedback-panel">
        <el-skeleton animated class="home-skeleton">
          <template #template>
            <div class="home-skeleton-hero">
              <div class="el-skeleton__item skeleton-kicker"></div>
              <div class="el-skeleton__item skeleton-title"></div>
              <div class="el-skeleton__item skeleton-line"></div>
              <div class="el-skeleton__item skeleton-line skeleton-line-short"></div>
              <div class="home-skeleton-actions">
                <div class="el-skeleton__item skeleton-button"></div>
                <div class="el-skeleton__item skeleton-button"></div>
                <div class="el-skeleton__item skeleton-button"></div>
              </div>
            </div>
            <div class="home-skeleton-grid">
              <div class="home-skeleton-card" v-for="index in 6" :key="`skeleton-${index}`">
                <div class="el-skeleton__item skeleton-card-cover"></div>
                <div class="home-skeleton-card-body">
                  <div class="el-skeleton__item skeleton-card-title"></div>
                  <div class="el-skeleton__item skeleton-line"></div>
                  <div class="el-skeleton__item skeleton-line skeleton-line-short"></div>
                </div>
              </div>
            </div>
          </template>
        </el-skeleton>
      </div>
    </section>

    <section v-else-if="loadError && !hasHomeContent" class="home-feedback-shell">
      <div class="home-feedback-panel">
        <el-alert
          class="home-error-alert"
          type="error"
          :closable="false"
          show-icon
          title="首页数据加载失败"
          :description="loadError"
        />
        <el-result class="home-error-result" icon="error" title="暂时无法打开首页" :sub-title="loadError">
          <template #extra>
            <el-button type="primary" round :loading="loading" :disabled="loading" @click="loadHome">重新加载</el-button>
          </template>
        </el-result>
      </div>
    </section>

    <template v-else>
      <section v-if="loadError" class="home-inline-feedback">
        <el-alert type="warning" :closable="false" show-icon>
          <template #title>首页数据未完全刷新</template>
          <template #default>
            <div class="home-inline-feedback-body">
              <span>{{ loadError }}</span>
              <el-button type="primary" round size="small" :loading="loading" :disabled="loading" @click="loadHome">重新加载</el-button>
            </div>
          </template>
        </el-alert>
      </section>

    <section class="learning-deck">
      <div class="deck-main">
        <p class="deck-kicker">DEDao Learning Hub</p>
        <h1 class="deck-title">在一个界面中完成看、听、读</h1>
        <p class="deck-subtitle">
          文稿阅读、课程视频与音频连播统一在同一学习工作台，支持续学与快速跳转。
        </p>
        <div class="deck-actions">
          <el-button type="primary" round @click="goContinueLearning">
            {{ lStore.hasLastArticle ? "继续上次文稿" : "开始学习" }}
          </el-button>
          <el-button round @click="goToCourseList">看课程</el-button>
          <el-button round @click="goToLiveList">看直播</el-button>
          <el-button round @click="goToAudioList">听音频</el-button>
          <el-button round @click="goToEbookList">读电子书</el-button>
        </div>
      </div>
      <div class="deck-side">
        <div class="deck-side-head">
          <span class="deck-side-title">学习快照</span>
          <span class="deck-side-sub">实时同步</span>
        </div>
        <div class="deck-stats-row">
          <div class="deck-stat">
            <span>今日学习</span>
            <div class="deck-stat-value">
              <strong>{{ todayStudyMinutes }}</strong>
              <em>分钟</em>
            </div>
          </div>
          <div class="deck-stat">
            <span>连续学习</span>
            <div class="deck-stat-value">
              <strong>{{ studyStreakDays }}</strong>
              <em>天</em>
            </div>
          </div>
          <div class="deck-stat">
            <span>文稿进度</span>
            <strong>{{ articleProgressText }}</strong>
          </div>
        </div>
        <button
          v-if="pStore.hasTrack"
          class="deck-playing-btn"
          @click="openNowPlaying"
          :title="nowPlayingTitle"
        >
          <el-icon><Headset /></el-icon>
          <span>{{ nowPlayingTitle }}</span>
        </button>
      </div>
    </section>

    <section class="workspace-lanes">
      <button class="lane-card lane-card-course" @click="goToCourseList">
        <div class="lane-icon">
          <el-icon><VideoPlay /></el-icon>
        </div>
        <div class="lane-body">
          <p class="lane-kicker">COURSE TRACK</p>
          <h3>课程学习区</h3>
          <p>视频与文稿双轨推进，按章节高效回到上次进度。</p>
          <div class="lane-meta">当前推荐课程 {{ courseSpotlightCount }} 个</div>
        </div>
      </button>

      <button class="lane-card lane-card-audio" @click="goToAudioList">
        <div class="lane-icon">
          <el-icon><Headset /></el-icon>
        </div>
        <div class="lane-body">
          <p class="lane-kicker">AUDIO TRACK</p>
          <h3>听书学习区</h3>
          <p>连续播放不打断，通勤、运动、碎片时间都可进入学习节奏。</p>
          <div class="lane-meta">{{ audioLaneSummary }}</div>
        </div>
      </button>

      <button class="lane-card lane-card-ebook" @click="goToEbookList">
        <div class="lane-icon">
          <el-icon><Reading /></el-icon>
        </div>
        <div class="lane-body">
          <p class="lane-kicker">EBOOK TRACK</p>
          <h3>电子书学习区</h3>
          <p>沉浸式阅读与书评互动结合，形成从输入到复盘的阅读闭环。</p>
          <div class="lane-meta">当前推荐电子书 {{ ebookSpotlightCount }} 本</div>
        </div>
      </button>
    </section>

    <!-- 顶部区域：分类菜单 + 轮播图 + 用户信息 -->
    <div class="top-section">
      <!-- 左侧分类菜单 -->
      <div class="menu-wrapper">
        <el-menu
          ref="menuRef"
          default-active=""
          class="classification-menu"
          :collapse="false"
          unique-opened
          :router="true"
          active-text-color="var(--accent-color)"
          :popper-class="'menu-popper'"
          @open="handleOpen"
          @close="handleClose"
          @mouseenter="onMenuEnter"
          @mouseleave="onMenuLeave"
        >
          <el-sub-menu
            :index="item.enid"
            v-for="(item, index) in categoryList"
            :key="item.enid"
            v-show="index < moreCategory"
            @mouseenter="handleMenuEnter(item.enid)"
          >
            <template #title>
              <span class="menu-title">{{ item.name }}</span>
            </template>
            <el-menu-item
              :index="i.enid"
              v-for="i in item.labelList"
              :key="i.enid"
              @click="gotoCategory(item, i.enid)"
            >
              <template #title>{{ i.name }}</template>
            </el-menu-item>
          </el-sub-menu>

          <el-sub-menu
            index="more"
            @mouseenter="mouseenter"
            v-show="categoryList.length > 9 && 9 == moreCategory"
          >
            <template #title><span class="menu-title">更多</span></template>
          </el-sub-menu>
        </el-menu>
        <div v-if="categoryList.length === 0" class="menu-empty">
          分类加载中，稍后自动同步
        </div>
      </div>

      <!-- 中间轮播图 -->
      <div class="banner-wrapper">
        <el-carousel v-if="bannerList.length > 0" :interval="5000" arrow="hover" height="380px" class="custom-carousel">
          <el-carousel-item v-for="item in bannerList" :key="item">
            <el-image :src="item.img" fit="cover" class="banner-image" @click="BrowserOpenURL(item.url)" />
          </el-carousel-item>
        </el-carousel>
        <div v-else class="banner-fallback">
          <p class="fallback-kicker">沉浸学习模式</p>
          <h2>今天想先看视频，还是先读文稿？</h2>
          <p>根据你的学习记录快速回到上次进度，并在课程、音频、电子书之间无缝切换。</p>
          <div class="fallback-actions">
            <button class="fallback-chip" @click="goToCourseList">
              <el-icon><VideoPlay /></el-icon>
              <span>课程视频</span>
            </button>
            <button class="fallback-chip" @click="goToAudioList">
              <el-icon><Headset /></el-icon>
              <span>音频连播</span>
            </button>
            <button class="fallback-chip" @click="goToEbookList">
              <el-icon><Reading /></el-icon>
              <span>文稿阅读</span>
            </button>
          </div>
        </div>
      </div>

      <!-- 右侧用户信息卡片 -->
      <div class="user-card-wrapper">
        <div class="user-card" :class="!store.loggedIn ? 'not-login' : ''">
          <div v-if="!store.loggedIn" class="login-prompt">
            <div class="login-placeholder">
               <img src="../assets/images/logo-universal.png" alt="Logo" class="login-logo" />
               <p>登录开启学习之旅</p>
            </div>
            <el-button type="primary" class="login-btn" @click="openLoginDialog()" round>
              立即登录
            </el-button>
          </div>
          <div v-else class="user-info">
            <div class="avatar-section">
              <el-avatar
                :size="80"
                :src="profileUser.avatar"
                class="user-avatar"
                @click="goToUserCenter"
              />
              <h3 class="user-name" @click="goToUserCenter">{{ profileUser.nickname }}</h3>
            </div>
            <div class="user-stats">
              <div class="stat-item">
                <span class="stat-label">今日学习</span>
                <div class="stat-value">
                  <span class="number">{{ profileUser.today_study_time > 0 ? (profileUser.today_study_time / 60).toFixed(0) : '0' }}</span>
                  <span class="unit">分钟</span>
                </div>
              </div>
              <div class="stat-divider"></div>
              <div class="stat-item">
                <span class="stat-label">连续学习</span>
                <div class="stat-value">
                  <span class="number">{{ profileUser.study_serial_days }}</span>
                  <span class="unit">天</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 内容模块列表 -->
    <div class="modules-container">
      <div
        class="module-section"
        v-for="(item, moduleIndex) in moduleList"
        :key="item.title"
        :style="{ '--module-delay': `${moduleIndex * 80}ms` }"
      >
        <div
          v-if="
            item.isShow == 3 &&
            (item.type == 'free_class' ||
              item.type == 'class' ||
              item.type == 'ebook')
          "
        >
          <div class="module-header">
            <div class="header-left">
              <h2 class="module-title">{{ item.title }}</h2>
              <span class="module-desc">{{ item.description }}</span>
            </div>
            
            <!-- 标签筛选 -->
            <div class="module-filters" v-if="item.type == 'class' || item.type == 'ebook'">
               <el-scrollbar>
                  <div class="filter-scroll">
                    <template v-if="item.type == 'class'">
                        <span 
                            v-for="(label, index) in courseLabelList.list"
                            :key="label.id"
                            class="filter-tag"
                            :class="idxCourseLabel === index ? 'active' : ''"
                            @click="handleLabel(label, index, 4)"
                        >
                            {{ label.name }}
                        </span>
                    </template>
                     <template v-if="item.type == 'ebook'">
                        <span 
                            v-for="(label, index) in ebookLabelList.list"
                            :key="label.id"
                            class="filter-tag"
                            :class="idxEbookLabel === index ? 'active' : ''"
                            @click="handleLabel(label, index, 2)"
                        >
                            {{ label.name }}
                        </span>
                    </template>
                  </div>
               </el-scrollbar>
            </div>
          </div>

          <div class="module-content">
            <!-- 免费专区 -->
             <div v-if="item.type == 'free_class'" class="cards-grid free-class-grid">
                <div 
                    v-for="(prod, index) in freeResourceList.list" 
                    :key="index"
                    class="content-card free-card"
                    @click="handleFreeProd(prod)"
                >
                    <div class="card-cover">
                        <img :src="ossProcess(prod.logo)" :alt="prod.name" loading="lazy" />
                        <div class="play-overlay"><el-icon><VideoPlay /></el-icon></div>
                    </div>
                    <div class="card-info">
                        <h4 class="card-title">{{ prod.name }}</h4>
                        <p class="card-intro">{{ prod.intro }}</p>
                        <div class="card-footer">
                             <el-rate
                                v-model.number="prod.score"
                                disabled
                                allow-half
                                size="small"
                            />
                        </div>
                    </div>
                </div>
             </div>

             <!-- 课程列表 -->
             <div v-if="item.type == 'class'" class="cards-grid course-grid">
                 <div 
                    v-for="(prod, index) in courseContentList.product_list" 
                    :key="prod.product_enid || index"
                    class="content-card course-card"
                    @click="handleClassProd(prod)"
                >
                    <div class="card-cover">
                         <img :src="ossProcess(prod.horizontal_image)" :alt="prod.title" loading="lazy" />
                    </div>
                     <div class="card-info">
                        <h4 class="card-title">{{ prod.title }}</h4>
                        <p class="card-intro">{{ prod.intro }}</p>
                        <div class="card-footer">
                            <span class="learn-count">{{ prod.learn_user_count }}人加入</span>
                        </div>
                    </div>
                </div>
                 <div class="more-btn-wrapper">
                    <el-button round @click="gotoCategory(currentCourse, '')">查看更多 {{ currentCourse.name }}</el-button>
                </div>
             </div>

             <!-- 电子书列表 -->
              <div v-if="item.type == 'ebook'" class="cards-grid ebook-grid">
                 <div 
                    v-for="(prod, index) in ebookContentList.product_list" 
                    :key="index"
                    class="content-card ebook-card"
                    @click="handleProd(prod.product_enid, prod.product_type)"
                >
                    <div class="ebook-cover-wrapper">
                         <img :src="ossEbookProcess(prod.index_image)" :alt="prod.title" loading="lazy" class="ebook-cover" />
                    </div>
                    <div class="card-info">
                        <h4 class="card-title">{{ ebookTitle(prod.title) }}</h4>
                        <p class="card-author">{{ authorList(prod.author_list) }}</p>
                         <div class="card-footer">
                            <el-rate
                                :model-value="handleScore(prod.score)"
                                disabled
                                allow-half
                                size="small"
                                v-if="prod.score.length > 0"
                            />
                             <span v-else class="no-score">暂无评分</span>
                        </div>
                    </div>
                </div>
                 <div class="more-btn-wrapper">
                    <el-button round @click="gotoCategory(currentEbook, '')">查看更多 {{ currentEbook.name }}</el-button>
                </div>
             </div>

          </div>
        </div>
      </div>
      <div v-if="moduleList.length === 0" class="module-empty-grid">
        <button class="empty-learning-card" @click="goToCourseList">
          <el-icon><VideoPlay /></el-icon>
          <h3>课程学习</h3>
          <p>按课程章节系统学习，支持视频与文稿切换。</p>
        </button>
        <button class="empty-learning-card" @click="goToAudioList">
          <el-icon><Headset /></el-icon>
          <h3>音频学习</h3>
          <p>播放列表连续收听，保持学习节奏不中断。</p>
        </button>
        <button class="empty-learning-card" @click="goToEbookList">
          <el-icon><Reading /></el-icon>
          <h3>阅读学习</h3>
          <p>沉浸式阅读界面，保留字体和进度偏好。</p>
        </button>
      </div>
    </div>
    </template>
  </div>

  <QrLogin
    v-if="loginVisible"
    :dialog-visible="loginVisible"
    @close="closeDialog"
  ></QrLogin>
  <EbookInfo
    v-if="ebookVisible"
    :enid="prodEnid"
    :dialog-visible="ebookVisible"
    @close="closeDialog"
  ></EbookInfo>
  <CourseInfo
    v-if="courseVisible"
    :enid="prodEnid"
    :dialog-visible="courseVisible"
    @close="closeDialog"
  ></CourseInfo>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted, onBeforeUnmount, computed, watch } from "vue";
import { ElMessage } from "element-plus";
import { VideoPlay, Headset, Reading } from '@element-plus/icons-vue'
import {
  GetHomeInitialState,
  SunflowerLabelList,
  SunflowerLabelContent,
  SunflowerResourceList,
  UserInfo,
} from "../../wailsjs/go/backend/App";
import { services } from "../../wailsjs/go/models";
import { BrowserOpenURL } from "../../wailsjs/runtime";
import QrLogin from "../components/QrLogin.vue";
import EbookInfo from "../components/EbookInfo.vue";
import CourseInfo from "../components/CourseInfo.vue";
import { useAppRouter } from "../composables/useRouter";
import { ROUTE_NAMES } from "../router/routes";
import { learningStore } from "../stores/learning";
import { playerStore } from "../stores/player";
import { classifySessionErrorMessage, userStore } from "../stores/user";

const { pushByName, push } = useAppRouter();
const lStore = learningStore();
const pStore = playerStore();
const store = userStore();

const loading = ref(true);
const loadError = ref("");
const page = ref(0);
const total = ref(0);
const pageSize = ref(4);
const dialogVisible = ref(false);
const loginVisible = ref(false);
const ebookVisible = ref(false);
const courseVisible = ref(false);
const prodEnid = ref("");
const menuRef = ref();

const moreCategory = ref(9);
const idxEbookLabel = ref(0);
const idxCourseLabel = ref(0);
const isMouseInMenu = ref(false);
const currentOpenedMenu = ref<string>("");

let initial = reactive(new services.HomeInitState());
initial.homeData= reactive(new services.HomeData)

let ebookLabelList = reactive(new services.SunflowerLabelList());
let courseLabelList = reactive(new services.SunflowerLabelList());

let freeResourceList = reactive(new services.SunflowerResourceList());
let ebookContentList = reactive(new services.SunflowerContent());
let courseContentList = reactive(new services.SunflowerContent());

let currentCourse = reactive(new services.Navigation());
let currentEbook = reactive(new services.Navigation());
let user = reactive(new services.User());
const emptyUser = new services.User();

const categoryList = computed(() => initial.homeData?.categoryList ?? []);
const bannerList = computed(() => initial.homeData?.banner ?? []);
const moduleList = computed(() => initial.homeData?.moduleList ?? []);
const profileUser = computed(() => {
  if (String(user.uid_hazy ?? "").trim()) return user;
  return store.user ?? emptyUser;
});
const todayStudyMinutes = computed(() => {
  const minutes = Number(profileUser.value.today_study_time ?? 0) / 60;
  return Math.max(0, Math.floor(minutes));
});
const studyStreakDays = computed(() => {
  return Math.max(0, Number(profileUser.value.study_serial_days ?? 0));
});
const articleProgressText = computed(() => {
  if (!lStore.hasLastArticle) return "暂无";
  const progress = Number(lStore.lastArticle?.progress ?? 0);
  return `${Math.max(0, Math.min(100, Math.round(progress)))}%`;
});
const nowPlayingTitle = computed(() => {
  return String(pStore.currentTrack?.title ?? "").trim() || "打开当前播放列表";
});
const courseSpotlightCount = computed(() => Number((courseContentList as any)?.product_list?.length ?? 0));
const ebookSpotlightCount = computed(() => Number((ebookContentList as any)?.product_list?.length ?? 0));
const audioLaneSummary = computed(() => {
  if (pStore.hasTrack) {
    return `正在播放：${nowPlayingTitle.value}`;
  }
  return "进入听书页可一键创建连播队列";
});
const hasHomeContent = computed(() => {
  return Boolean(
    categoryList.value.length ||
    bannerList.value.length ||
    moduleList.value.length ||
    freeResourceList.list?.length ||
    ebookContentList.product_list?.length ||
    courseContentList.product_list?.length
  );
});
const userInfoLoaded = ref(false);
let userInfoRequest: Promise<boolean> | null = null;
const HOME_EBOOK_PAGE_SIZE = 10;
const HOME_COURSE_PAGE_SIZE = 4;
let homeLoadGeneration = 0;
let userInfoGeneration = 0;
let homeActive = true;
let expiredSessionHandled = false;

type HomeHandledError = {
  __homeHandled: true;
  cause: unknown;
  message: string;
};

const normalizeErrorMessage = (error: unknown) => {
  const message = String(error || "").trim();
  return message || "请检查网络连接后重试";
};

const showGenericError = (error: unknown) => {
  const message = normalizeErrorMessage(error);
  if (!message) return;
  ElMessage({
    message,
    type: "warning",
  });
};

const handleSessionError = async (
  error: unknown,
  options: { notify?: boolean } = {}
) => {
  const raw = String(error || "");
  const classification = classifySessionErrorMessage(raw);
  if (!classification) return "";

  let sessionMessage = "";
  if (classification === "verification") {
    sessionMessage = "需要验证，请先在得到官网完成验证码后重试";
  } else {
    if (!expiredSessionHandled) {
      expiredSessionHandled = true;
      sessionMessage = await store.classifySessionError(error);
      pushByName(ROUTE_NAMES.LOGIN);
    } else {
      sessionMessage = "登录已失效，请重新扫码登录";
    }
  }

  if (options.notify !== false) {
    ElMessage({
      message: sessionMessage,
      type: "warning",
    });
  }
  return sessionMessage;
};

const isCurrentLoadGeneration = (generation: number) => {
  return homeActive && generation === homeLoadGeneration;
};

const isHomeHandledError = (error: unknown): error is HomeHandledError => {
  return Boolean(error && typeof error === "object" && "__homeHandled" in error);
};

const buildHandledError = (cause: unknown, message: string): HomeHandledError => {
  return {
    __homeHandled: true,
    cause,
    message,
  };
};

const setVisibleLoadError = (message: string, generation?: number) => {
  if (!message) return;
  if (typeof generation === "number" && !isCurrentLoadGeneration(generation)) return;
  if (!homeActive) return;
  loadError.value = message;
};

const resolveErrorMessage = async (error: unknown) => {
  if (isHomeHandledError(error)) {
    return error.message;
  }
  const sessionMessage = await handleSessionError(error, { notify: false });
  return sessionMessage || normalizeErrorMessage(error);
};

const collectRejectedReasons = (results: PromiseSettledResult<unknown>[]) => {
  return results
    .filter((result): result is PromiseRejectedResult => result.status === "rejected")
    .map((result) => result.reason);
};

const resolveLoadErrorMessage = async (errors: unknown[]) => {
  const messages: string[] = [];
  for (const error of errors) {
    const message = await resolveErrorMessage(error);
    if (message && !messages.includes(message)) {
      messages.push(message);
    }
  }
  return messages.join("；");
};

const invalidateUserInfoState = (options: { clearUser?: boolean } = {}) => {
  userInfoGeneration += 1;
  userInfoLoaded.value = false;
  userInfoRequest = null;
  if (options.clearUser !== false) {
    Object.assign(user, new services.User());
  }
};

const loadSunflowerContent = async (enid: string, nType: number, size = pageSize.value) => {
  try {
    const list = await SunflowerLabelContent(enid, nType, page.value, size);
    if (nType == 2) {
      Object.assign(ebookContentList, list);
    } else if (nType == 4) {
      Object.assign(courseContentList, list);
    }
    return true;
  } catch (error) {
    if (await handleSessionError(error)) return false;
    showGenericError(error);
    return false;
  }
};

const getUserInfo = async (requestGeneration: number) => {
  const result = await UserInfo();
  if (!homeActive || requestGeneration !== userInfoGeneration || !store.loggedIn) {
    return false;
  }
  Object.assign(user, result);
  store.acceptLogin(Object.assign(new services.User(), result));
  if (!homeActive || requestGeneration !== userInfoGeneration || !store.loggedIn) {
    return false;
  }
  userInfoLoaded.value = true;
  return true;
};

const ensureUserInfo = async () => {
  if (!store.loggedIn) {
    invalidateUserInfoState();
    return true;
  }
  if (userInfoLoaded.value) return true;
  if (userInfoRequest) return userInfoRequest;

  const requestGeneration = ++userInfoGeneration;
  const request = getUserInfo(requestGeneration)
    .catch(async (error) => {
      const message = await resolveErrorMessage(error);
      if (requestGeneration === userInfoGeneration && homeActive) {
        setVisibleLoadError(message);
      }
      throw buildHandledError(error, message);
    })
    .finally(() => {
      if (requestGeneration === userInfoGeneration) {
        userInfoRequest = null;
      }
    });

  userInfoRequest = request;
  return userInfoRequest;
};

const loadHome = async () => {
  const generation = ++homeLoadGeneration;
  loading.value = true;
  loadError.value = "";
  const initialResults = await Promise.allSettled([
    GetHomeInitialState(),
    SunflowerLabelList(2),
    SunflowerLabelList(4),
    SunflowerResourceList(),
  ] as const);

  if (!isCurrentLoadGeneration(generation)) return;

  const [homeStateResult, ebookLabelsResult, courseLabelsResult, resourcesResult] = initialResults;

  if (homeStateResult.status === "fulfilled") {
    Object.assign(initial, homeStateResult.value);
  }
  if (ebookLabelsResult.status === "fulfilled") {
    Object.assign(ebookLabelList, ebookLabelsResult.value);
    if (ebookLabelsResult.value.list?.[0]) {
      Object.assign(currentEbook, ebookLabelsResult.value.list[0]);
    }
  }
  if (courseLabelsResult.status === "fulfilled") {
    Object.assign(courseLabelList, courseLabelsResult.value);
    if (courseLabelsResult.value.list?.[0]) {
      Object.assign(currentCourse, courseLabelsResult.value.list[0]);
    }
  }
  if (resourcesResult.status === "fulfilled") {
    Object.assign(freeResourceList, resourcesResult.value);
  }

  const contentResults = await Promise.allSettled([
    SunflowerLabelContent("", 2, page.value, HOME_EBOOK_PAGE_SIZE),
    SunflowerLabelContent("", 4, page.value, HOME_COURSE_PAGE_SIZE),
    store.sessionLoaded && store.loggedIn ? ensureUserInfo() : Promise.resolve(true),
  ] as const);

  if (!isCurrentLoadGeneration(generation)) return;

  const [ebooksResult, coursesResult] = contentResults;

  if (ebooksResult.status === "fulfilled") {
    Object.assign(ebookContentList, ebooksResult.value);
    idxEbookLabel.value = 0;
  }
  if (coursesResult.status === "fulfilled") {
    Object.assign(courseContentList, coursesResult.value);
    idxCourseLabel.value = 0;
  }

  const loadErrors = [
    ...collectRejectedReasons(initialResults),
    ...collectRejectedReasons(contentResults),
  ];

  if (loadErrors.length > 0) {
    const message = await resolveLoadErrorMessage(loadErrors);
    setVisibleLoadError(message, generation);
  }

  if (isCurrentLoadGeneration(generation)) {
    loading.value = false;
  }
};

watch(
  () => store.loggedIn,
  (loggedIn) => {
    if (loggedIn) {
      expiredSessionHandled = false;
    } else {
      invalidateUserInfoState();
    }
  }
);

watch(
  () => [store.sessionLoaded, store.loggedIn] as const,
  async ([sessionLoaded, loggedIn]) => {
    if (!sessionLoaded || !loggedIn) return;
    try {
      await ensureUserInfo();
    } catch {
    }
  },
  { immediate: true }
);

onMounted(() => {
  void loadHome();
});

onBeforeUnmount(() => {
  homeActive = false;
  homeLoadGeneration += 1;
  invalidateUserInfoState({ clearUser: false });
});

const goToCourseList = () => {
  pushByName(ROUTE_NAMES.COURSE);
};

const goToLiveList = () => {
  pushByName(ROUTE_NAMES.LIVE);
};

const goToAudioList = () => {
  pushByName(ROUTE_NAMES.ODOB);
};

const goToEbookList = () => {
  pushByName(ROUTE_NAMES.EBOOK);
};

const openNowPlaying = () => {
  pStore.openPlaylist();
};

const goContinueLearning = () => {
  const path = String(lStore.lastArticle?.path ?? "").trim();
  if (!path) {
    goToCourseList();
    return;
  }
  push(path);
};


const handleProd = (enid: string, iType: number) => {
  // 2-ebook,4-专栏,36-大师课,66-class,22-course,65-学习圈
  // console.log('handleProd:', enid, iType);
  prodEnid.value = enid;
  if (iType == 2) {
    ebookVisible.value = true;
  } else if (iType == 65) {
    courseVisible.value = false;
  } else {
    courseVisible.value = true;
  }
};

const handleFreeProd = (item: any) => {
  // 统一跳转到详情页，不再弹窗
  pushByName(ROUTE_NAMES.ARTICLE_LIST, { id: item.enid }, { 
    enid: item.enid, 
    title: item.name 
  });
};

const handleClassProd = async (item: services.ProductSimple) => {
  const enid = String(item?.product_enid ?? "").trim();
  if (!enid) return;
  if (item?.product_type != 66) {
    courseVisible.value = false;
    return;
  };
  pushByName(ROUTE_NAMES.ARTICLE_LIST, { id: enid }, { enid, title: item.title });
};

const handleLabel = (
  item: services.Navigation,
  index: number,
  nType: number
) => {
  if (nType == 2) {
    pageSize.value = 10;
    Object.assign(currentEbook, item);
    idxEbookLabel.value = index;
  }
  if (nType == 4) {
    pageSize.value = 4;
    Object.assign(currentCourse, item);
    idxCourseLabel.value = index;
  }
  sunflowerLabelContent(item.enid, nType);
};

const sunflowerLabelContent = async (enid: string, nType: number) => {
  const size = nType == 2 ? 10 : 4;
  await loadSunflowerContent(enid, nType, size);
};

const ossProcess = (url: string) => {
  return url + "?x-oss-process=image/crop,h_608/resize,w_1080,h_608,m_fill";
};

const ossAvatarProcess = (url: string) => {
  return url + "?x-oss-process=image/resize,w_96,m_lfit";
};

const ossEbookProcess = (url: string) => {
  return url + "?x-oss-process=image/crop,h_790/resize,w_600,h_790,m_fill";
};

const ebookTitle = (title: string) => {
  if (title.length <= 9) {
    return title;
  } else {
    return title.substring(0, 9).concat("...");
  }
};

const authorList = (authors: string[]) => {
  if (authors.length > 1) {
    return authors[0] + " 等";
  } else {
    return authors.toString();
  }
};

const ebookPopoverContent = (intro: string, introduction: string) => {
  if (intro.concat(introduction).length <= 146) {
    return intro.concat(introduction);
  } else {
    return intro.concat(introduction).substring(0, 146).concat("...");
  }
};

const mouseenter = () => {
  // 显示所有分类
  moreCategory.value = categoryList.value.length;
};

const onMenuEnter = () => {
  // 鼠标进入菜单区域
  isMouseInMenu.value = true;
};

const onMenuLeave = (event: MouseEvent) => {
  // 检查鼠标是否还在菜单相关的元素上
  const menuElement = (event.currentTarget as HTMLElement);
  const relatedTarget = event.relatedTarget as Element | null;

  // 如果鼠标还在菜单内或二级菜单上，不隐藏
  if (relatedTarget &&
      (menuElement.contains(relatedTarget) ||
       (relatedTarget.closest && relatedTarget.closest('.el-menu--popup')))) {
    return;
  }

  // 鼠标真正离开菜单区域，隐藏多余的分类
  isMouseInMenu.value = false;
  moreCategory.value = Math.min(9, categoryList.value.length);
  
  // 收起已展开的菜单
  if (menuRef.value && currentOpenedMenu.value) {
    menuRef.value.close(currentOpenedMenu.value);
    currentOpenedMenu.value = "";
  }
};

const handleMenuEnter = (index: string) => {
  // 鼠标悬浮展开二级菜单
  if (menuRef.value) {
    menuRef.value.open(index);
    currentOpenedMenu.value = index;
  }
};

const handleOpen = (key: string, keyPath: string[]) => {
  // 不需要在这里处理 more 的展开
};

const handleClose = (key: string, keyPath: string[]) => {
  // 不需要在这里处理 more 的收起
};

const handleScore = (score: string) => {
  return score != "" ? parseFloat(score) : 0;
};

const openLoginDialog = () => {
  loginVisible.value = true;
};

const openDialog = () => {
  dialogVisible.value = true;
};
const closeDialog = () => {
  //   initForm()
  dialogVisible.value = false;
  loginVisible.value = false;
  ebookVisible.value = false;
  courseVisible.value = false;
};

const goToUserCenter = () => {
  pushByName(ROUTE_NAMES.PROFILE);
};

const gotoCategory = (item: any, label_id: string) => {
  if (item.nav_type == 4 && item.enid && item.enid.startsWith("aiSphereGroupType:")) {
    pushByName("aiChannel");
    return;
  }

  let product_type = "0"; // 默认设置为课程类型
  if (item.nav_type == 2) {
    product_type = "2"; // 电子书
  } else if (item.nav_type == 4) {
    product_type = "66"; // 课程/专栏
  }
  pushByName(ROUTE_NAMES.CATEGORY, {}, {
    id: item.id,
    enid: item.enid,
    name: item.name,
    nav_type: item.nav_type,
    label_id: label_id,
    product_type: product_type,
  });
};
</script>

<style scoped>
.home-container-wrapper {
  position: relative;
  isolation: isolate;
  padding: clamp(14px, 1.6vw, 24px);
  width: 100%;
  max-width: none;
  margin: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 24px;
  overflow-y: auto;
  box-sizing: border-box;
  /* 隐藏滚动条但保留功能 - 清新风格 */
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE 10+ */
}

.home-container-wrapper::-webkit-scrollbar {
  display: none; /* Chrome/Safari */
}

.home-feedback-shell {
  position: relative;
  z-index: 1;
}

.home-feedback-panel {
  padding: clamp(18px, 2vw, 28px);
  border-radius: 20px;
  background:
    linear-gradient(140deg, color-mix(in srgb, var(--hero-gradient-1) 28%, transparent) 0%, transparent 58%),
    linear-gradient(220deg, color-mix(in srgb, var(--hero-gradient-2) 24%, transparent) 0%, transparent 62%),
    color-mix(in srgb, var(--surface-glass) 84%, transparent);
  border: 1px solid color-mix(in srgb, var(--border-soft) 78%, transparent);
  box-shadow: 0 18px 34px rgba(12, 20, 36, 0.08);
  backdrop-filter: blur(14px);
}

.home-inline-feedback {
  position: relative;
  z-index: 2;
}

.home-inline-feedback-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
}

.home-error-alert {
  margin-bottom: 16px;
}

.home-error-result {
  padding: 12px 0 0;
}

.home-skeleton-hero {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 8px 0 24px;
}

.home-skeleton-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.home-skeleton-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.home-skeleton-card {
  overflow: hidden;
  border-radius: 18px;
  background: color-mix(in srgb, var(--surface-color) 94%, transparent);
  border: 1px solid color-mix(in srgb, var(--border-soft) 70%, transparent);
}

.home-skeleton-card-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
}

.skeleton-kicker {
  width: 140px;
  height: 14px;
}

.skeleton-title {
  width: min(460px, 72%);
  height: 38px;
}

.skeleton-line {
  width: 100%;
  height: 14px;
}

.skeleton-line-short {
  width: min(480px, 64%);
}

.skeleton-button {
  width: 116px;
  height: 36px;
}

.skeleton-card-cover {
  display: block;
  width: 100%;
  height: 180px;
}

.skeleton-card-title {
  width: 72%;
  height: 20px;
}

.home-ambient {
  position: absolute;
  border-radius: 999px;
  filter: blur(42px);
  pointer-events: none;
  z-index: -1;
}

.home-ambient-left {
  width: 260px;
  height: 260px;
  background: color-mix(in srgb, var(--accent-color) 24%, transparent);
  top: 80px;
  left: -130px;
  opacity: 0.28;
}

.home-ambient-right {
  width: 320px;
  height: 320px;
  background: color-mix(in srgb, var(--primary-color) 24%, transparent);
  right: -120px;
  top: 10px;
  opacity: 0.24;
}

.learning-deck {
  position: relative;
  display: grid;
  grid-template-columns: 1fr 280px;
  align-items: start;
  gap: 18px;
  min-height: auto;
  height: auto;
  border-radius: 18px;
  padding: 22px;
  background:
    linear-gradient(130deg, color-mix(in srgb, var(--hero-gradient-1) 74%, transparent) 0%, transparent 56%),
    linear-gradient(220deg, color-mix(in srgb, var(--hero-gradient-2) 72%, transparent) 0%, transparent 60%),
    color-mix(in srgb, var(--surface-glass) 76%, transparent);
  border: 1px solid color-mix(in srgb, var(--border-soft) 78%, transparent);
  box-shadow: 0 16px 34px rgba(12, 20, 36, 0.08);
  backdrop-filter: blur(14px);
  overflow: visible;
  z-index: 2;
}

.learning-deck::after {
  content: "";
  position: absolute;
  width: 180px;
  height: 180px;
  border-radius: 999px;
  right: 180px;
  top: -80px;
  pointer-events: none;
  background: color-mix(in srgb, var(--accent-color) 16%, transparent);
  filter: blur(26px);
}

.deck-main {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
}

.deck-kicker {
  margin: 0 0 6px;
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--accent-color);
  font-weight: 700;
  font-family: var(--font-family-display);
}

.deck-title {
  margin: 0;
  font-size: 34px;
  line-height: 1.12;
  color: var(--text-primary);
  font-family: var(--font-family-display);
}

.deck-subtitle {
  margin: 10px 0 0;
  color: var(--text-secondary);
  font-size: 14px;
  max-width: 680px;
}

.deck-actions {
  margin-top: 18px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.deck-side {
  display: grid;
  grid-template-columns: 1fr;
  grid-template-rows: auto 1fr;
  gap: 8px;
  align-content: start;
  min-height: auto;
  overflow: visible;
}

.deck-stats-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.deck-side-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 2px;
}

.deck-side-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.deck-side-sub {
  font-size: 11px;
  color: var(--text-tertiary);
  border-radius: 999px;
  padding: 2px 8px;
  background: color-mix(in srgb, var(--fill-color-light) 72%, transparent);
}

.deck-stat {
  flex: 0 0 auto;
  min-height: 56px;
  padding: 8px 12px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 72%, transparent);
  background: color-mix(in srgb, var(--card-bg) 74%, transparent);
  text-align: left;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.deck-stat span {
  display: block;
  font-size: 12px;
  color: var(--text-tertiary);
}

.deck-stat strong {
  display: block;
  margin-top: 4px;
  font-size: 16px;
  color: var(--text-primary);
}

.deck-stat-value {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
}

.deck-stat-value strong {
  margin-top: 0;
  font-size: 24px;
  line-height: 1.15;
}

.deck-stat-value em {
  font-style: normal;
  font-size: 12px;
  color: var(--text-secondary);
}

.deck-playing-btn {
  margin-top: 0;
  height: 40px;
  border: 0;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  background: color-mix(in srgb, var(--accent-color) 18%, var(--card-bg) 82%);
  color: var(--text-primary);
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.deck-playing-btn span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.deck-playing-btn:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-soft);
}

.workspace-lanes {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.lane-card {
  position: relative;
  border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
  border-radius: 16px;
  background: color-mix(in srgb, var(--card-bg) 88%, transparent);
  color: var(--text-primary);
  text-align: left;
  padding: 16px;
  min-height: 150px;
  display: grid;
  grid-template-columns: 42px 1fr;
  gap: 12px;
  cursor: pointer;
  overflow: hidden;
  transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease;
}

.lane-card::before {
  content: "";
  position: absolute;
  inset: 0;
  opacity: 0.85;
  z-index: 0;
  pointer-events: none;
}

.lane-card > * {
  position: relative;
  z-index: 1;
}

.lane-icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.lane-body {
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.lane-kicker {
  margin: 0;
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-secondary);
  font-family: var(--font-family-mono);
}

.lane-card h3 {
  margin: 6px 0 6px;
  font-size: 20px;
  line-height: 1.2;
  font-family: var(--font-family-display);
}

.lane-card p {
  margin: 0;
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.55;
}

.lane-meta {
  margin-top: auto;
  padding-top: 8px;
  font-size: 12px;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.lane-card-course::before {
  background:
    radial-gradient(220px 120px at 0% 0%, rgba(78, 133, 255, 0.18) 0%, transparent 70%),
    linear-gradient(120deg, rgba(236, 244, 255, 0.9) 0%, rgba(255, 255, 255, 0.8) 100%);
}

.lane-card-course .lane-icon {
  background: rgba(78, 133, 255, 0.16);
  color: #2f5de2;
}

.lane-card-audio::before {
  background:
    radial-gradient(220px 120px at 0% 0%, rgba(15, 142, 161, 0.2) 0%, transparent 70%),
    linear-gradient(120deg, rgba(235, 250, 252, 0.92) 0%, rgba(255, 255, 255, 0.82) 100%);
}

.lane-card-audio .lane-icon {
  background: rgba(15, 142, 161, 0.16);
  color: #0f8ea1;
}

.lane-card-ebook::before {
  background:
    radial-gradient(220px 120px at 0% 0%, rgba(165, 98, 47, 0.2) 0%, transparent 70%),
    linear-gradient(120deg, rgba(255, 248, 240, 0.92) 0%, rgba(255, 255, 255, 0.82) 100%);
}

.lane-card-ebook .lane-icon {
  background: rgba(165, 98, 47, 0.16);
  color: #8d4a22;
}

.lane-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 14px 26px rgba(8, 16, 30, 0.12);
}

.lane-card-course:hover {
  border-color: rgba(78, 133, 255, 0.44);
}

.lane-card-audio:hover {
  border-color: rgba(15, 142, 161, 0.46);
}

.lane-card-ebook:hover {
  border-color: rgba(165, 98, 47, 0.46);
}

/* --- Top Section --- */
.top-section {
  display: flex;
  gap: 24px;
  margin-bottom: 20px;
  height: 380px;
}

.menu-wrapper {
  width: 200px;
  flex-shrink: 0;
  background: color-mix(in srgb, var(--card-bg) 85%, transparent);
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
  box-shadow: 0 12px 30px rgba(10, 18, 32, 0.08);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.classification-menu {
  border: none;
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
  background-color: transparent !important; /* 确保背景透明，使用外层容器背景 */
  
  /* 隐藏滚动条但保留功能 */
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE 10+ */
}

/* 隐藏滚动条 */
.classification-menu::-webkit-scrollbar {
  display: none; /* Chrome/Safari */
}

.menu-empty {
  padding: 10px 14px 14px;
  color: var(--text-tertiary);
  font-size: 12px;
  text-align: left;
}

/* 强制覆盖 Element Plus 菜单选中和 Hover 样式，实现清新风格 */
:deep(.el-menu) {
  background-color: transparent;
}

:deep(.el-sub-menu__title) {
  height: 40px;
  line-height: 40px;
  border-radius: 8px;
  margin: 0 8px;
  color: var(--text-primary);
}

:deep(.el-sub-menu__title:hover) {
  background-color: var(--fill-color-light) !important;
  color: var(--accent-color);
}

:deep(.el-menu-item) {
  height: 36px;
  line-height: 36px;
  border-radius: 8px;
  margin: 4px 16px;
  color: var(--text-secondary);
}

:deep(.el-menu-item:hover) {
  background-color: var(--fill-color-light);
  color: var(--accent-color);
}

:deep(.el-menu-item.is-active) {
  background-color: var(--fill-color);
  color: var(--accent-color);
  font-weight: 500;
}


.banner-wrapper {
  flex: 1;
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--border-soft) 72%, transparent);
  box-shadow: 0 14px 32px rgba(8, 16, 30, 0.1);
  min-width: 0; /* 防止 Flex 子元素溢出 */
}

.banner-image {
  width: 100%;
  height: 100%;
  cursor: pointer;
  transition: transform 0.5s ease;
}

.banner-image:hover {
  transform: scale(1.02);
}

.banner-fallback {
  width: 100%;
  height: 100%;
  padding: 32px;
  box-sizing: border-box;
  text-align: left;
  display: flex;
  flex-direction: column;
  justify-content: center;
  background:
    radial-gradient(260px 180px at 18% 20%, color-mix(in srgb, var(--accent-color) 16%, transparent) 0%, transparent 70%),
    radial-gradient(280px 220px at 90% 10%, color-mix(in srgb, var(--primary-color) 20%, transparent) 0%, transparent 76%),
    linear-gradient(130deg, color-mix(in srgb, var(--fill-color-light) 76%, transparent) 0%, color-mix(in srgb, var(--card-bg) 82%, transparent) 100%);
}

.fallback-kicker {
  margin: 0;
  font-size: 12px;
  color: var(--accent-color);
  letter-spacing: 0.12em;
  text-transform: uppercase;
  font-family: var(--font-family-display);
}

.banner-fallback h2 {
  margin: 12px 0 8px;
  font-size: 30px;
  line-height: 1.18;
  color: var(--text-primary);
  max-width: 620px;
  font-family: var(--font-family-display);
}

.banner-fallback p {
  margin: 0;
  color: var(--text-secondary);
  max-width: 620px;
}

.fallback-actions {
  margin-top: 20px;
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.fallback-chip {
  height: 38px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 80%, transparent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--card-bg) 90%, transparent);
  padding: 0 14px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-primary);
  cursor: pointer;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

.fallback-chip:hover {
  transform: translateY(-1px);
  border-color: var(--accent-color);
  box-shadow: var(--shadow-soft);
}

.user-card-wrapper {
  width: 260px;
  flex-shrink: 0;
}

.user-card {
  height: 100%;
  background: color-mix(in srgb, var(--card-bg) 84%, transparent);
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
  box-shadow: 0 12px 28px rgba(10, 18, 32, 0.08);
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 24px;
  box-sizing: border-box;
  text-align: center;
}

.user-card.not-login {
  background: linear-gradient(135deg, var(--card-bg) 0%, var(--fill-color-light) 100%);
}

.login-placeholder {
  margin-bottom: 24px;
}

.login-logo {
  width: 64px;
  height: 64px;
  margin-bottom: 16px;
  border-radius: 12px;
}

.login-placeholder p {
  color: var(--text-secondary);
  font-size: 14px;
  margin: 0;
}

.login-btn {
  width: 100%;
  font-weight: 600;
}

.user-info {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.avatar-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-top: 20px;
}

.user-avatar {
  border: 4px solid var(--fill-color-light);
  cursor: pointer;
  transition: transform 0.3s ease;
}

.user-avatar:hover {
  transform: scale(1.05);
  border-color: var(--accent-color);
}

.user-name {
  margin: 16px 0 0;
  font-size: 18px;
  color: var(--text-primary);
  cursor: pointer;
}

.user-stats {
  display: flex;
  justify-content: space-around;
  align-items: center;
  background: var(--fill-color-light);
  border-radius: 8px;
  padding: 16px 8px;
  margin-bottom: 10px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stat-label {
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 4px;
}

.stat-value {
  display: flex;
  align-items: baseline;
  gap: 2px;
}

.stat-value .number {
  font-size: 20px;
  font-weight: bold;
  color: var(--accent-color);
  font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;
}

.stat-value .unit {
  font-size: 12px;
  color: var(--text-tertiary);
}

.stat-divider {
  width: 1px;
  height: 24px;
  background-color: var(--border-color);
}

/* --- Modules Section --- */
.module-section {
  margin-bottom: 48px;
  animation: module-rise 0.5s ease both;
  animation-delay: var(--module-delay, 0ms);
}

.module-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-soft);
}

.header-left {
  display: flex;
  align-items: baseline;
  gap: 16px;
}

.module-title {
  font-size: 24px;
  color: var(--text-primary);
  margin: 0;
  font-family: var(--font-family-display);
}

.module-desc {
  font-size: 14px;
  color: var(--text-tertiary);
}

.module-filters {
  max-width: 50%;
}

.filter-scroll {
  display: flex;
  gap: 8px;
  padding-bottom: 4px;
}

.filter-tag {
  font-size: 13px;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 4px 12px;
  border-radius: 16px;
  background: var(--fill-color);
  transition: all 0.2s ease;
  white-space: nowrap;
}

.filter-tag:hover {
  color: var(--accent-color);
  background: var(--fill-color-light);
}

.filter-tag.active {
  color: #fff;
  background: var(--accent-color);
}

/* --- Cards Grid --- */
.cards-grid {
  display: grid;
  gap: 24px;
}

.free-class-grid {
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
}

.course-grid {
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
}

.ebook-grid {
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
}

.content-card {
  background: var(--card-bg);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: var(--shadow-soft);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
  display: flex;
  flex-direction: column;
}

.content-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-medium);
}

.card-cover {
  width: 100%;
  aspect-ratio: 16/9;
  position: relative;
  overflow: hidden;
}

.card-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.5s ease;
}

.content-card:hover .card-cover img {
  transform: scale(1.05);
}

.play-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.3s ease;
}

.play-overlay .el-icon {
  font-size: 48px;
  color: #fff;
  filter: drop-shadow(0 2px 4px rgba(0,0,0,0.5));
}

.content-card:hover .play-overlay {
  opacity: 1;
}

.ebook-cover-wrapper {
  width: 100%;
  aspect-ratio: 3/4;
  overflow: hidden;
  background: var(--fill-color-light);
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 12px;
  box-sizing: border-box;
}

.ebook-cover {
  width: auto;
  height: 100%;
  max-width: 100%;
  box-shadow: 0 4px 8px rgba(0,0,0,0.15);
  transition: transform 0.3s ease;
}

.content-card:hover .ebook-cover {
  transform: translateY(-4px) scale(1.02);
  box-shadow: 0 8px 16px rgba(0,0,0,0.2);
}

.card-info {
  padding: 12px;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 6px;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-intro, .card-author {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0 0 8px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  flex: 1;
}

.card-footer {
  margin-top: auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.learn-count {
  font-size: 12px;
  color: var(--text-tertiary);
}

.no-score {
  font-size: 12px;
  color: var(--text-placeholder);
}

.more-btn-wrapper {
    grid-column: 1 / -1;
    display: flex;
    justify-content: center;
    margin-top: 24px;
}

.more-btn-wrapper .el-button {
    width: 240px;
    height: 40px;
    font-size: 14px;
    border: 1px solid var(--border-soft);
    background-color: var(--card-bg);
    color: var(--text-secondary);
    transition: all 0.3s ease;
}

.more-btn-wrapper .el-button:hover {
    background-color: var(--fill-color-light);
    border-color: var(--accent-color);
    color: var(--accent-color);
    transform: translateY(-2px);
    box-shadow: var(--shadow-soft);
}

.module-empty-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.empty-learning-card {
  border: 1px solid color-mix(in srgb, var(--border-soft) 78%, transparent);
  background: color-mix(in srgb, var(--card-bg) 88%, transparent);
  border-radius: 14px;
  padding: 22px;
  text-align: left;
  color: var(--text-primary);
  cursor: pointer;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

.empty-learning-card .el-icon {
  font-size: 22px;
  color: var(--accent-color);
}

.empty-learning-card h3 {
  margin: 12px 0 8px;
  font-size: 18px;
  font-family: var(--font-family-display);
}

.empty-learning-card p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.empty-learning-card:hover {
  transform: translateY(-3px);
  border-color: var(--accent-color);
  box-shadow: var(--shadow-soft);
}

@keyframes module-rise {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Responsive adjustments */
@media (max-width: 1200px) {
  .learning-deck {
    grid-template-columns: 1fr;
    gap: 14px;
  }

  .deck-side {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    grid-auto-flow: row;
    gap: 10px;
  }

  .deck-side-head {
    grid-column: 1 / -1;
  }

  .workspace-lanes {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .deck-playing-btn {
    margin-top: 4px;
    max-width: 100%;
  }

  .top-section {
    height: auto;
    flex-wrap: wrap;
  }
  
  .banner-wrapper {
    min-width: 55%;
  }

  .module-empty-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  
  .course-grid {
      grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 992px) {
  .home-container-wrapper {
    padding: 14px;
    gap: 14px;
  }

  .home-inline-feedback-body {
    flex-direction: column;
    align-items: flex-start;
  }

  .home-skeleton-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .home-ambient {
    display: none;
  }

  .deck-title {
    font-size: 26px;
  }

  .deck-subtitle {
    max-width: none;
  }

  .top-section {
    gap: 14px;
  }

  .workspace-lanes {
    grid-template-columns: 1fr;
  }

  .menu-wrapper,
  .user-card-wrapper {
    width: 100%;
  }

  .banner-wrapper {
    min-height: 260px;
  }

  .banner-fallback {
    padding: 20px;
  }

  .banner-fallback h2 {
    font-size: 24px;
  }

  .module-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .module-filters {
    max-width: 100%;
  }

  .module-empty-grid {
    grid-template-columns: 1fr;
  }

  .course-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .ebook-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 680px) {
  .deck-actions {
    width: 100%;
  }

  .deck-side {
    grid-template-columns: 1fr;
  }

  .deck-actions :deep(.el-button) {
    margin: 0;
  }

  .fallback-actions {
    gap: 8px;
  }

  .lane-card {
    grid-template-columns: 1fr;
    min-height: 0;
  }

  .lane-icon {
    width: 38px;
    height: 38px;
    border-radius: 10px;
  }

  .fallback-chip {
    width: 100%;
    justify-content: center;
  }

  .home-skeleton-grid {
    grid-template-columns: 1fr;
  }

  .skeleton-title,
  .skeleton-line-short {
    width: 100%;
  }

  .course-grid,
  .ebook-grid,
  .free-class-grid {
    grid-template-columns: 1fr;
  }
}
</style>
