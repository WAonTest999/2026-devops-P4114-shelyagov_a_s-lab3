import { beforeEach, afterEach, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { api } from './api'
import App from './App.vue'
import AuthForm from './AuthForm.vue'
import Concerts from './Concerts.vue'
import EventAdmin from './EventAdmin.vue'
vi.mock('./api', () => ({ api: vi.fn() }))
const concert = { id: 1, title: 'Концерт', starts_at: '2099-10-01T12:00:00Z', rows: 2, cols: 3 }
const user = { id: 1, login: 'alice', admin: false }
let wrappers = []
function render(c, props = {}) {
  const w = mount(c, { props })
  wrappers.push(w)
  return w
}
async function button(w, text) {
  const b = w.findAll('button').find((b) => b.text().includes(text))
  expect(b, text).toBeTruthy()
  await b.trigger('click')
  await flushPromises()
}
beforeEach(() => {
  vi.resetAllMocks()
  api.mockImplementation(async (path) =>
    path === '/events' ? [concert] : path.endsWith('/bookings') ? [] : user
  )
})
afterEach(() => {
  wrappers.forEach((w) => w.unmount())
  wrappers = []
  vi.restoreAllMocks()
})
it('logs in and registers, clears password and shows failures', async () => {
  const w = render(AuthForm)
  await w.get('[name=login]').setValue('alice')
  await w.get('[name=password]').setValue('password123')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(w.emitted('signed-in')[0]).toEqual([user])
  expect(w.get('[name=password]').element.value).toBe('')
  await button(w, 'Зарегистрироваться')
  await w.get('[name=password]').setValue('password123')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(api).toHaveBeenCalledWith('/register', 'POST', { login: 'alice', password: 'password123' })
  api.mockRejectedValueOnce(new Error('Логин занят'))
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(w.get('[role=alert]').text()).toBe('Логин занят')
  await button(w, 'Уже есть')
  expect(w.text()).toContain('Добро пожаловать')
})
it('navigates, restores user, handles logout errors and sign-in', async () => {
  const w = render(App)
  await flushPromises()
  expect(w.text()).toContain('alice')
  await button(w, 'Новости')
  expect(w.find('h1').text()).toBe('Новости')
  api.mockRejectedValueOnce(new Error('Нет сети'))
  await button(w, 'Выйти')
  expect(w.text()).toContain('Нет сети')
  await button(w, 'Выйти')
  await button(w, 'Личный кабинет')
  w.findComponent(AuthForm).vm.$emit('signed-in', user)
  await flushPromises()
  expect(w.findComponent(Concerts).exists()).toBe(true)
})
it('supports guest navigation from home', async () => {
  api.mockRejectedValueOnce(new Error('401'))
  const w = render(App)
  await flushPromises()
  expect(w.text()).toContain('Личный кабинет')
  await button(w, 'Выбрать концерт')
  expect(w.text()).toContain('Для бронирования')
  await button(w, 'О школе')
  await button(w, 'Концерты и билеты')
  expect(w.findComponent(Concerts).exists()).toBe(true)
})
it('selects seats, confirms booking, cancels own ticket', async () => {
  const w = render(Concerts, { user })
  await flushPromises()
  expect(w.findAll('.seat')).toHaveLength(6)
  await w.findAll('.seat')[0].trigger('click')
  await w.findAll('.seat')[0].trigger('click')
  expect(w.text()).toContain('Выбрано мест: 0')
  await w.findAll('.seat')[1].trigger('click')
  api.mockResolvedValueOnce({}).mockResolvedValueOnce([
    { id: 7, seat: 2, mine: true },
    { id: 0, seat: 3, mine: false }
  ])
  await button(w, 'Подтвердить')
  expect(api).toHaveBeenCalledWith('/events/1/bookings', 'POST', { seats: [2] })
  expect(w.text()).toContain('Ваши места')
  expect(w.findAll('.seat')[2].attributes('disabled')).toBeDefined()
  await button(w, 'Отменить бронь')
  expect(api).toHaveBeenCalledWith('/bookings/7', 'DELETE')
  expect(w.text()).toContain('Бронь отменена')
})
it('refreshes occupied seats after conflict and reports load errors', async () => {
  const w = render(Concerts, { user })
  await flushPromises()
  await w.findAll('.seat')[0].trigger('click')
  api
    .mockRejectedValueOnce(new Error('Занято'))
    .mockResolvedValueOnce([{ id: 7, seat: 1, mine: true }])
  await button(w, 'Подтвердить')
  expect(w.text()).toContain('Занято')
  api.mockRejectedValueOnce(new Error('Ошибка отмены'))
  await button(w, 'Отменить бронь')
  expect(w.text()).toContain('Ошибка отмены')
  api.mockRejectedValueOnce(new Error('Ошибка мест'))
  await w.setProps({ user: null })
  await flushPromises()
  expect(w.text()).toContain('Ошибка мест')
})
it('handles empty concerts and failed list', async () => {
  api.mockResolvedValueOnce([])
  const w = render(Concerts)
  await flushPromises()
  expect(w.text()).toContain('Пока нет')
  api.mockRejectedValueOnce(new Error('Нет сети'))
  const failed = render(Concerts)
  await flushPromises()
  expect(failed.text()).toContain('Нет сети')
})
it('admin creates, edits and deletes concerts with errors', async () => {
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  const w = render(EventAdmin, { events: [concert] })
  const fill = async () => {
    const inputs = w.findAll('input')
    await inputs[0].setValue('Новый')
    await inputs[1].setValue('2099-10-01T12:00')
  }
  await fill()
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(api).toHaveBeenCalledWith(
    '/events',
    'POST',
    expect.objectContaining({ title: 'Новый', rows: 10 })
  )
  await button(w, 'Изменить')
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(api).toHaveBeenCalledWith('/events/1', 'PUT', expect.objectContaining({ rows: 2 }))
  await button(w, 'Изменить')
  await button(w, 'Отмена')
  await fill()
  api.mockRejectedValueOnce(new Error('Ошибка сохранения'))
  await w.get('form').trigger('submit')
  await flushPromises()
  expect(w.text()).toContain('Ошибка сохранения')
  await button(w, 'Изменить')
  await button(w, 'Удалить')
  expect(api).toHaveBeenCalledWith('/events/1', 'DELETE')
  api.mockRejectedValueOnce(new Error('Ошибка удаления'))
  await button(w, 'Удалить')
  expect(w.text()).toContain('Ошибка удаления')
  window.confirm.mockReturnValue(false)
  const calls = api.mock.calls.length
  await button(w, 'Удалить')
  expect(api.mock.calls.length).toBe(calls)
})
it('shows admin tools only to administrators', async () => {
  const w = render(Concerts, { user: { ...user, admin: true } })
  await flushPromises()
  expect(w.findComponent(EventAdmin).exists()).toBe(true)
  w.findComponent(EventAdmin).vm.$emit('changed')
  await flushPromises()
  expect(w.text()).toContain('Концерт')
})
