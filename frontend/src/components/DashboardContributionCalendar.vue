<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'

const CONTRIBUTION_WINDOW_DAYS = 180
const CONTRIBUTION_WEEKDAY_LABELS = ['', '一', '', '三', '', '五', '']
const CONTRIBUTION_LEVELS = [0, 1, 2, 3, 4]

const props = defineProps({
  rows: { type: Array, default: () => [] },
  formatNumber: { type: Function, required: true },
})

const contributionCalendar = computed(() =>
  buildContributionCalendar(props.rows, CONTRIBUTION_WINDOW_DAYS),
)
const contributionCalendarWeeks = computed(() => contributionCalendar.value.weeks)
const contributionCalendarSummary = computed(() => contributionCalendar.value.summary)
const contributionGridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${Math.max(1, contributionCalendarWeeks.value.length)}, var(--contribution-cell))`,
}))
const activeContributionTooltip = ref(null)

function buildContributionCalendar(rows, windowDays) {
  const usageByDate = new Map((rows || []).map((row) => [row.date, row]))
  const today = startOfLocalDay(new Date())
  const windowStart = addDays(today, -(windowDays - 1))
  const gridStart = addDays(windowStart, -windowStart.getDay())
  const gridEnd = addDays(today, 6 - today.getDay())
  const maxRequests = Math.max(
    1,
    ...Array.from({ length: windowDays }, (_, index) => {
      const key = localDateKeyFromDate(addDays(windowStart, index))
      return Number(usageByDate.get(key)?.requestCount || 0)
    }),
  )
  const weeks = []
  const summary = {
    days: windowDays,
    activeDays: 0,
    requests: 0,
    tokens: 0,
  }

  for (let cursor = new Date(gridStart); cursor <= gridEnd; cursor = addDays(cursor, 7)) {
    const week = []
    for (let dayIndex = 0; dayIndex < 7; dayIndex++) {
      const day = addDays(cursor, dayIndex)
      const date = localDateKeyFromDate(day)
      const row = usageByDate.get(date) || {}
      const outside = day < windowStart || day > today
      const requests = outside ? 0 : Number(row.requestCount || 0)
      const tokens = outside ? 0 : Number(row.totalTokens || 0)
      if (!outside) {
        summary.requests += requests
        summary.tokens += tokens
        if (requests > 0) summary.activeDays += 1
      }
      week.push({
        key: `${date}-${dayIndex}`,
        date,
        dayOfMonth: day.getDate(),
        level: contributionLevel(requests, maxRequests),
        monthKey: `${day.getFullYear()}-${day.getMonth()}`,
        monthLabel: `${day.getMonth() + 1}月`,
        outside,
        requests,
        tokens,
      })
    }
    weeks.push(week)
  }

  let lastMonthKey = ''
  const monthLabels = weeks.map((week, index) => {
    const visibleDays = week.filter((day) => !day.outside)
    const labelDay = index === 0 ? visibleDays[0] : visibleDays.find((day) => day.dayOfMonth === 1)
    if (!labelDay || labelDay.monthKey === lastMonthKey) return ''
    lastMonthKey = labelDay.monthKey
    return labelDay.monthLabel
  })

  return { monthLabels, summary, weeks }
}

function contributionLevel(value, maxValue) {
  if (value <= 0) return 0
  return Math.max(1, Math.min(4, Math.ceil((value / maxValue) * 4)))
}

function contributionDayTitle(day) {
  if (day.outside) return ''
  return `${day.date} · ${props.formatNumber(day.requests)} 次请求 · ${props.formatNumber(day.tokens)} Token`
}

function contributionTooltipPosition(event) {
  const target = event?.currentTarget
  const rect = target?.getBoundingClientRect?.()
  const rawX = rect ? rect.left + rect.width / 2 : event?.clientX || 0
  const rawY = rect ? rect.top + rect.height / 2 : event?.clientY || 0
  const viewportWidth = typeof window === 'undefined' ? 1280 : window.innerWidth
  const tooltipWidth = 230
  const margin = 16
  const x = Math.min(
    Math.max(rawX, tooltipWidth / 2 + margin),
    Math.max(tooltipWidth / 2 + margin, viewportWidth - tooltipWidth / 2 - margin),
  )

  return {
    x,
    y: rawY,
    placement: rawY < 140 ? 'below' : 'above',
  }
}

function showContributionTooltip(day, event) {
  if (day.outside) return
  activeContributionTooltip.value = {
    ...day,
    ...contributionTooltipPosition(event),
  }
}

function moveContributionTooltip(day, event) {
  if (!activeContributionTooltip.value || activeContributionTooltip.value.key !== day.key) return
  activeContributionTooltip.value = {
    ...activeContributionTooltip.value,
    ...contributionTooltipPosition(event),
  }
}

function hideContributionTooltip() {
  activeContributionTooltip.value = null
}

function isContributionTooltipActive(day) {
  return activeContributionTooltip.value?.key === day.key
}

onBeforeUnmount(hideContributionTooltip)

function startOfLocalDay(value) {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate())
}

function addDays(value, days) {
  const next = new Date(value)
  next.setDate(next.getDate() + days)
  return next
}

function localDateKeyFromDate(value) {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
</script>

<template>
  <section class="panel contribution-panel">
    <div class="section-heading contribution-heading">
      <div>
        <h2>请求活跃日历</h2>
        <p>最近 {{ contributionCalendarSummary.days }} 天代理请求活跃度</p>
      </div>
      <div class="contribution-total">
        <strong>{{ formatNumber(contributionCalendarSummary.requests) }}</strong>
        <span>次请求</span>
      </div>
    </div>

    <div
      class="contribution-calendar"
      :aria-label="`最近 ${contributionCalendarSummary.days} 天代理请求活跃日历`"
    >
      <div class="contribution-months" :style="contributionGridStyle" aria-hidden="true">
        <span
          v-for="(label, index) in contributionCalendar.monthLabels"
          :key="`contribution-month-${index}`"
        >
          {{ label }}
        </span>
      </div>
      <div class="contribution-weekdays" aria-hidden="true">
        <span v-for="(label, index) in CONTRIBUTION_WEEKDAY_LABELS" :key="`weekday-${index}`">
          {{ label }}
        </span>
      </div>
      <div class="contribution-grid" :style="contributionGridStyle">
        <div
          v-for="(week, weekIndex) in contributionCalendarWeeks"
          :key="`contribution-week-${weekIndex}`"
          class="contribution-week"
        >
          <span
            v-for="day in week"
            :key="day.key"
            :class="['contribution-day', `level-${day.level}`, { outside: day.outside }]"
            :aria-label="day.outside ? undefined : contributionDayTitle(day)"
            :aria-describedby="isContributionTooltipActive(day) ? 'contribution-tooltip' : undefined"
            :tabindex="day.outside ? undefined : 0"
            :role="day.outside ? undefined : 'img'"
            @mouseenter="showContributionTooltip(day, $event)"
            @mousemove="moveContributionTooltip(day, $event)"
            @mouseleave="hideContributionTooltip"
            @focus="showContributionTooltip(day, $event)"
            @blur="hideContributionTooltip"
          ></span>
        </div>
      </div>
    </div>

    <Teleport to="body">
      <Transition name="contribution-tooltip-fade">
        <div
          v-if="activeContributionTooltip"
          id="contribution-tooltip"
          class="contribution-tooltip"
          :class="{ below: activeContributionTooltip.placement === 'below' }"
          :style="{
            left: `${activeContributionTooltip.x}px`,
            top: `${activeContributionTooltip.y}px`,
          }"
          role="tooltip"
        >
          <div class="contribution-tooltip-date">{{ activeContributionTooltip.date }}</div>
          <div class="contribution-tooltip-metrics">
            <span>
              <strong>{{ formatNumber(activeContributionTooltip.requests) }}</strong>
              次请求
            </span>
            <span>
              <strong>{{ formatNumber(activeContributionTooltip.tokens) }}</strong>
              Token
            </span>
          </div>
          <p>{{ activeContributionTooltip.requests > 0 ? '当天有代理活动' : '当天暂无请求' }}</p>
        </div>
      </Transition>
    </Teleport>

    <div class="contribution-footer">
      <span>
        {{ formatNumber(contributionCalendarSummary.tokens) }} Token · 活跃
        {{ formatNumber(contributionCalendarSummary.activeDays) }} 天
      </span>
      <div class="contribution-legend" aria-hidden="true">
        <span>少</span>
        <i v-for="level in CONTRIBUTION_LEVELS" :key="`legend-${level}`" :class="`level-${level}`"></i>
        <span>多</span>
      </div>
    </div>
  </section>
</template>
