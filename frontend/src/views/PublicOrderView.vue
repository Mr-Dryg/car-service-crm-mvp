<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, ApiError, setSession } from '../api'
import { todayISO } from '../utils'

const router = useRouter()

const branches = ref([])
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const fieldErrors = ref({})

const form = ref({
  name: '',
  phone: '',
  password: '',
  branch_id: '',
  car_number: '',
  service_type: '',
  preferred_date: todayISO(),
  preferred_time: '10:00',
  comment: '',
})

const serviceTypes = [
  'Замена масла и фильтров',
  'Диагностика ходовой',
  'Замена тормозных колодок',
  'Компьютерная диагностика',
  'Замена ремня ГРМ',
  'Ремонт кондиционера',
  'Плановое ТО',
  'Другое',
]

// Часы работы автосервиса.
const timeSlots = Array.from({ length: 12 }, (_, i) => `${String(9 + i).padStart(2, '0')}:00`)

onMounted(async () => {
  try {
    branches.value = await api.branches()
    if (branches.value.length) form.value.branch_id = branches.value[0].id
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
})

async function submit() {
  error.value = ''
  fieldErrors.value = {}
  submitting.value = true
  try {
    const result = await api.createPublicOrder({
      ...form.value,
      branch_id: Number(form.value.branch_id),
    })
    // Клиент сразу авторизован и попадает в личный кабинет.
    setSession(result.token, result.user)
    router.push({ name: 'cabinet' })
  } catch (e) {
    if (e instanceof ApiError) {
      error.value = e.message
      fieldErrors.value = e.fields
    } else {
      error.value = 'Не удалось отправить заявку. Попробуйте позже.'
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="mx-auto w-full max-w-6xl px-4 py-6 sm:py-10">
    <div class="grid gap-6 lg:grid-cols-[1fr_380px] lg:items-start">
      <div class="card">
        <h1 class="text-2xl font-bold text-slate-900 sm:text-3xl">Запись на ремонт</h1>
        <p class="mt-2 text-sm text-slate-500">
          Заполните форму — личный кабинет создастся автоматически, подтверждать почту не нужно.
        </p>

        <div
          v-if="error"
          class="mt-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
          role="alert"
        >
          {{ error }}
        </div>

        <form class="mt-6 grid gap-4 sm:grid-cols-2" novalidate @submit.prevent="submit">
          <div>
            <label class="label" for="name">Ваше имя</label>
            <input id="name" v-model.trim="form.name" class="input" type="text" placeholder="Иван Иванов"
              autocomplete="name" required />
            <p v-if="fieldErrors.name" class="mt-1 text-xs text-red-600">{{ fieldErrors.name }}</p>
          </div>

          <div>
            <label class="label" for="phone">Телефон</label>
            <input id="phone" v-model.trim="form.phone" class="input" type="tel" placeholder="+7 999 123-45-67"
              autocomplete="tel" required />
            <p v-if="fieldErrors.phone" class="mt-1 text-xs text-red-600">{{ fieldErrors.phone }}</p>
          </div>

          <div class="sm:col-span-2">
            <label class="label" for="password">Пароль для личного кабинета</label>
            <input id="password" v-model="form.password" class="input" type="password" placeholder="Минимум 6 символов"
              autocomplete="new-password" required />
            <p v-if="fieldErrors.password" class="mt-1 text-xs text-red-600">{{ fieldErrors.password }}</p>
          </div>

          <div>
            <label class="label" for="branch">Филиал</label>
            <select id="branch" v-model="form.branch_id" class="input" required>
              <option v-if="loading" value="">Загрузка…</option>
              <option v-for="b in branches" :key="b.id" :value="b.id">{{ b.address }}</option>
            </select>
            <p v-if="fieldErrors.branch_id" class="mt-1 text-xs text-red-600">{{ fieldErrors.branch_id }}</p>
          </div>

          <div>
            <label class="label" for="car">Госномер</label>
            <input id="car" v-model.trim="form.car_number" class="input" type="text" placeholder="А123ВС777" required />
            <p v-if="fieldErrors.car_number" class="mt-1 text-xs text-red-600">{{ fieldErrors.car_number }}</p>
          </div>

          <div>
            <label class="label" for="service">Тип обслуживания</label>
            <select id="service" v-model="form.service_type" class="input" required>
              <option value="" disabled>Выберите услугу</option>
              <option v-for="s in serviceTypes" :key="s" :value="s">{{ s }}</option>
            </select>
            <p v-if="fieldErrors.service_type" class="mt-1 text-xs text-red-600">{{ fieldErrors.service_type }}</p>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="label" for="date">Дата</label>
              <input id="date" v-model="form.preferred_date" class="input" type="date" :min="todayISO()" required />
              <p v-if="fieldErrors.preferred_date" class="mt-1 text-xs text-red-600">{{ fieldErrors.preferred_date }}</p>
            </div>
            <div>
              <label class="label" for="time">Время</label>
              <select id="time" v-model="form.preferred_time" class="input" required>
                <option v-for="t in timeSlots" :key="t" :value="t">{{ t }}</option>
              </select>
              <p v-if="fieldErrors.preferred_time" class="mt-1 text-xs text-red-600">{{ fieldErrors.preferred_time }}</p>
            </div>
          </div>

          <div class="sm:col-span-2">
            <label class="label" for="comment">Комментарий</label>
            <textarea id="comment" v-model.trim="form.comment" class="input" rows="3"
              placeholder="Что беспокоит автомобиль?"></textarea>
          </div>

          <div class="sm:col-span-2">
            <button class="btn-primary w-full sm:w-auto" type="submit" :disabled="submitting || loading">
              {{ submitting ? 'Создаём заявку…' : 'Записаться на ремонт' }}
            </button>
          </div>
        </form>
      </div>

      <aside class="space-y-3">
        <div class="card bg-gradient-to-br from-brand-600 to-brand-700 text-white">
          <h2 class="text-lg font-bold">Без звонков и ожидания</h2>
          <p class="mt-2 text-sm text-brand-50">
            Статус ремонта виден в личном кабинете. Когда машина готова — придёт SMS со ссылкой.
          </p>
        </div>
        <div class="card">
          <h3 class="text-sm font-semibold text-slate-900">Как это работает</h3>
          <ol class="mt-3 space-y-3 text-sm text-slate-600">
            <li class="flex gap-3"><span class="font-bold text-brand-600">1.</span> Заполните форму — кабинет создастся сразу.</li>
            <li class="flex gap-3"><span class="font-bold text-brand-600">2.</span> Менеджер подтвердит запись и назовёт слот.</li>
            <li class="flex gap-3"><span class="font-bold text-brand-600">3.</span> Следите за статусом в кабинете.</li>
            <li class="flex gap-3"><span class="font-bold text-brand-600">4.</span> Нажмите «Подтверждаю, скоро буду» — звонка не будет.</li>
          </ol>
        </div>
      </aside>
    </div>
  </div>
</template>
