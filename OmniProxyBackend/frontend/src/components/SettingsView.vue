<script setup>
import { RefreshRight } from '@element-plus/icons-vue'

defineProps({
  config: {
    type: Object,
    required: true,
  },
  dataDirectory: {
    type: Object,
    required: true,
  },
  dataDirChanging: {
    type: Boolean,
    required: true,
  },
  autoStartChanging: {
    type: Boolean,
    required: true,
  },
  autoStartEnabled: {
    type: Boolean,
    required: true,
  },
  updateChecking: {
    type: Boolean,
    required: true,
  },
})

defineEmits([
  'persist-config',
  'choose-data-directory',
  'toggle-auto-start',
  'manual-check-for-updates',
])
</script>

<template>
  <section class="panel settings-panel">
    <div class="section-heading">
      <div>
        <h2>代理设置</h2>
        <p>保存后新请求会使用最新配置，端口变更需要重启代理</p>
      </div>
      <button type="button" class="primary-button" @click="$emit('persist-config')">保存设置</button>
    </div>
    <div class="settings-stack">
      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>应用维护</h3>
            <p>本地数据、后台常驻和版本更新集中放在这里。</p>
          </div>
        </div>
        <div class="settings-action-list">
          <div class="data-directory-row">
            <div>
              <span>数据目录</span>
              <strong>{{ dataDirectory.dataDir || '未加载' }}</strong>
              <small v-if="dataDirectory.pendingDataDir && dataDirectory.restartRequired">
                重启后使用：{{ dataDirectory.pendingDataDir }}
              </small>
              <small v-else-if="dataDirectory.envOverride">
                当前由 OMNIPROXY_DATA_DIR 环境变量控制
              </small>
              <small v-else-if="dataDirectory.bootstrapPath">
                指针文件：{{ dataDirectory.bootstrapPath }}
              </small>
            </div>
            <button
              type="button"
              class="ghost-button"
              :disabled="dataDirectory.envOverride || dataDirChanging"
              @click="$emit('choose-data-directory')"
            >
              {{ dataDirChanging ? '选择中' : '更改目录' }}
            </button>
          </div>
          <div class="data-directory-row startup-row">
            <div>
              <span>常驻后台</span>
              <strong>系统托盘与开机自启</strong>
              <small>关闭主窗口时保留托盘入口，可从托盘启动/停止代理、查看端口、打开主界面或退出。</small>
            </div>
            <button
              type="button"
              class="ghost-button"
              :disabled="autoStartChanging"
              @click="$emit('toggle-auto-start')"
            >
              {{ autoStartChanging ? '更新中' : autoStartEnabled ? '关闭自启' : '开启自启' }}
            </button>
          </div>
          <div class="data-directory-row">
            <div>
              <span>软件更新</span>
              <strong>手动检查新版本</strong>
              <small>启动时会自动检查；手动检查会忽略已经跳过的版本。</small>
            </div>
            <el-button :icon="RefreshRight" :loading="updateChecking" @click="$emit('manual-check-for-updates')">
              {{ updateChecking ? '检查中' : '检查更新' }}
            </el-button>
          </div>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>本机服务</h3>
            <p>端口和本地代理能力，端口变更后需要重启代理。</p>
          </div>
        </div>
        <div class="settings-grid compact-settings-grid">
          <label>
            <span>代理端口</span>
            <input v-model="config.proxyPort" type="number" min="1" max="65535" />
          </label>
          <label>
            <span>控制端口</span>
            <input v-model="config.controlPort" type="number" min="1" max="65535" />
          </label>
          <label class="toggle-field">
            <span>启用 Codex WebSocket</span>
            <input
              v-model="config.websocketMode"
              type="checkbox"
              true-value="enabled"
              false-value="disabled"
            />
          </label>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>调度与保护</h3>
            <p>控制账号轮换、低额度跳过和失败重试。</p>
          </div>
        </div>
        <div class="settings-grid compact-settings-grid">
          <label>
            <span>账号调度模式</span>
            <select v-model="config.schedulingMode">
              <option value="queue">队列模式</option>
              <option value="balanced">优先平衡使用</option>
            </select>
          </label>
          <label>
            <span>额度切换阈值</span>
            <input v-model="config.switchThreshold" type="number" min="1" max="100" />
          </label>
          <label>
            <span>自动重试次数</span>
            <input v-model="config.maxRetries" type="number" min="0" max="5" />
          </label>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>OpenAI / Anthropic / Codex</h3>
            <p>常用协议入口和 Codex 额度查询地址。</p>
          </div>
        </div>
        <div class="settings-grid">
          <label class="wide-field">
            <span>OpenAI API Base URL</span>
            <input v-model="config.openaiBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Anthropic API Base URL</span>
            <input v-model="config.anthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Codex ChatGPT Base URL</span>
            <input v-model="config.codexBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Codex 限额查询地址</span>
            <input v-model="config.codexUsageEndpoint" type="url" />
          </label>
          <label class="wide-field">
            <span>兼容旧版上游 API Base URL</span>
            <input v-model="config.upstreamBaseUrl" type="url" />
          </label>
        </div>
      </section>

      <section class="settings-section">
        <div class="settings-section-head">
          <div>
            <h3>第三方路由</h3>
            <p>DeepSeek、Kimi 和 Xiaomi MiMo 的 OpenAI / Anthropic 兼容入口。</p>
          </div>
        </div>
        <div class="settings-grid">
          <label class="wide-field">
            <span>DeepSeek API Base URL</span>
            <input v-model="config.deepseekBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>DeepSeek Anthropic Base URL</span>
            <input v-model="config.deepseekAnthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Kimi Code Base URL</span>
            <input v-model="config.kimiBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo 按量 OpenAI Base URL</span>
            <input v-model="config.xiaomiApiBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo 按量 Anthropic Base URL</span>
            <input v-model="config.xiaomiApiAnthropicBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo Token Plan OpenAI Base URL</span>
            <input v-model="config.xiaomiTokenPlanBaseUrl" type="url" />
          </label>
          <label class="wide-field">
            <span>Xiaomi MiMo Token Plan Anthropic Base URL</span>
            <input v-model="config.xiaomiTokenPlanAnthropicBaseUrl" type="url" />
          </label>
        </div>
      </section>
    </div>
  </section>
</template>
