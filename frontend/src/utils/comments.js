export function normalizeCommentsResponse(response) {
  if (Array.isArray(response?.data)) return response.data
  if (Array.isArray(response)) return response
  return []
}
