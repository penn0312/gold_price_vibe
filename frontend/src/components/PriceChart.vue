<script setup lang="ts">
import { GridComponent, TooltipComponent } from 'echarts/components'
import { LineChart } from 'echarts/charts'
import { CanvasRenderer } from 'echarts/renderers'
import {
  graphic,
  init,
  type ECharts,
  use
} from 'echarts/core'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

import type { Candle } from '../api/types'

use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

const props = defineProps<{
  items: Candle[]
}>()

const containerRef = ref<HTMLDivElement | null>(null)
let chart: ECharts | null = null

function renderChart() {
  if (!containerRef.value) {
    return
  }

  if (!chart) {
    chart = init(containerRef.value)
  }

  chart.setOption({
    animationDuration: 500,
    backgroundColor: 'transparent',
    grid: { top: 24, right: 18, bottom: 28, left: 54 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#ffffff',
      borderColor: '#d7dfe8',
      textStyle: { color: '#142033' }
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: props.items.map((item) => item.time.slice(11, 16)),
      axisLine: { lineStyle: { color: '#d7dfe8' } },
      axisLabel: { color: '#738095' }
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLine: { show: false },
      axisLabel: { color: '#738095' },
      splitLine: { lineStyle: { color: '#edf1f5' } }
    },
    series: [
      {
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: props.items.map((item) => item.close),
        lineStyle: {
          color: '#9f7a2b',
          width: 2
        },
        areaStyle: {
          color: new graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(177, 143, 58, 0.24)' },
            { offset: 1, color: 'rgba(177, 143, 58, 0.02)' }
          ])
        }
      }
    ]
  })
}

function handleResize() {
  chart?.resize()
}

onMounted(() => {
  renderChart()
  window.addEventListener('resize', handleResize)
})

watch(
  () => props.items,
  () => {
    renderChart()
  },
  { deep: true }
)

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  chart?.dispose()
})
</script>

<template>
  <div ref="containerRef" class="chart-shell"></div>
</template>

<style scoped>
.chart-shell {
  width: 100%;
  height: 340px;
}
</style>
