import { afterEach, expect, it, vi } from 'vitest'
import { api } from './api'
import { seatLabel, pages } from './pages'
afterEach(() => vi.unstubAllGlobals())
it('reads data and sends JSON with cookies', async () => {
  const fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ id: 1 }) })
  vi.stubGlobal('fetch', fetch)
  expect(await api('/events')).toEqual({ id: 1 })
  await api('/login', 'POST', { login: 'alice' })
  expect(fetch).toHaveBeenLastCalledWith(
    '/api/login',
    expect.objectContaining({
      method: 'POST',
      credentials: 'same-origin',
      body: '{"login":"alice"}'
    })
  )
})
it('reports server and fallback errors', async () => {
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockResolvedValueOnce({ ok: false, json: async () => ({ error: 'Занято' }) })
      .mockResolvedValueOnce({ ok: false, json: async () => ({}) })
  )
  await expect(api('/events')).rejects.toThrow('Занято')
  await expect(api('/events')).rejects.toThrow('Ошибка запроса')
})
it('labels seats and supplies information pages', () => {
  expect(seatLabel(11, 10)).toBe('Ряд 2, место 1')
  expect(Object.keys(pages).length).toBeGreaterThan(5)
})
it('handles non-JSON gateway errors', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: false,
      json: async () => {
        throw new Error('HTML')
      }
    })
  )
  await expect(api('/login')).rejects.toThrow('Сервис временно недоступен')
})
