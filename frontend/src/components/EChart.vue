<template>
  <div ref="el" class="echart" :style="{ height: typeof height === 'number' ? `${height}px` : height }" />
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'

const props = defineProps({
  option: { type: Object, required: true },
  height: { type: [Number, String], default: 260 }
})

const el = ref(null)
let chart = null
let observer = null

function render() {
  if (!chart) return
  chart.setOption(props.option, true)
}

onMounted(() => {
  chart = echarts.init(el.value, null, { renderer: 'canvas' })
  render()
  observer = new ResizeObserver(() => chart && chart.resize())
  observer.observe(el.value)
})

watch(
  () => props.option,
  () => render(),
  { deep: true }
)

onBeforeUnmount(() => {
  if (observer) observer.disconnect()
  if (chart) chart.dispose()
  chart = null
})
</script>

<style scoped>
.echart {
  width: 100%;
}
</style>
