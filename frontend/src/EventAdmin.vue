<script setup>
import { ref } from 'vue'
import { api } from './api'
defineProps({ events: { type: Array, required: true } })
const emit = defineEmits(['changed'])
const editing = ref(null),
  title = ref(''),
  date = ref(''),
  rows = ref(10),
  cols = ref(10),
  error = ref(''),
  busy = ref(false)
function edit(e) {
  editing.value = e.id
  title.value = e.title
  const d = new Date(e.starts_at)
  date.value = new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
  rows.value = e.rows
  cols.value = e.cols
}
function reset() {
  editing.value = null
  title.value = ''
  date.value = ''
  rows.value = 10
  cols.value = 10
}
async function save() {
  busy.value = true
  error.value = ''
  try {
    await api(
      editing.value ? `/events/${editing.value}` : '/events',
      editing.value ? 'PUT' : 'POST',
      {
        title: title.value,
        starts_at: new Date(date.value).toISOString(),
        rows: Number(rows.value),
        cols: Number(cols.value)
      }
    )
    reset()
    emit('changed')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
async function remove(id) {
  if (!window.confirm('Удалить концерт и все его брони?')) return
  busy.value = true
  error.value = ''
  try {
    await api(`/events/${id}`, 'DELETE')
    if (editing.value === id) reset()
    emit('changed')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <section class="admin">
    <h2>Управление концертами</h2>
    <p>Размер зала задаётся при создании и затем не меняется.</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div v-for="e in events" :key="e.id" class="event-row">
      <span>{{ e.title }}</span
      ><button class="plain" :disabled="busy" @click="edit(e)">Изменить</button
      ><button class="plain" :disabled="busy" @click="remove(e.id)">Удалить</button>
    </div>
    <form @submit.prevent="save">
      <h3>{{ editing ? 'Изменить концерт' : 'Новый концерт' }}</h3>
      <label>Название<input v-model="title" required maxlength="100" /></label
      ><label>Дата и время<input v-model="date" type="datetime-local" required /></label>
      <div class="dimensions">
        <label
          >Рядов<input
            v-model="rows"
            type="number"
            min="1"
            max="20"
            :disabled="!!editing"
            required /></label
        ><label
          >Мест в ряду<input
            v-model="cols"
            type="number"
            min="1"
            max="20"
            :disabled="!!editing"
            required
        /></label>
      </div>
      <button :disabled="busy">Сохранить</button
      ><button v-if="editing" type="button" class="plain" @click="reset">Отмена</button>
    </form>
  </section>
</template>
