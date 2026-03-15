<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref } from 'vue'

import { getAccuracyCurve, getDashboardOverview, getPriceHistory } from './api/client'
import type { AccuracyCurve, DashboardOverview, PriceHistory } from './api/types'

const PriceChart = defineAsyncComponent(() => import('./components/PriceChart.vue'))
const AccuracyChart = defineAsyncComponent(() => import('./components/AccuracyChart.vue'))
const StatePanel = defineAsyncComponent(() => import('./components/StatePanel.vue'))
const SkeletonPanel = defineAsyncComponent(() => import('./components/SkeletonPanel.vue'))

const overview = ref<DashboardOverview | null>(null)
const history = ref<PriceHistory | null>(null)
const accuracy = ref<AccuracyCurve | null>(null)
const overviewLoading = ref(true)
const historyLoading = ref(true)
const accuracyLoading = ref(true)
const overviewError = ref('')
const historyError = ref('')
const accuracyError = ref('')
let refreshTimer: number | undefined

const isRefreshing = computed(() => overviewLoading.value || historyLoading.value || accuracyLoading.value)

const trendLabel = computed(() => {
  if (!overview.value) {
    return ''
  }

  const trend = overview.value.latest_report.trend
  if (trend === 'bullish') return '偏多'
  if (trend === 'bearish') return '偏空'
  if (trend === 'range') return '区间'
  return '震荡'
})

const hasPriceHistory = computed(() => Boolean(history.value && history.value.items.length > 0))
const hasAccuracyHistory = computed(() => Boolean(accuracy.value && accuracy.value.items.length > 0))
const hasFactors = computed(() => Boolean(overview.value && overview.value.factors.length > 0))
const hasHeadlines = computed(() => Boolean(overview.value && overview.value.headlines.length > 0))

async function loadOverview() {
  overviewLoading.value = true
  overviewError.value = ''

  try {
    overview.value = await getDashboardOverview()
  } catch (error) {
    overviewError.value = error instanceof Error ? error.message : '加载失败'
  } finally {
    overviewLoading.value = false
  }
}

async function loadHistory() {
  historyLoading.value = true
  historyError.value = ''

  try {
    history.value = await getPriceHistory('1d', '1m')
  } catch (error) {
    historyError.value = error instanceof Error ? error.message : '加载失败'
  } finally {
    historyLoading.value = false
  }
}

async function loadAccuracy() {
  accuracyLoading.value = true
  accuracyError.value = ''

  try {
    accuracy.value = await getAccuracyCurve('30d')
  } catch (error) {
    accuracyError.value = error instanceof Error ? error.message : '加载失败'
  } finally {
    accuracyLoading.value = false
  }
}

async function loadData() {
  await Promise.all([loadOverview(), loadHistory(), loadAccuracy()])
}

onMounted(() => {
  loadData()
  refreshTimer = window.setInterval(loadData, 30000)
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
})
</script>

