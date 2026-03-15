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

import type { AccuracyItem } from '../api/types'

use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

const props = defineProps<{
  items: AccuracyItem[]
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
    grid: { top: 20, right: 18, bottom: 28, left: 42 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#ffffff',
      borderColor: '#d7dfe8',
      textStyle: { color: '#142033' }
    },
    xAxis: {
      type: 'category',
      data: props.items.map((item) => item.report_date.slice(5)),
      axisLine: { lineStyle: { color: '#d7dfe8' } },
      axisLabel: { color: '#738095' }
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 100,
      splitLine: { lineStyle: { color: '#edf1f5' } },
      axisLabel: { color: '#738095' }
    },
    series: [
      {
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: props.items.map((item) => item.score),
        lineStyle: {
          color: '#1f4f95',
          width: 2
        },
        areaStyle: {
          color: new graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(31, 79, 149, 0.18)' },
            { offset: 1, color: 'rgba(31, 79, 149, 0.01)' }
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
  height: 280px;
}
</style>
