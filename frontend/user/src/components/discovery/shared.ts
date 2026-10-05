/** Discovery block 通用：把 action_type/action_target 解析成可用链接 */
export interface ResolvedAction {
  href: string
  external: boolean
}

const isHttp = (v: string) => /^https?:\/\//i.test(v)

export const resolveBlockAction = (actionType: unknown, target: unknown): ResolvedAction => {
  const raw = String(target || '').trim()
  const type = String(actionType || '').toLowerCase()
  if (!raw) return { href: '', external: false }
  if (type === 'external' || isHttp(raw)) {
    return { href: isHttp(raw) ? raw : '', external: true }
  }
  // internal：必须是站内相对路径
  if (raw.startsWith('/') && !raw.startsWith('//')) return { href: raw, external: false }
  return { href: '', external: false }
}
