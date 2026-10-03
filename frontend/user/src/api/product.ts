import { userApi } from './client'

// HCZ 全站登录收口：商品/内容/分类/会员等级读接口已由后端移入 JWT 保护组，
// 必须携带登录 token（userApi），未登录由后端返回 401 并触发前端跳登录。
export const productAPI = {
    list: (params?: any) => userApi.get('/public/products', { params }),
    detail: (slug: string) => userApi.get(`/public/products/${slug}`),
}

export const postAPI = {
    list: (params?: any) => userApi.get('/public/posts', { params }),
    detail: (slug: string) => userApi.get(`/public/posts/${slug}`),
}

export const bannerAPI = {
    list: (params?: any) => userApi.get('/public/banners', { params }),
}

export const categoryAPI = {
    list: (params?: any) => userApi.get('/public/categories', { params }),
}

export const memberLevelAPI = {
    list: () => userApi.get('/public/member-levels'),
}

