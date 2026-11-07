/** Общие утилиты форматирования и справочники статусов. */

export const STATUS_LABELS = {
  new: 'Новая',
  confirmed: 'Подтверждена',
  in_progress: 'В работе',
  ready: 'Готова',
  completed: 'Завершена',
  canceled: 'Отменена',
}

/** CSS-классы бейджа для каждого статуса. */
export const STATUS_STYLES = {
  new: 'bg-blue-100 text-blue-700',
  confirmed: 'bg-indigo-100 text-indigo-700',
  in_progress: 'bg-amber-100 text-amber-800',
  ready: 'bg-emerald-100 text-emerald-700',
  completed: 'bg-slate-200 text-slate-700',
  canceled: 'bg-red-100 text-red-700',
}

/** Палитра цветовой шкалы загрузки филиала. */
export const LOAD_STYLES = {
  green: { bar: 'bg-emerald-500', chip: 'bg-emerald-100 text-emerald-700', label: 'Свободно' },
  yellow: { bar: 'bg-amber-500', chip: 'bg-amber-100 text-amber-800', label: 'Загрузка' },
  red: { bar: 'bg-red-500', chip: 'bg-red-100 text-red-700', label: 'Перегрузка' },
}

/** Форматирует дату YYYY-MM-DD в «21 окт. 2026». */
export function formatDate(value) {
  if (!value) return '—'
  const d = new Date(`${String(value).slice(0, 10)}T00:00:00`)
  if (Number.isNaN(d.getTime())) return String(value)
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' })
}

/** Возвращает сегодняшнюю дату в формате YYYY-MM-DD. */
export function todayISO() {
  const d = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** Форматирует номер телефона для отображения: +7 (999) 123-45-67. */
export function formatPhone(phone) {
  if (!phone) return '—'
  const digits = String(phone).replace(/\D/g, '')
  if (digits.length === 11 && (digits[0] === '7' || digits[0] === '8')) {
    return `+7 (${digits.slice(1, 4)}) ${digits.slice(4, 7)}-${digits.slice(7, 9)}-${digits.slice(9, 11)}`
  }
  return phone
}

/** Убирает секунды из времени вида "10:00:00" и приводит к "10:00". */
export function formatTime(value) {
  if (!value) return '—'
  return String(value).slice(0, 5)
}

/** Отдаёт ссылку для звонка клиенту. */
export function telHref(phone) {
  return `tel:${String(phone || '').replace(/[^\d+]/g, '')}`
}
