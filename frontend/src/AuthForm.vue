<script setup>
import { ref } from 'vue'
import { api } from './api'
const emit = defineEmits(['signed-in'])
const register = ref(false),
  login = ref(''),
  password = ref(''),
  error = ref(''),
  busy = ref(false)
function toggleMode() {
  register.value = !register.value
  error.value = ''
}
async function submit() {
  busy.value = true
  error.value = ''
  try {
    const body = { login: login.value, password: password.value }
    if (register.value) await api('/register', 'POST', body)
    const user = await api('/login', 'POST', body)
    password.value = ''
    emit('signed-in', user)
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <section class="auth">
    <p class="eyebrow">ЛИЧНЫЙ КАБИНЕТ</p>
    <h1>{{ register ? 'Регистрация' : 'Добро пожаловать' }}</h1>
    <p>Войдите, чтобы забронировать места на концерт.</p>
    <form @submit.prevent="submit">
      <label
        >Логин<input
          v-model="login"
          name="login"
          autocomplete="username"
          minlength="3"
          maxlength="40"
          required /></label
      ><label
        >Пароль<input
          v-model="password"
          name="password"
          type="password"
          :autocomplete="register ? 'new-password' : 'current-password'"
          minlength="8"
          maxlength="72"
          required
      /></label>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <button :disabled="busy">
        {{ busy ? 'Подождите…' : register ? 'Создать аккаунт' : 'Войти' }}
      </button>
    </form>
    <button class="plain" @click="toggleMode">
      {{ register ? 'Уже есть аккаунт? Войти' : 'Нет аккаунта? Зарегистрироваться' }}
    </button>
  </section>
</template>
