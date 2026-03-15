<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    lines?: number
    chart?: boolean
  }>(),
  {
    lines: 3,
    chart: false
  }
)
</script>

<template>
  <div class="skeleton-panel" :class="props.chart ? 'skeleton-chart' : ''">
    <div class="skeleton-line short"></div>
    <div v-for="line in props.lines" :key="line" class="skeleton-line"></div>
  </div>
</template>

<style scoped>
.skeleton-panel {
  display: grid;
  gap: 12px;
  min-height: 180px;
  align-content: center;
}

.skeleton-chart {
  min-height: 320px;
}

.skeleton-line {
  height: 14px;
  border-radius: 999px;
  background: linear-gradient(90deg, rgba(233, 238, 244, 0.9), rgba(248, 250, 252, 1), rgba(233, 238, 244, 0.9));
  background-size: 220% 100%;
  animation: shimmer 1.6s linear infinite;
}

.short {
  width: 34%;
}

.skeleton-chart .skeleton-line:nth-child(2) {
  height: 180px;
  border-radius: 24px;
}

@keyframes shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -20% 0;
  }
}
</style>
