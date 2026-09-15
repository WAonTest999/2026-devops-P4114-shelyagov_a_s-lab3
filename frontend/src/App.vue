<script setup>
import { ref, onMounted } from 'vue'
import { api } from './api'
import { pages } from './pages'
import AuthForm from './AuthForm.vue'
import Concerts from './Concerts.vue'
const page = ref('О школе'),
  user = ref(null),
  error = ref('')
onMounted(async () => {
  try {
    user.value = await api('/me')
  } catch {
    user.value = null
  }
})
async function logout() {
  try {
    await api('/logout', 'POST')
    user.value = null
    error.value = ''
  } catch (e) {
    error.value = e.message
  }
}
function signedIn(value) {
  user.value = value
  page.value = 'Концерты'
}
</script>
<template>
  <header>
    <div class="brand">
      <span class="emblem" aria-hidden="true">𝄞</span>
      <div>
        <p class="eyebrow">ДЕТСКАЯ МУЗЫКАЛЬНАЯ ШКОЛА</p>
        <strong>имени Н. А. Римского-Корсакова</strong>
      </div>
    </div>
    <div class="account">
      <template v-if="user"
        ><span>{{ user.login }}</span
        ><button class="plain" @click="logout">Выйти</button></template
      ><button v-else class="plain" @click="page = 'Вход'">Личный кабинет →</button>
    </div>
  </header>
  <div class="layout">
    <nav aria-label="Главное меню">
      <p class="eyebrow">ШКОЛА</p>
      <button
        v-for="(_, name) in pages"
        :key="name"
        :class="{ active: page === name }"
        @click="page = name"
      >
        {{ name }}
      </button>
      <p class="eyebrow nav-gap">СОБЫТИЯ</p>
      <button :class="{ active: page === 'Концерты' }" @click="page = 'Концерты'">
        Концерты и билеты
      </button>
    </nav>
    <main>
      <p v-if="error" role="alert" class="error">{{ error }}</p>
      <AuthForm v-if="page === 'Вход'" @signed-in="signedIn" /><Concerts
        v-else-if="page === 'Концерты'"
        :user="user"
      />
      <section v-else class="intro">
        <p class="eyebrow">МУЗЫКА ОБЪЕДИНЯЕТ ПОКОЛЕНИЯ</p>
        <h1>{{ page }}</h1>
        <p class="lead">{{ pages[page] }}</p>
        <div v-if="page === 'О школе'" class="hero">
          <span aria-hidden="true">♫</span>
          <h2>Встречаемся<br />в концертном зале</h2>
          <p>Музыка наших учеников — для вас.</p>
          <button @click="page = 'Концерты'">Выбрать концерт →</button>
        </div>
      </section>
    </main>
  </div>
  <footer>
    ДМШ имени Н. А. Римского-Корсакова <span>Учебный проект · Бронирование без оплаты</span>
  </footer>
</template>
