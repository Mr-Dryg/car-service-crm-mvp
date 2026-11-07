<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api, getToken, userState } from '../api'
import { formatDate, formatTime, LOAD_STYLES, STATUS_LABELS, STATUS_STYLES, todayISO } from '../utils'

const user = computed(() => userState.value)

const orders = ref([])
const slots = ref(null)
const loading = ref(true)
const error = ref('')
const busyId = ref(0)
const connected = ref(false)

const selectedDate = ref(todayISO())
const statusFilter = ref('')
const search = ref('')

/** Секция «Красная зона» — отменённые клиентами записи. */
const canceled = computed(() => orders.value.filter((o) => o.status === 'canceled'))

/** Активные записи, отсортированные по времени. */
const active = computed(() =>
  orders.value
    .filter((o) => o.status !== 'canceled')
    .sort((a, b) => `${a.preferred_date}${a.preferred_time}`.localeCompare(`${b.preferred_date}${b.preferred_time}`)),
)

/** Доступные действия по статусу заказа. */
const NEXT_STATUS = {
  new: [{ value: 'confirmed', label: 'Подтвердить', cls: 'btn-primary' }],
  confirmed: [{ value: 'in_progress', label: 'В работу', cls: 'btn-primary' }],
  in_progress: [{ value: 'ready', label: 'Готово + SMS', cls: 'btn-success' }],
  ready: [{ value: 'completed', label: 'Выдана', cls: 'btn-primary' }],
  completed: [],
  canceled: [],
}

let ws = null
let reconnectTimer = null
let sound = null

onMounted(async () => {
  await Promise.all([loadOrders(), loadSlots()])
  connectWS()
  enableSound()
})

onUnmounted(() => {
  if (ws) ws.close()
  if (reconnectTimer) clearTimeout(reconnectTimer)
})

// Перечитываем данные при смене фильтров.
watch([selectedDate, statusFilter, search], loadOrders)

// --- WebSocket: события в реальном времени ---

function connectWS() {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  // Браузер не позволяет передать заголовок Authorization при открытии WebSocket,
  // поэтому токен передаётся в query-параметре и проверяется на сервере.
  const token = getToken()
  ws = new WebSocket(`${proto}://${window.location.host}/api/manager/ws?token=${encodeURIComponent(token)}`)

  ws.onopen = () => {
    connected.value = true
  }
  ws.onclose = () => {
    connected.value = false
    // Переподключаемся только пока сессия менеджера действительна.
    // Если токен истёк или роль сменилась, WS-сервер отклонит подключение,
    // и бессмысленные попытки прекратятся.
    if (!getToken() || user.value?.role !== 'manager') {
      return
    }
    reconnectTimer = setTimeout(connectWS, 3000)
  }
  ws.onerror = () => ws.close()
  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      handleEvent(data)
    } catch {
      /* игнорируем некорректные сообщения */
    }
  }
}

function handleEvent(data) {
  // Звуковое уведомление о новой заявке и об отмене.
  if (data.type === 'order.new' || data.type === 'order.canceled') {
    playSound()
  }
  loadOrders()
  loadSlots()
}

/** Разрешает воспроизведение звука после первого взаимодействия пользователя. */
function enableSound() {
  const unlock = () => {
    sound = new Audio('/notify.wav')
    window.removeEventListener('click', unlock)
    window.removeEventListener('keydown', unlock)
  }
  window.addEventListener('click', unlock, { once: true })
  window.addEventListener('keydown', unlock, { once: true })
}

function playSound() {
  if (!sound) return
  sound.currentTime = 0
  sound.play().catch(() => {
    /* звук недоступен — не критично */
  })
}

// --- Загрузка данных ---

