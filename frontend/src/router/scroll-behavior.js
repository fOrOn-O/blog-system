export const NAV_SCROLL_OFFSET = 84

export function getScrollTarget(to, from = {}, savedPosition) {
  if (savedPosition) {
    return savedPosition
  }

  const isHomePagination =
    to.name === 'Home' &&
    from.name === 'Home' &&
    to.query?.page !== from.query?.page

  if (isHomePagination) {
    return false
  }

  if (to.hash) {
    return {
      el: to.hash,
      top: NAV_SCROLL_OFFSET,
      behavior: 'smooth'
    }
  }

  return { top: 0 }
}

export function scrollBehavior(to, _from, savedPosition) {
  const target = getScrollTarget(to, _from, savedPosition)

  if (!savedPosition) {
    return target
  }

  // 返回首页时等待异步文章列表恢复高度，再还原浏览位置。
  return new Promise((resolve) => {
    window.setTimeout(() => resolve(target), 250)
  })
}
