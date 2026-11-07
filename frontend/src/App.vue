<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { clearSession, userState } from './api'

const router = useRouter()
// userState — реактивный ref, поэтому профиль обновляется сразу после входа/выхода.
const user = computed(() => userState.value)

const navItems = computed(() => {
  if (!user.value) return []
  if (user.value.role === 'client') return [{ to: '/cabinet', label: 'Мои записи' }]
  if (user.value.role === 'manager') return [{ to: '/manager', label: 'CRM' }]
  if (user.value.role === 'superadmin') return [{ to: '/admin', label: 'Админка' }]
  return []
})

function logout() {
  clearSession()
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="flex min-h-screen flex-col">
    <header class="sticky top-0 z-30 border-b border-slate-200 bg-white/95 backdrop-blur">
      <div class="mx-auto flex w-full max-w-6xl items-center justify-between gap-3 px-4 py-3">
        <RouterLink to="/" class="flex items-center gap-2 font-bold text-slate-900">
          <span class="grid h-9 w-9 place-items-center rounded-xl bg-brand-600 text-white">🔧</span>
          <span class="hidden sm:inline">СТО Онлайн</span>
        </RouterLink>

        <nav class="flex shrink-0 items-center gap-2">
          <RouterLink
            v-for="item in navItems"
            :key="item.to"
            :to="item.to"
            class="rounded-lg px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 hover:text-slate-900"
          >
            {{ item.label }}
          </RouterLink>

          <template v-if="user">
            <span class="hidden text-sm text-slate-500 md:inline">{{ user.name }}</span>
            <button class="btn-secondary btn-sm shrink-0" type="button" @click="logout">Выйти</button>
          </template>
          <RouterLink v-else to="/login" class="btn-primary btn-sm shrink-0">Войти</RouterLink>
        </nav>
      </div>
    </header>

    <main class="flex-1">
      <RouterView />
    </main>

    <footer class="border-t border-slate-200 bg-white py-4">
      <p class="mx-auto max-w-6xl px-4 text-center text-xs text-slate-400">
        CRM сети автосервисов · MVP
      </p>
    </footer>
  </div>
</template>
