<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, setSession } from '../api'

const router = useRouter()
const route = useRoute()

const phone = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const result = await api.login(phone.value, password.value)
    setSession(result.token, result.user)

    // Возвращаем пользователя в раздел, соответствующий его роли.
    const redirect = route.query.redirect
    if (redirect) {
      router.push(String(redirect))
    } else if (result.user.role === 'manager') {
      router.push({ name: 'manager' })
    } else if (result.user.role === 'superadmin') {
      router.push({ name: 'admin' })
    } else {
      router.push({ name: 'cabinet' })
    }
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-md flex-col justify-center px-4 py-10">
    <div class="card">
      <h1 class="text-2xl font-bold text-slate-900">Вход</h1>
      <p class="mt-2 text-sm text-slate-500">Войдите по телефону и паролю</p>

      <div v-if="error" class="mt-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700" role="alert">
        {{ error }}
      </div>

      <form class="mt-6 space-y-4" novalidate @submit.prevent="submit">
        <div>
          <label class="label" for="login-phone">Телефон</label>
          <input id="login-phone" v-model.trim="phone" class="input" type="tel" placeholder="+7 999 123-45-67"
            autocomplete="tel" required />
        </div>
        <div>
          <label class="label" for="login-password">Пароль</label>
          <input id="login-password" v-model="password" class="input" type="password" placeholder="••••••"
            autocomplete="current-password" required />
        </div>
        <button class="btn-primary w-full" type="submit" :disabled="loading">
          {{ loading ? 'Входим…' : 'Войти' }}
        </button>
      </form>

      <div class="mt-6 rounded-xl bg-slate-50 p-3 text-xs text-slate-500">
        <p class="font-semibold text-slate-700">Демо-доступы</p>
        <p class="mt-1">Менеджер: +79000001000 / manager123</p>
        <p>Клиент: +79000002001 / client123</p>
        <p>Суперадмин: +79000000001 / admin12345</p>
      </div>
    </div>
  </div>
</template>