<template>
  <main class="page-shell">
    <section class="hero">
      <div class="hero-copy">
        <p class="eyebrow">Gold Insight Console</p>
        <h1>黄金价格走势分析</h1>
        <p class="hero-text">
          面向人民币/克金价的实时追踪、因子监控、新闻事件归因、AI 趋势判断与准确率复盘。
        </p>
      </div>
      <button class="refresh-button" type="button" :disabled="isRefreshing" @click="loadData">
        {{ isRefreshing ? '刷新中...' : '刷新数据' }}
      </button>
    </section>

    <section class="grid-top">
      <article class="card price-card">
        <SkeletonPanel v-if="overviewLoading && !overview" :lines="4" />
        <StatePanel
          v-else-if="overviewError && !overview"
          title="实时价格暂不可用"
          :description="overviewError"
          tone="error"
          action-label="重试价格概览"
          @action="loadOverview"
        />
        <template v-else-if="overview">
          <p class="card-label">实时金价</p>
          <div class="price-row">
            <strong>{{ overview.realtime_price.price.toFixed(3) }}</strong>
            <span>{{ overview.realtime_price.currency }}/{{ overview.realtime_price.unit }}</span>
          </div>
          <p
            class="delta"
            :class="overview.realtime_price.change_amount >= 0 ? 'positive' : 'negative'"
          >
            {{ overview.realtime_price.change_amount >= 0 ? '+' : '' }}{{
              overview.realtime_price.change_amount.toFixed(3)
            }}
            ({{ overview.realtime_price.change_rate.toFixed(2) }}%)
          </p>
          <p class="meta">更新时间 {{ overview.realtime_price.captured_at.replace('T', ' ').slice(0, 19) }}</p>
        </template>
      </article>

      <article class="card report-card">
        <SkeletonPanel v-if="overviewLoading && !overview" :lines="4" />
        <StatePanel
          v-else-if="overviewError && !overview"
          title="AI 结论加载失败"
          :description="overviewError"
          tone="error"
          action-label="重试报告概览"
          @action="loadOverview"
        />
        <template v-else-if="overview">
          <p class="card-label">最新 AI 结论</p>
          <div class="report-head">
            <strong>{{ trendLabel }}</strong>
            <span>置信度 {{ overview.latest_report.confidence.toFixed(0) }}%</span>
          </div>
          <h2>{{ overview.latest_report.title }}</h2>
          <p class="report-summary">{{ overview.latest_report.summary }}</p>
          <div class="chip-row">
            <span v-for="item in overview.latest_report.key_drivers" :key="item" class="chip">
              {{ item }}
            </span>
          </div>
        </template>
      </article>
    </section>

    <section class="card chart-card">
      <div class="section-head">
        <div>
          <p class="card-label">实时走势图</p>
          <h2>人民币/克 1 日分时</h2>
        </div>
        <span class="muted">样例阶段已接通后端 API 骨架</span>
      </div>
      <SkeletonPanel v-if="historyLoading && !history" chart :lines="2" />
      <StatePanel
        v-else-if="historyError && !history"
        title="价格图表加载失败"
        :description="historyError"
        tone="error"
        action-label="重试图表"
        @action="loadHistory"
      />
      <StatePanel
        v-else-if="history && !hasPriceHistory"
        title="暂未生成价格曲线"
        description="当前时间窗口内还没有可展示的分时数据，系统会在后续采集后自动补齐。"
      />
      <PriceChart v-else-if="history" :items="history.items" />
    </section>

    <section class="content-grid">
      <article class="card">
        <div class="section-head">
          <div>
            <p class="card-label">核心因子</p>
            <h2>黄金驱动面板</h2>
          </div>
          <span v-if="overviewError && overview" class="status-pill">刷新失败，展示上次结果</span>
        </div>
        <SkeletonPanel v-if="overviewLoading && !overview" :lines="4" />
        <StatePanel
          v-else-if="overviewError && !overview"
          title="因子面板加载失败"
          :description="overviewError"
          tone="error"
          action-label="重试因子"
          @action="loadOverview"
        />
        <StatePanel
          v-else-if="overview && !hasFactors"
          title="暂无因子快照"
          description="系统尚未生成最新因子数据，完成下一轮更新后会自动展示。"
        />
        <div v-else-if="overview" class="factor-grid">
          <div v-for="item in overview.factors" :key="item.code" class="factor-item">
            <div class="factor-top">
              <strong>{{ item.name }}</strong>
              <span :class="item.score >= 0 ? 'positive' : 'negative'">{{ item.score.toFixed(1) }}</span>
            </div>
            <p class="factor-value">{{ item.value }} {{ item.unit }}</p>
            <p class="meta">
              {{ item.impact_direction === 'bullish' ? '利多' : item.impact_direction === 'bearish' ? '利空' : '中性' }}
              · 强度 {{ item.impact_strength.toFixed(0) }}
            </p>
          </div>
        </div>
      </article>

      <article class="card">
        <div class="section-head">
          <div>
            <p class="card-label">新闻事件</p>
            <h2>最新影响事件</h2>
          </div>
          <span v-if="overviewError && overview" class="status-pill">刷新失败，展示上次结果</span>
        </div>
        <SkeletonPanel v-if="overviewLoading && !overview" :lines="5" />
        <StatePanel
          v-else-if="overviewError && !overview"
          title="新闻列表加载失败"
          :description="overviewError"
          tone="error"
          action-label="重试新闻"
          @action="loadOverview"
        />
        <StatePanel
          v-else-if="overview && !hasHeadlines"
          title="暂未抓到影响事件"
          description="当前暂无可展示的新闻事件，新闻抓取任务完成后会自动补齐。"
        />
        <div v-else-if="overview" class="news-list">
          <a
            v-for="item in overview.headlines"
            :key="item.id"
            class="news-item"
            :href="item.url"
            target="_blank"
            rel="noreferrer"
          >
            <div class="news-meta">
              <span>{{ item.region }}</span>
              <span>重要性 {{ item.importance }}</span>
            </div>
            <strong>{{ item.title }}</strong>
            <p>{{ item.summary }}</p>
          </a>
        </div>
      </article>
    </section>

    <section class="card">
      <div class="section-head">
        <div>
          <p class="card-label">预测复盘</p>
          <h2>历史准确率曲线</h2>
        </div>
        <strong v-if="accuracy && hasAccuracyHistory" class="score-pill">平均 {{ accuracy.avg_score.toFixed(1) }}</strong>
      </div>
      <SkeletonPanel v-if="accuracyLoading && !accuracy" chart :lines="2" />
      <StatePanel
        v-else-if="accuracyError && !accuracy"
        title="准确率曲线加载失败"
        :description="accuracyError"
        tone="error"
        action-label="重试准确率"
        @action="loadAccuracy"
      />
      <StatePanel
        v-else-if="accuracy && !hasAccuracyHistory"
        title="暂无评分历史"
        description="系统尚未积累到可展示的准确率记录，完成新一轮评分后会自动展示。"
      />
      <AccuracyChart v-else-if="accuracy" :items="accuracy.items" />
    </section>
  </main>
</template>
