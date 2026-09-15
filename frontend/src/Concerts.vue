<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { api } from './api'
import { seatLabel } from './pages'
import EventAdmin from './EventAdmin.vue'
const props = defineProps({ user: { type: Object, default: null } })
const events = ref([]),
  eventId = ref(''),
  bookings = ref([]),
  selected = ref([]),
  error = ref(''),
  message = ref(''),
  busy = ref(false),
  loading = ref(false)
const event = computed(() => events.value.find((e) => String(e.id) === String(eventId.value)))
const mine = computed(() => bookings.value.filter((b) => b.mine))
let requestVersion = 0
async function loadEvents() {
  try {
    events.value = await api('/events')
    if (!events.value.some((e) => String(e.id) === String(eventId.value)))
      eventId.value = events.value.length ? String(events.value[0].id) : ''
  } catch (e) {
    error.value = e.message
  }
}
async function loadSeats() {
  const version = ++requestVersion
  selected.value = []
  bookings.value = []
  if (!eventId.value) return
  loading.value = true
  try {
    const data = await api(`/events/${eventId.value}/bookings`)
    if (version === requestVersion) bookings.value = data
  } catch (e) {
    if (version === requestVersion) error.value = e.message
  } finally {
    if (version === requestVersion) loading.value = false
  }
}
watch([eventId, () => props.user], () => {
  error.value = ''
  message.value = ''
  loadSeats()
})
onMounted(loadEvents)
function occupied(seat) {
  return bookings.value.some((b) => b.seat === seat)
}
function choose(seat) {
  selected.value = selected.value.includes(seat)
    ? selected.value.filter((s) => s !== seat)
    : [...selected.value, seat]
}
async function reserve() {
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    await api(`/events/${eventId.value}/bookings`, 'POST', { seats: selected.value })
    message.value = 'Места забронированы. До встречи на концерте!'
    await loadSeats()
  } catch (e) {
    error.value = e.message
    await loadSeats()
  } finally {
    busy.value = false
  }
}
async function cancel(id) {
  busy.value = true
  error.value = ''
  try {
    await api(`/bookings/${id}`, 'DELETE')
    message.value = 'Бронь отменена'
    await loadSeats()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <section>
    <p class="eyebrow">ОТЧЁТНЫЕ КОНЦЕРТЫ</p>
    <h1>Место для музыки</h1>
    <p>Выберите концерт и места в актовом зале школы.</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="message" class="success" role="status">{{ message }}</p>
    <label
      >Мероприятие<select v-model="eventId" :disabled="busy">
        <option disabled value="">Выберите концерт</option>
        <option v-for="e in events" :key="e.id" :value="String(e.id)">
          {{ e.title }} · {{ new Date(e.starts_at).toLocaleString('ru-RU') }}
        </option>
      </select></label
    >
    <p v-if="!events.length">Пока нет запланированных концертов.</p>
    <template v-if="event"
      ><p v-if="!user" class="notice">Для бронирования войдите в личный кабинет.</p>
      <div class="hall">
        <div class="stage">СЦЕНА</div>
        <p v-if="loading" role="status">Обновляем места…</p>
        <div class="seats" :style="{ gridTemplateColumns: `repeat(${event.cols}, 1fr)` }">
          <button
            v-for="seat in event.rows * event.cols"
            :key="seat"
            class="seat"
            :class="{ occupied: occupied(seat), selected: selected.includes(seat) }"
            :aria-label="seatLabel(seat, event.cols)"
            :aria-pressed="selected.includes(seat)"
            :disabled="
              !user ||
              occupied(seat) ||
              busy ||
              loading ||
              (!selected.includes(seat) && selected.length >= 10)
            "
            @click="choose(seat)"
          >
            {{ ((seat - 1) % event.cols) + 1 }}
          </button>
        </div>
        <div class="legend">
          <span>□ Свободно</span><span>■ Выбрано</span><span class="muted">■ Занято</span>
        </div>
      </div>
      <div class="booking-bar">
        <span>Выбрано мест: {{ selected.length }} <small>(до 10 за раз)</small></span
        ><button :disabled="!selected.length || busy || loading" @click="reserve">
          Подтвердить бронь
        </button>
      </div>
      <section v-if="mine.length" class="tickets">
        <h2>Ваши места</h2>
        <div v-for="b in mine" :key="b.id">
          {{ seatLabel(b.seat, event.cols)
          }}<button class="plain" :disabled="busy" @click="cancel(b.id)">Отменить бронь</button>
        </div>
      </section></template
    ><EventAdmin v-if="user?.admin" :events="events" @changed="loadEvents" />
  </section>
</template>
