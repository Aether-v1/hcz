import { getImageUrl } from './image'
import DOMPurify from 'dompurify'

/**
 * 将 HTML 内容中的图片路径转换为显示用的绝对路径
 * /uploads/xxx -> http://domain/uploads/xxx
 */
export function processHtmlForDisplay(html: string): string {
    if (!html) return ''

    // 匹配 src="/uploads/..."，支持单引号和双引号
    // 使用非贪婪匹配 .*?
    return html.replace(/src=["'](\/uploads\/.*?)["']/g, (_, path) => {
        return `src="${getImageUrl(path)}"`
    })
}

/**
 * 把后端下发的富文本（博客正文 / 商品详情 / 装修富文本）安全地交给 v-html。
 *
 * 这些内容是管理员/站长/分销商在后台富文本编辑器录入的，理论可信，但 v-html 会原样
 * 透传 <script>、<img onerror>、href="javascript:" 等可执行载荷（一旦后台账号被滥用，
 * 脚本会落到所有访客浏览器）。因此先重写图片路径，再过一道 DOMPurify 默认白名单：
 * 保留正常排版标签，剥离脚本/事件属性/可执行协议。与订单履约 instructions、公告弹窗、
 * 分销商站点配置预览的既有消毒模式保持一致。
 */
export function sanitizeRichHtml(html: string): string {
    if (!html) return ''
    return DOMPurify.sanitize(processHtmlForDisplay(html))
}

/**
 * 将 HTML 内容中的图片路径转换为存储用的相对路径
 * http://domain/uploads/xxx -> /uploads/xxx
 */
export function processHtmlForStorage(html: string): string {
    if (!html) return ''

    const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || ''
    let apiHost = ''
    try {
        if (apiBaseUrl) {
            // 提取域名 (例如 localhost:8080 或 domain.com)
            apiHost = new URL(apiBaseUrl).host
        } else {
            // 如果未配置 API_BASE_URL，则认为是同源，使用当前域名
            apiHost = window.location.host
        }
    } catch (e) {
        // Fallback
        apiHost = window.location.host
    }

    // 匹配 src="value" 或 src='value'
    return html.replace(/src=["'](.*?)["']/g, (match, src) => {
        try {
            // 如果是完整 URL (http:// 或 https://)
            if (src.startsWith('http://') || src.startsWith('https://')) {
                const url = new URL(src)

                // 检查域名是否匹配 API 域名，且路径以 /uploads/ 开头
                // url.host 会自动处理端口号 (例如 localhost:8080)
                if (url.host === apiHost && url.pathname.startsWith('/uploads/')) {
                    // 返回相对路径，例如 src="/uploads/xxx.png"
                    // 始终标准化为双引号
                    return `src="${url.pathname}"`
                }
            }
        } catch (e) {
            // URL 解析失败，忽略，返回原样
        }
        return match
    })
}

