import { Crown, Gamepad2, Gift, HousePlug, Smartphone, Wifi, Zap } from 'lucide-vue-next'

export function getServiceIcon(category?: { slug?: string; name?: unknown } | null) {
  const label = `${category?.slug || ''} ${JSON.stringify(category?.name || '')}`.toLowerCase()
  if (/data|traffic|流量/.test(label)) return Wifi
  if (/mobile|phone|话费|話費/.test(label)) return Smartphone
  if (/member|subscription|权益|權益|会员|會員/.test(label)) return Crown
  if (/gift|礼品|禮品/.test(label)) return Gift
  if (/utilit|bill|生活缴费|生活繳費/.test(label)) return HousePlug
  if (/game|digital|游戏|遊戲|数字|數位/.test(label)) return Gamepad2
  return Zap
}
