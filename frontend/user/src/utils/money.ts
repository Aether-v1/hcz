const DECIMAL_PATTERN = /^[+-]?\d+(?:\.\d+)?$/

const parseDecimalToScaledInt = (value: unknown, scale: number): number | null => {
  if (scale < 0) return null
  if (value === null || value === undefined) return null
  const raw = String(value).trim()
  if (!raw) return null
  if (!DECIMAL_PATTERN.test(raw)) return null

  const negative = raw.startsWith('-')
  const normalized = raw.replace(/^[+-]/, '')
  const [intPartRaw, fracPartRaw = ''] = normalized.split('.')
  const intPart = intPartRaw || '0'
  const factor = 10 ** scale
  if (!Number.isSafeInteger(factor)) return null

  const mainFrac = fracPartRaw.padEnd(scale + 1, '0').slice(0, scale)
  const roundDigit = fracPartRaw.padEnd(scale + 1, '0').charAt(scale) || '0'

  const major = Number(intPart)
  const minor = mainFrac ? Number(mainFrac) : 0
  if (!Number.isSafeInteger(major) || !Number.isSafeInteger(minor)) return null

  let scaled = major * factor + minor
  if (!Number.isSafeInteger(scaled)) return null
  if (roundDigit >= '5') {
    scaled += 1
  }
  if (!Number.isSafeInteger(scaled)) return null

  return negative ? -scaled : scaled
}

export const amountToCents = (value: unknown): number | null => parseDecimalToScaledInt(value, 2)

export const rateToBasisPoints = (value: unknown): number | null => parseDecimalToScaledInt(value, 2)

export const centsToAmount = (cents: number): string => {
  if (!Number.isFinite(cents) || !Number.isSafeInteger(cents)) return '0.00'
  const negative = cents < 0
  const absolute = Math.abs(cents)
  const major = Math.floor(absolute / 100)
  const minor = String(absolute % 100).padStart(2, '0')
  return `${negative ? '-' : ''}${major}.${minor}`
}

export const basisPointsToPercent = (basisPoints: number): string => {
  if (!Number.isFinite(basisPoints) || !Number.isSafeInteger(basisPoints)) return '0.00'
  const negative = basisPoints < 0
  const absolute = Math.abs(basisPoints)
  const major = Math.floor(absolute / 100)
  const minor = String(absolute % 100).padStart(2, '0')
  return `${negative ? '-' : ''}${major}.${minor}`
}

export const calculateFeeCents = (baseCents: number, rateBasisPoints: number): number | null => {
  if (!Number.isSafeInteger(baseCents) || !Number.isSafeInteger(rateBasisPoints)) return null
  const multiplied = baseCents * rateBasisPoints
  if (!Number.isFinite(multiplied) || Math.abs(multiplied) > Number.MAX_SAFE_INTEGER) return null
  const fee = Math.round(multiplied / 10000)
  if (!Number.isSafeInteger(fee)) return null
  return fee
}

export const parseInteger = (value: unknown): number | null => {
  if (value === null || value === undefined) return null
  const num = Number(value)
  if (!Number.isFinite(num) || !Number.isInteger(num)) return null
  if (!Number.isSafeInteger(num)) return null
  return num
}

// ---- HCZ P0-2 USDT display formatters (display only, NO money calculation) ----
// 金额统一 2 位小数；null 安全；汇率 8 位。不做任何换算。

const trimTrailing = (n: string) => n

// 通用：把后端金额（字符串/数字，已是两位小数的十进制）格式化为 "123.45 CCY"
export const formatMoney = (value: unknown, currency?: string): string => {
  if (value === null || value === undefined || value === '') return '--'
  const raw = String(value).trim()
  if (!DECIMAL_PATTERN.test(raw)) return '--'
  const [i, f = ''] = raw.split('.')
  const fixed = `${i || '0'}.${(f + '00').slice(0, 2)}`
  return currency ? `${fixed} ${currency}` : fixed
}

// 站点币金额（商品原价）：formatSiteMoney(total_amount, order.currency)
export const formatSiteMoney = (value: unknown, currency?: string): string => formatMoney(value, currency)

// USDT 钱包金额格式化：保留 2 位小数，ROUND_HALF_UP（四舍五入，非截断）。
// 复用 parseDecimalToScaledInt（已实现 roundDigit >= '5' 时 +1），再用 centsToAmount 输出。
// null/undefined/空串/非法格式 → '--'（不是 '0.00'）；真实 0 → '0.00'。
// 业务值保持 string decimal，不做 parseFloat/Number 转换后再 toFixed。
export const formatUsdt = (value: unknown, currency?: string): string => {
  const cents = parseDecimalToScaledInt(value, 2)
  if (cents === null) return '--'
  return currency ? `${centsToAmount(cents)} ${currency}` : centsToAmount(cents)
}

// 钱包/退款/返利/流水：固定 USDT。优先用 API 返回的 currency，缺省 USDT。
// 内部走 formatUsdt（ROUND_HALF_UP 2 位小数）。
export const formatWalletMoney = (value: unknown, currency: string = 'USDT'): string => formatUsdt(value, currency || 'USDT')

// 汇率：1 USDT = R SiteCurrency
export const formatRate = (rate: unknown, siteCurrency?: string): string => {
  if (rate === null || rate === undefined || rate === '') return '--'
  const raw = String(rate).trim()
  if (!DECIMAL_PATTERN.test(raw)) return '--'
  const [i, f = ''] = raw.split('.')
  const fixed = `${i || '0'}.${(f + '00000000').slice(0, 8)}`
  return siteCurrency ? `1 USDT = ${fixed} ${siteCurrency}` : `1 USDT = ${fixed}`
}

void trimTrailing

