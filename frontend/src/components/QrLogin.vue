<template>
  <el-dialog
    v-model="dialogVisible"
    width="400px"
    :before-close="closeDialog"
    :show-close="true"
    destroy-on-close
    center
    class="login-dialog"
  >
    <div class="login-container">
      <div class="login-header">
        <h3 class="title">{{ data.title }}</h3>
        <p class="subtitle">{{ data.tips }}</p>
      </div>

      <div class="qr-container">
        <div class="qr-wrapper">
          <el-image
            v-if="qrStatus === 'ready' && data.qrCode"
            class="qr-code-img"
            :src="data.qrCode"
            fit="fill"
            @error="onQrImageError"
          />
          <div v-else-if="qrStatus === 'loading'" class="qr-loading">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>二维码生成中...</span>
          </div>
          <div v-else class="qr-error">
            <el-icon><WarningFilled /></el-icon>
            <p>{{ qrError || "二维码加载失败，请重试" }}</p>
            <div class="qr-error-actions">
              <el-button type="primary" round size="small" @click="getQrcode()">
                重试获取
              </el-button>
              <el-button round size="small" @click="resetLoginState">
                重置登录状态
              </el-button>
            </div>
          </div>
        </div>
      </div>

      <p v-if="qrStatus === 'ready'" class="countdown-tip">
        二维码剩余有效期约 {{ Math.max(1, Math.ceil(timeState.time / 60)) }} 分钟
      </p>

      <div class="login-footer">
        <el-image
          src="https://piccdn2.umiwi.com/fe-oss/default/MTYzNzMwNzUyMzQy.png"
          class="app-logo"
        />
        <span class="footer-text">得到 App 扫码登录</span>
      </div>

      <div class="login-helper">
        <p>若你在网页版重新登录后这里失效，点“重置登录状态”即可恢复。</p>
        <code>rm -rf ~/.config/dedao/config.json</code>
      </div>
    </div>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted, onBeforeUnmount } from "vue";
import { ElMessage } from "element-plus";
import { Loading, WarningFilled } from '@element-plus/icons-vue';
import { useRouter } from "vue-router";
import { userStore } from "../stores/user";
import { services } from "../../wailsjs/go/models";
import { Local } from "../utils/storage";
import { hasBackendBridge, invokeBackend } from "../utils/backend";

const store = userStore();
const router = useRouter();
const data = reactive({
  qrCode: "",
  qrCodeString: "",
  token: "",
  title: "扫码登录",
  tips: "使用得到App或微信扫码登录",
});

type QrStatus = "loading" | "ready" | "error";

const timeState = reactive({
  time: 600, // 600s倒计时
  timer: null as ReturnType<typeof window.setInterval> | null,
});

const dialogVisible = ref(false);
const qrStatus = ref<QrStatus>("loading");
const qrError = ref("");
const gettingQrcode = ref(false);
const checkingLogin = ref(false);
const emits = defineEmits(["close"]);
const props = defineProps({
  dialogVisible: {
    type: Boolean,
    default: false,
  },
});

onMounted(() => {
  openDialog();
  getQrcode(true);
});

const normalizeQrSrc = (raw: string) => {
  const src = String(raw || "").trim();
  if (!src) return "";
  if (src.startsWith("data:image") || src.startsWith("http://") || src.startsWith("https://")) return src;
  if (src.startsWith("//")) return `https:${src}`;
  if (src.startsWith("/")) return `https://www.dedao.cn${src}`;
  return src;
};

const safeErrorMessage = (error: unknown) => {
  if (error instanceof Error) return String(error.message || "").trim();
  return String(error || "").trim();
};

const clearTimer = () => {
  if (timeState.timer != null) {
    clearInterval(timeState.timer);
    timeState.timer = null;
  }
};

const resetLocalSession = () => {
  store.user = null;
  Local.remove("cookies");
  Local.remove("userStore");
};

const buildQrError = (error: unknown) => {
  const msg = safeErrorMessage(error);
  const lower = msg.toLowerCase();
  if (!msg) {
    return "二维码获取失败，请重试";
  }
  if (
    lower.includes("eof") ||
    lower.includes("timeout") ||
    lower.includes("connection reset") ||
    lower.includes("stream error")
  ) {
    return "网络连接波动，登录接口中断。可先点“重试获取”，仍失败再点“重置登录状态”。";
  }
  if (msg.includes("backend") || msg.includes("桌面后端")) {
    return "桌面后端连接异常，请重启应用后再试";
  }
  if (
    lower.includes("getaccesstoken failed(status=403)") ||
    lower.includes("csrf token") ||
    lower.includes("missing csrf token") ||
    lower.includes("invalid csrf token")
  ) {
    return "登录态已失效或被网页版顶掉，请点“重置登录状态”后重新扫码。";
  }
  if (msg.includes("csrf") || msg.includes("401") || msg.includes("cookie") || msg.includes("token")) {
    return "登录态可能失效，建议点“重置登录状态”后重新扫码";
  }
  return msg;
};

const syncUserAfterLogin = (loginResult: any) => {
  const user = reactive(new services.User());
  Object.assign(user, loginResult?.user || {});
  store.user = user;

  const cookie = String(loginResult?.cookie || "").trim();
  if (cookie) {
    Local.set("cookies", cookie);
  }

  const existed = store.userList.some((item) => item.uid_hazy === user.uid_hazy);
  if (!existed) {
    store.userList.push(user);
  }
};

