<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'
import { formatDate, formatTime, STATUS_LABELS, STATUS_STYLES } from '../utils'

const orders = ref([])
const loading = ref(true)
const error = ref('')
const busyId = ref(0)

/** Заказы в работе и ожидающие — показываем первыми. */
const sorted = computed(() => {
  const rank = { ready: 0, in_progress: 1, new: 2, confirmed: 3, canceled: 4, completed: 5 }
  return [...orders.value].sort((a, b) => {
    const r = (rank[a.status] ?? 9) - (rank[b.status] ?? 9)
    if (r !== 0) return r
    return `${a.preferred_date}${a.preferred_time}`.localeCompare(`${b.preferred_date}${b.preferred_time}`)
  })
})

const active = computed(() => orders.value.filter((o) => !['completed', 'canceled'].includes(o.status)))
const history = computed(() => orders.value.filter((o) => ['completed', 'canceled'].includes(o.status)))

onMounted(load)

async function load() {
  loading.value = true
  try {
    orders.value = await api.clientOrders()
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

/** Кнопка «Подтверждаю, скоро буду» — доступна только в статусе ready. */
async function confirmEnRoute(order) {
  busyId.value = order.id
  error.value = ''
  try {
    const updated = await api.enRoute(order.id)
    replaceOrder(updated)
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = 0
  }
}

/** Отмена записи — пока ремонт не начался. */
async function cancel(order) {
  if (!window.confirm('Отменить запись на ремонт?')) return
  busyId.value = order.id
  error.value = ''
  try {
    const updated = await api.cancelOrder(order.id)
    replaceOrder(updated)
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = 0
  }
}

function replaceOrder(updated) {
  const idx = orders.value.findIndex((o) => o.id === updated.id)
  if (idx >= 0) orders.value[idx] = updated
}
</script>

<template>
  <div class="mx-auto w-full max-w-4xl px-4 py-6 sm:py-10">
    <h1 class="text-2xl font-bold text-slate-900 sm:text-3xl">Мои записи</h1>
    <p class="mt-2 text-sm text-slate-500">Статус ремонта ваших автомобилей</p>

    <div v-if="error" class="mt-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700" role="alert">
      {{ error }}
    </div>

    <div v-if="loading" class="mt-8 text-center text-sm text-slate-400">Загрузка…</div>

    <div v-else-if="!orders.length" class="card mt-6 text-center">
      <p class="text-sm text-slate-500">У вас пока нет записей.</p>
      <RouterLink to="/" class="btn-primary mt-4">Записаться на ремонт</RouterLink>
    </div>

    <template v-else>
      <!-- Активные записи -->
      <section class="mt-6">
        <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500">Активные записи</h2>
        <div class="mt-3 space-y-3">
          <article v-for="o in active" :key="o.id" class="card">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p class="font-bold text-slate-900">{{ o.car_number }}</p>
                <p class="text-sm text-slate-600">{{ o.service_type }}</p>
              </div>
              <span class="badge" :class="STATUS_STYLES[o.status]">{{ STATUS_LABELS[o.status] }}</span>
            </div>

            <dl class="mt-4 grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
              <div>
                <dt class="text-xs text-slate-400">Дата</dt>
                <dd class="font-medium text-slate-800">{{ formatDate(o.preferred_date) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-slate-400">Время</dt>
                <dd class="font-medium text-slate-800">{{ formatTime(o.preferred_time) }}</dd>
              </div>
              <div class="col-span-2 sm:col-span-1">
                <dt class="text-xs text-slate-400">Филиал</dt>
                <dd class="font-medium text-slate-800">{{ o.branch_address }}</dd>
              </div>
            </dl>

            <p v-if="o.comment" class="mt-3 rounded-lg bg-slate-50 px-3 py-2 text-sm text-slate-600">
              {{ o.comment }}
            </p>

            <!-- Статус «клиент едет» -->
            <div v-if="o.is_client_en_route"
              class="mt-4 flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-800">
              <span class="text-lg">🚗</span>
              <span>Вы едете. Менеджер уведомлён — звонить не будем.</span>
            </div>

            <div class="mt-4 flex flex-wrap gap-2">
              <!-- Кнопка сценария «Я в пути» -->
              <button v-if="o.status === 'ready' && !o.is_client_en_route" class="btn-success flex-1 sm:flex-none"
                type="button" :disabled="busyId === o.id" @click="confirmEnRoute(o)">
                ✓ Подтверждаю, скоро буду
              </button>

              <button v-if="['new', 'confirmed'].includes(o.status)" class="btn-secondary" type="button"
                :disabled="busyId === o.id" @click="cancel(o)">
                Отменить
              </button>
            </div>
          </article>
        </div>
      </section>

      <!-- История -->
      <section v-if="history.length" class="mt-8">
        <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500">История</h2>
        <div class="mt-3 space-y-3">
          <article v-for="o in history" :key="o.id" class="card opacity-80">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p class="font-semibold text-slate-800">{{ o.car_number }} · {{ o.service_type }}</p>
                <p class="text-sm text-slate-500">
                  {{ formatDate(o.preferred_date) }}, {{ formatTime(o.preferred_time) }}
                </p>
              </div>
              <span class="badge" :class="STATUS_STYLES[o.status]">{{ STATUS_LABELS[o.status] }}</span>
            </div>
          </article>
        </div>
      </section>
    </template>
  </div>
</template>
