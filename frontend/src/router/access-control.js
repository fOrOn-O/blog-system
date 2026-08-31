export function resolveRouteAccess(to, { authenticated, user }) {
  if ((to.meta.requiresAuth || to.meta.requiresAdmin) && !authenticated) {
    return {
      name: 'Login',
      query: { redirect: to.fullPath }
    }
  }

  if (to.meta.requiresAdmin && user?.role !== 'admin') {
    return { name: 'Home' }
  }

  if (to.meta.guest && authenticated) {
    return { name: 'Home' }
  }

  return null
}