const startCheckLoginTimer = () => {
  if (timeState.timer != null) return;
  timeState.timer = window.setInterval(async () => {
    if (!data.token || !data.qrCodeString || checkingLogin.value) return;
    checkingLogin.value = true;
    try {
      timeState.time = Math.max(0, timeState.time - 2);
      if (timeState.time <= 0) {
        await getQrcode(true);
        return;
      }

      const loginResult = await invokeBackend<any>("CheckLogin", data.token, data.qrCodeString);
      if (loginResult?.status === 1) {
        clearTimer();
        syncUserAfterLogin(loginResult);
        emits("close");
        router.push("/user/profile");
        return;
      }

      if (loginResult?.status === 2) {
        await getQrcode(true);
        ElMessage({ message: "二维码已过期，已自动刷新", type: "warning" });
      }
    } catch (error) {
      const msg = safeErrorMessage(error);
      if (msg.includes("二维码已过期")) {
        await getQrcode(true);
      } else if (msg.includes("backend") || msg.includes("桌面后端")) {
        clearTimer();
        qrStatus.value = "error";
        qrError.value = buildQrError(error);
      }
    } finally {
      checkingLogin.value = false;
    }
  }, 2000);
};

const getQrcode = async (silent = false) => {
  if (gettingQrcode.value) return;
  gettingQrcode.value = true;
  clearTimer();
  qrStatus.value = "loading";
  qrError.value = "";
  data.qrCode = "";
  data.token = "";
  data.qrCodeString = "";
  data.tips = "使用得到App或微信扫码登录";
  timeState.time = 600;

  try {
    const result = await invokeBackend<any>("GetQrcode");
    const qrCode = normalizeQrSrc(result?.qrCode);
    const token = String(result?.token || "").trim();
    const qrCodeString = String(result?.qrCodeString || "").trim();
    if (!qrCode || !token || !qrCodeString) {
      throw new Error("二维码数据为空");
    }

    data.qrCode = qrCode;
    data.token = token;
    data.qrCodeString = qrCodeString;
    qrStatus.value = "ready";
    startCheckLoginTimer();
  } catch (error) {
    qrStatus.value = "error";
    qrError.value = buildQrError(error);
    if (!silent) {
      ElMessage({ message: qrError.value, type: "warning" });
    }
  } finally {
    gettingQrcode.value = false;
  }
};

const resetLoginState = async () => {
  clearTimer();
  resetLocalSession();
  if (!hasBackendBridge()) {
    qrStatus.value = "error";
    qrError.value = "桌面后端未就绪，请重启应用后再试";
    return;
  }
  try {
    await invokeBackend("Logout");
  } catch (error) {
    const msg = buildQrError(error);
    qrStatus.value = "error";
    qrError.value = msg;
    ElMessage({ message: `重置失败：${msg}`, type: "warning" });
    return;
  }
  ElMessage({ message: "已重置登录状态，请重新扫码", type: "success" });
  await getQrcode(true);
};

const onQrImageError = () => {
  qrStatus.value = "error";
  qrError.value = "二维码图片加载失败，请点击重试";
};

onBeforeUnmount(() => {
  clearTimer();
});

const openDialog = () => {
  dialogVisible.value = props.dialogVisible;
};

const closeDialog = () => {
  clearTimer();
  emits("close");
};
</script>

<style scoped>
.login-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 0;
}

.login-header {
  text-align: center;
  margin-bottom: 24px;
}

.title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px;
}

.subtitle {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0;
}

.qr-container {
  background: var(--bg-color);
  padding: 20px;
  border-radius: 12px;
  box-shadow: var(--shadow-inner);
  margin-bottom: 24px;
}

.qr-wrapper {
  width: 180px;
  height: 180px;
  background: white;
  padding: 10px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.qr-code-img {
  width: 100%;
  height: 100%;
}

.qr-loading {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
  font-size: 24px;
  flex-direction: column;
  gap: 8px;
}

.qr-loading span {
  font-size: 12px;
}

.qr-error {
  width: 100%;
  height: 100%;
  padding: 8px;
  box-sizing: border-box;
  text-align: center;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  color: var(--text-secondary);
}

.qr-error .el-icon {
  font-size: 20px;
  color: var(--accent-color);
}

.qr-error p {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
}

.qr-error-actions {
  display: flex;
  justify-content: center;
  gap: 6px;
}

.countdown-tip {
  margin: -12px 0 16px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.login-footer {
  width: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
}

.app-logo {
  height: 40px;
  opacity: 0.8;
}

.footer-text {
  font-size: 13px;
  color: var(--text-secondary);
}

.login-helper {
  margin-top: 14px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--fill-color-light) 76%, transparent);
  border: 1px solid color-mix(in srgb, var(--border-soft) 76%, transparent);
  padding: 10px;
  width: 100%;
  box-sizing: border-box;
  text-align: left;
}

.login-helper p {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
}

.login-helper code {
  font-size: 11px;
  color: var(--text-primary);
  background: var(--card-bg);
  border-radius: 6px;
  border: 1px solid var(--border-soft);
  padding: 4px 6px;
  display: inline-block;
}

:deep(.el-dialog) {
  border-radius: 16px;
  overflow: hidden;
  background: var(--card-bg);
  box-shadow: var(--shadow-heavy);
}

:deep(.el-dialog__header) {
  margin: 0;
  padding: 0;
}

:deep(.el-dialog__body) {
  padding: 30px;
}
</style>
