import type { Component } from 'vue'
import {
  BatteryCharging,
  ArrowLeftRight,
  Wallet,
  BanknoteArrowUp,
  Gift,
  LifeBuoy,
  ReceiptText,
  Ticket,
  Compass,
  CircleUserRound,
} from 'lucide-vue-next'

/**
 * 业务入口 icon 白名单。
 *
 * 后台只能下发 key，前端用这张表映射到具体的 lucide 图标组件；
 * 不允许任意 class / 远程 SVG，避免 XSS 与样式注入。
 * 未命中时回落到 Gift（中性图标）。
 */
export const HOME_ENTRY_ICON_MAP: Record<string, Component> = {
  recharge: BatteryCharging,
  c2c: ArrowLeftRight,
  wallet: Wallet,
  withdrawal: BanknoteArrowUp,
  invitation: Gift,
  support: LifeBuoy,
  orders: ReceiptText,
  gift: Gift,
  ticket: Ticket,
  discovery: Compass,
  account: CircleUserRound,
}

export const FALLBACK_HOME_ENTRY_ICON: Component = Gift

/** 取 icon 组件：白名单命中则返回，否则回落 */
export const resolveHomeEntryIcon = (key: string | undefined): Component => {
  if (key && Object.prototype.hasOwnProperty.call(HOME_ENTRY_ICON_MAP, key)) {
    return HOME_ENTRY_ICON_MAP[key] as Component
  }
  return FALLBACK_HOME_ENTRY_ICON
}

/**
 * internal 类型业务入口的路由映射（key → 站内路径）。
 * action_target 显式下发时以 action_target 为准；未下发时按 key 取这里的默认路由。
 */
export const HOME_ENTRY_ROUTE_MAP: Record<string, string> = {
  recharge: '/recharge',
  c2c: '/c2c',
  wallet: '/me/wallet',
  withdrawal: '/me/wallet/withdrawal',
  invitation: '/me/invitation',
  support: '/support',
  orders: '/me/orders',
  discovery: '/discovery',
}
