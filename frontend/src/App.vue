<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref } from 'vue'

import { getAccuracyCurve, getDashboardOverview, getPriceHistory } from './api/client'
import type { AccuracyCurve, DashboardOverview, PriceHistory } from './api/types'

const PriceChart = defineAsyncComponent(() => import('./components/PriceChart.vue'))
const AccuracyChart = defineAsyncComponent(() => import('./components/AccuracyChart.vue'))

const loading = ref(true)
const errorMessage = ref('')
const overview = ref<DashboardOverview | null>(null)
const history = ref<PriceHistory | null>(null)
const accuracy = ref<AccuracyCurve | null>(null)
let refreshTimer: number | undefined

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

async function loadData() {
  loading.value = true
  errorMessage.value = ''

  try {
    const [overviewPayload, historyPayload, accuracyPayload] = await Promise.all([
      getDashboardOverview(),
      getPriceHistory('1d', '1m'),
      getAccuracyCurve('30d')
    ])

    overview.value = overviewPayload
    history.value = historyPayload
    accuracy.value = accuracyPayload
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '加载失败'
  } finally {
    loading.value = false
  }
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
      <button class="refresh-button" type="button" @click="loadData">刷新数据</button>
    </section>

    <section v-if="loading" class="status-card">正在加载市场数据...</section>
    <section v-else-if="errorMessage" class="status-card error-card">
      接口暂时不可用：{{ errorMessage }}
    </section>

    <template v-else-if="overview && history && accuracy">
      <section class="grid-top">
        <article class="card price-card">
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
        </article>

        <article class="card report-card">
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
        <PriceChart :items="history.items" />
      </section>

      <section class="content-grid">
        <article class="card">
          <div class="section-head">
            <div>
              <p class="card-label">核心因子</p>
              <h2>黄金驱动面板</h2>
            </div>
          </div>
          <div class="factor-grid">
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
          </div>
          <div class="news-list">
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
          <strong class="score-pill">平均 {{ accuracy.avg_score.toFixed(1) }}</strong>
        </div>
        <AccuracyChart :items="accuracy.items" />
      </section>
    </template>
  </main>
</template>