async function loadOrders() {
  try {
    orders.value = await api.managerOrders({
      date: selectedDate.value,
      status: statusFilter.value,
      search: search.value,
    })
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function loadSlots() {
  try {
    slots.value = await api.managerSlots(selectedDate.value)
  } catch (e) {
    error.value = e.message
  }
}

// --- Действия ---

async function changeStatus(order, status) {
  busyId.value = order.id
  error.value = ''
  try {
    await api.changeStatus(order.id, status)
    await Promise.all([loadOrders(), loadSlots()])
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = 0
  }
}

/** Окончательное удаление отменённой записи — освобождает слот. */
async function removeOrder(order) {
  if (!window.confirm('Удалить запись окончательно? Слот будет освобождён.')) return
  busyId.value = order.id
  error.value = ''
  try {
    await api.deleteOrder(order.id)
    await Promise.all([loadOrders(), loadSlots()])
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = 0
  }
}

/** Восстановление записи с возможностью смены даты и времени. */
const restoreTarget = ref(null)
const restoreForm = ref({ preferred_date: '', preferred_time: '' })

function openRestore(order) {
  restoreTarget.value = order
  restoreForm.value = { preferred_date: order.preferred_date, preferred_time: order.preferred_time }
}

async function submitRestore() {
  if (!restoreTarget.value) return
  busyId.value = restoreTarget.value.id
  error.value = ''
  try {
    await api.restoreOrder(restoreTarget.value.id, restoreForm.value)
    restoreTarget.value = null
    await Promise.all([loadOrders(), loadSlots()])
  } catch (e) {
    error.value = e.message
  } finally {
    busyId.value = 0
  }
}
</script>

<template>
  <div class="mx-auto w-full max-w-7xl px-4 py-6">
    <!-- Заголовок -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-slate-900">CRM филиала</h1>
        <p class="mt-1 text-sm text-slate-500">
          {{ user?.name }} · записи на {{ formatDate(selectedDate) }}
        </p>
      </div>
      <div class="flex items-center gap-2 text-xs">
        <span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 font-semibold"
          :class="connected ? 'bg-emerald-100 text-emerald-700' : 'bg-slate-200 text-slate-600'">
          <span class="h-2 w-2 rounded-full" :class="connected ? 'bg-emerald-500 pulse-dot' : 'bg-slate-400'"></span>
          {{ connected ? 'Онлайн' : 'Нет связи' }}
        </span>
      </div>
    </div>

    <div v-if="error" class="mt-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700" role="alert">
      {{ error }}
    </div>

    <!-- Интеллектуальный календарь: цветовая шкала дня -->
    <section v-if="slots" class="card mt-5">
      <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500">Загрузка филиала</h2>
        <div class="flex flex-wrap items-center gap-2">
          <span class="badge shrink-0" :class="LOAD_STYLES[slots.load_level].chip">
            {{ LOAD_STYLES[slots.load_level].label }} · {{ slots.load_pct }}%
          </span>
          <span class="text-xs text-slate-500">{{ slots.total }} записей / {{ slots.capacity }} боксов</span>
        </div>
      </div>

      <!-- Таймлайн часов -->
      <div class="-mx-1 mt-4 overflow-x-auto px-1 pb-1">
        <div class="grid min-w-[420px] grid-cols-6 gap-1.5 sm:min-w-0 md:grid-cols-12">
          <div v-for="s in slots.slots" :key="s.time"
            class="rounded-lg px-1 py-2 text-center"
            :class="LOAD_STYLES[s.load_level].bar + ' ' + (s.orders ? 'text-white' : 'bg-slate-100 text-slate-400')"
            :title="`${s.time} — ${s.orders} из ${s.capacity} боксов (${s.load_pct}%)`">
            <p class="text-[10px] font-semibold leading-tight sm:text-[11px]">{{ s.time }}</p>
            <p class="text-sm font-bold leading-tight">{{ s.orders }}</p>
          </div>
        </div>
      </div>

      <div class="mt-3 flex flex-wrap gap-4 text-xs text-slate-500">
        <span class="inline-flex items-center gap-1.5">
          <span class="h-3 w-3 rounded bg-emerald-500"></span> до 30% — зелёный
        </span>
        <span class="inline-flex items-center gap-1.5">
          <span class="h-3 w-3 rounded bg-amber-500"></span> 30–70% — жёлтый
        </span>
        <span class="inline-flex items-center gap-1.5">
          <span class="h-3 w-3 rounded bg-red-500"></span> более 70% — красный
        </span>
      </div>
    </section>

    <!-- Фильтры -->
    <section class="card mt-5">
      <div class="grid gap-3 sm:grid-cols-3">
        <div>
          <label class="label" for="filter-date">Дата</label>
          <input id="filter-date" v-model="selectedDate" class="input" type="date" />
        </div>
        <div>
          <label class="label" for="filter-status">Статус</label>
          <select id="filter-status" v-model="statusFilter" class="input">
            <option value="">Все статусы</option>
            <option v-for="(label, key) in STATUS_LABELS" :key="key" :value="key">{{ label }}</option>
          </select>
        </div>
        <div>
          <label class="label" for="filter-search">Поиск</label>
          <input id="filter-search" v-model="search" class="input" type="search" placeholder="Имя, телефон, госномер" />
        </div>
      </div>
    </section>

    <!-- Красная зона отмен -->
    <section v-if="canceled.length" class="mt-5 overflow-hidden rounded-2xl border-2 border-red-300 bg-red-50">
      <div class="flex items-center gap-2 bg-red-600 px-4 py-3 text-white">
        <span class="text-lg">🚨</span>
        <h2 class="font-bold">Отменено клиентами</h2>
        <span class="badge bg-red-800 text-white">{{ canceled.length }}</span>
      </div>
      <div class="space-y-2 p-3 sm:p-4">
        <article v-for="o in canceled" :key="o.id" class="rounded-xl border border-red-200 bg-white p-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p class="font-semibold text-slate-900">{{ o.client_name }} · {{ o.car_number }}</p>
              <p class="text-sm text-slate-500">
                {{ o.service_type }} · {{ formatDate(o.preferred_date) }}, {{ formatTime(o.preferred_time) }}
              </p>
              <a :href="`tel:${o.client_phone}`" class="mt-1 inline-block text-sm font-medium text-brand-600 hover:underline">
                {{ o.client_phone }}
              </a>
            </div>
            <div class="flex gap-2">
              <button class="btn-secondary btn-sm" type="button" :disabled="busyId === o.id" @click="openRestore(o)">
                Восстановить
              </button>
              <button class="btn-danger btn-sm" type="button" :disabled="busyId === o.id" @click="removeOrder(o)">
                Удалить
              </button>
            </div>
          </div>
        </article>
      </div>
    </section>

    <!-- Список записей -->
    <section class="mt-5">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500">
        Записи ({{ active.length }})
      </h2>

      <div v-if="loading" class="mt-4 text-center text-sm text-slate-400">Загрузка…</div>

      <div v-else-if="!active.length" class="card mt-3 text-center text-sm text-slate-500">
        На выбранную дату записей нет.
      </div>

      <div v-else class="mt-3 space-y-3">
        <article v-for="o in active" :key="o.id" class="card">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="font-bold text-slate-900">
                <span class="text-slate-400">#{{ o.id }}</span> {{ o.client_name }}
              </p>
              <p class="text-sm text-slate-600">{{ o.car_number }} · {{ o.service_type }}</p>
              <a :href="`tel:${o.client_phone}`" class="text-sm font-medium text-brand-600 hover:underline">
                {{ o.client_phone }}
              </a>
            </div>
            <div class="flex flex-col items-end gap-2">
              <span class="badge" :class="STATUS_STYLES[o.status]">{{ STATUS_LABELS[o.status] }}</span>
              <span class="text-sm font-semibold text-slate-700">
                {{ formatDate(o.preferred_date) }}, {{ formatTime(o.preferred_time) }}
              </span>
            </div>
          </div>

          <p v-if="o.comment" class="mt-3 rounded-lg bg-slate-50 px-3 py-2 text-sm text-slate-600">
            💬 {{ o.comment }}
          </p>

          <!-- Клиент подтвердил, что едет — звонить не нужно -->
          <div v-if="o.is_client_en_route"
            class="mt-3 flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm font-semibold text-emerald-800">
            🚗 Клиент едет — звонить не нужно
          </div>

          <div class="mt-4 flex flex-wrap gap-2">
            <button v-for="a in NEXT_STATUS[o.status]" :key="a.value" :class="[a.cls, 'btn-sm']" type="button"
              :disabled="busyId === o.id" @click="changeStatus(o, a.value)">
              {{ a.label }}
            </button>
          </div>
        </article>
      </div>
    </section>

    <!-- Модальное окно восстановления -->
    <div v-if="restoreTarget" class="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4"
      @click.self="restoreTarget = null">
      <div class="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl">
        <h3 class="text-lg font-bold text-slate-900">Восстановить запись</h3>
        <p class="mt-1 text-sm text-slate-500">
          Запись #{{ restoreTarget.id }} · {{ restoreTarget.client_name }}
        </p>

        <div class="mt-4 space-y-3">
          <div>
            <label class="label" for="restore-date">Дата</label>
            <input id="restore-date" v-model="restoreForm.preferred_date" class="input" type="date" />
          </div>
          <div>
            <label class="label" for="restore-time">Время</label>
            <input id="restore-time" v-model="restoreForm.preferred_time" class="input" type="time" />
          </div>
        </div>

        <div class="mt-5 flex gap-2">
          <button class="btn-primary flex-1" type="button" :disabled="busyId === restoreTarget.id" @click="submitRestore">
            Восстановить
          </button>
          <button class="btn-secondary" type="button" @click="restoreTarget = null">Отмена</button>
        </div>
      </div>
    </div>
  </div>
</template>
