<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { formatDate, LOAD_STYLES, STATUS_LABELS } from '../utils'

const dashboard = ref(null)
const branches = ref([])
const loading = ref(true)
const error = ref('')
const success = ref('')

const newBranch = ref({ address: '' })
const newManager = ref({ phone: '', password: '', name: '', branch_id: '' })

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [dash, br] = await Promise.all([api.dashboard(), api.branches()])
    dashboard.value = dash
    branches.value = br
    if (br.length && !newManager.value.branch_id) newManager.value.branch_id = br[0].id
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function createBranch() {
  error.value = ''
  success.value = ''
  try {
    await api.createBranch(newBranch.value.address)
    newBranch.value.address = ''
    success.value = 'Филиал создан'
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function createManager() {
  error.value = ''
  success.value = ''
  try {
    await api.createManager({ ...newManager.value, branch_id: Number(newManager.value.branch_id) })
    newManager.value = { phone: '', password: '', name: '', branch_id: newManager.value.branch_id }
    success.value = 'Менеджер создан'
    await load()
  } catch (e) {
    error.value = e.message
  }
}
</script>

<template>
  <div class="mx-auto w-full max-w-6xl px-4 py-6">
    <h1 class="text-2xl font-bold text-slate-900 sm:text-3xl">Панель администратора</h1>
    <p class="mt-2 text-sm text-slate-500">Сводка по филиалам и управление персоналом</p>

    <div v-if="error" class="mt-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700" role="alert">
      {{ error }}
    </div>
    <div v-if="success" class="mt-4 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
      {{ success }}
    </div>

    <div v-if="loading" class="mt-8 text-center text-sm text-slate-400">Загрузка…</div>

    <template v-else-if="dashboard">
      <!-- Сводные показатели -->
      <section class="mt-6 grid gap-3 sm:grid-cols-3">
        <div class="card">
          <p class="text-xs uppercase tracking-wide text-slate-400">Записей сегодня</p>
          <p class="mt-1 text-3xl font-bold text-slate-900">{{ dashboard.total_today }}</p>
        </div>
        <div class="card">
          <p class="text-xs uppercase tracking-wide text-slate-400">Новых заявок</p>
          <p class="mt-1 text-3xl font-bold text-blue-600">{{ dashboard.total_new }}</p>
        </div>
        <div class="card">
          <p class="text-xs uppercase tracking-wide text-slate-400">Отмен</p>
          <p class="mt-1 text-3xl font-bold text-red-600">{{ dashboard.total_canceled }}</p>
        </div>
      </section>

      <!-- Филиалы -->
      <section class="mt-6">
        <h2 class="text-sm font-semibold uppercase tracking-wide text-slate-500">
          Филиалы на {{ formatDate(dashboard.date) }}
        </h2>
        <div class="mt-3 grid gap-3 md:grid-cols-2">
          <article v-for="b in dashboard.branches" :key="b.branch_id" class="card">
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="font-bold text-slate-900">{{ b.address }}</p>
                <p class="mt-1 text-sm text-slate-500">Записей сегодня: {{ b.today }}</p>
              </div>
              <span class="badge" :class="LOAD_STYLES[b.load_level].chip">
                {{ LOAD_STYLES[b.load_level].label }} · {{ b.load_pct }}%
              </span>
            </div>

            <dl class="mt-4 grid grid-cols-3 gap-2 text-center text-xs">
              <div class="rounded-lg bg-blue-50 px-2 py-2">
                <dt class="text-slate-500">Новые</dt>
                <dd class="mt-0.5 text-base font-bold text-blue-700">{{ b.new }}</dd>
              </div>
              <div class="rounded-lg bg-amber-50 px-2 py-2">
                <dt class="text-slate-500">В работе</dt>
                <dd class="mt-0.5 text-base font-bold text-amber-700">{{ b.in_progress }}</dd>
              </div>
              <div class="rounded-lg bg-red-50 px-2 py-2">
                <dt class="text-slate-500">Отмены</dt>
                <dd class="mt-0.5 text-base font-bold text-red-700">{{ b.canceled_today }}</dd>
              </div>
            </dl>
          </article>
        </div>
      </section>

      <!-- Управление -->
      <section class="mt-6 grid gap-4 md:grid-cols-2">
        <div class="card">
          <h2 class="font-bold text-slate-900">Новый филиал</h2>
          <form class="mt-4 space-y-3" @submit.prevent="createBranch">
            <div>
              <label class="label" for="branch-address">Адрес</label>
              <input id="branch-address" v-model.trim="newBranch.address" class="input" type="text"
                placeholder="г. Москва, ул. Примерная, д. 1" required />
            </div>
            <button class="btn-primary w-full" type="submit">Создать филиал</button>
          </form>
        </div>

        <div class="card">
          <h2 class="font-bold text-slate-900">Новый менеджер</h2>
          <form class="mt-4 space-y-3" @submit.prevent="createManager">
            <div>
              <label class="label" for="mgr-name">Имя</label>
              <input id="mgr-name" v-model.trim="newManager.name" class="input" type="text" required />
            </div>
            <div>
              <label class="label" for="mgr-phone">Телефон</label>
              <input id="mgr-phone" v-model.trim="newManager.phone" class="input" type="tel" placeholder="+7 999 123-45-67" required />
            </div>
            <div>
              <label class="label" for="mgr-password">Пароль</label>
              <input id="mgr-password" v-model="newManager.password" class="input" type="password" placeholder="Минимум 6 символов" required />
            </div>
            <div>
              <label class="label" for="mgr-branch">Филиал</label>
              <select id="mgr-branch" v-model="newManager.branch_id" class="input" required>
                <option v-for="b in branches" :key="b.id" :value="b.id">{{ b.address }}</option>
              </select>
            </div>
            <button class="btn-primary w-full" type="submit">Создать менеджера</button>
          </form>
        </div>
      </section>
    </template>
  </div>
</template>
