export async function api(path, method = 'GET', body) {
  const response = await fetch(`/api${path}`, {
    method,
    credentials: 'same-origin',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  const data = await response
    .json()
    .catch(() => ({ error: 'Сервис временно недоступен. Попробуйте позже.' }))
  if (!response.ok) throw new Error(data.error || 'Ошибка запроса')
  return data
}
