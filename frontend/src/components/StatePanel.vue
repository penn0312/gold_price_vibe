<script setup lang="ts">
defineProps<{
  title: string
  description: string
  tone?: 'neutral' | 'error'
  actionLabel?: string
}>()

defineEmits<{
  action: []
}>()
</script>

<template>
  <div class="state-panel" :class="tone === 'error' ? 'state-panel-error' : ''">
    <p class="state-kicker">{{ tone === 'error' ? 'Service Notice' : 'Data Notice' }}</p>
    <strong>{{ title }}</strong>
    <p>{{ description }}</p>
    <button v-if="actionLabel" class="state-action" type="button" @click="$emit('action')">
      {{ actionLabel }}
    </button>
  </div>
</template>

<style scoped>
.state-panel {
  display: grid;
  gap: 10px;
  min-height: 180px;
  align-content: center;
  justify-items: start;
  text-align: left;
  padding: 18px;
  border-radius: 22px;
  border: 1px dashed #d8e0e8;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(248, 250, 252, 0.96));
}

.state-panel strong {
  font-size: 18px;
  line-height: 1.3;
}

.state-panel p {
  margin: 0;
  color: #738095;
  line-height: 1.6;
}

.state-kicker {
  font-size: 12px;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.state-panel-error {
  border-color: rgba(171, 92, 67, 0.22);
  background: linear-gradient(180deg, rgba(255, 251, 249, 0.98), rgba(255, 246, 242, 0.96));
}

.state-panel-error .state-kicker,
.state-panel-error strong {
  color: #9c3f29;
}

.state-action {
  border: 1px solid #d7dfe8;
  background: #ffffff;
  border-radius: 999px;
  padding: 10px 16px;
  cursor: pointer;
}
</style>
