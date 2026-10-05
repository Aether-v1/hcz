/**
 * @deprecated 已停用。
 *
 * Phase 9 安全要求：前端不执行任何来自 /public/config 的 JS（Admin 注入脚本风险）。
 * 此模块保留为空实现，仅为兼容历史 import；不再创建/插入任何 <script> 节点。
 * 装修内容一律以纯文本插值渲染，禁止 v-html 执行富脚本。
 */
export const clearCustomScripts = (): void => {
  /* no-op：历史上清理过的托管脚本已不再注入 */
}

export const applyCustomScripts = (_rawScripts: unknown): void => {
  // 故意空实现：不执行后台下发的任何脚本
}
