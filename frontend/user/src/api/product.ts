import { api, userApi } from './client'

const authenticated = () => Boolean(localStorage.getItem('user_token'))

export const productAPI = {
    list: (params?: any) => authenticated() ? userApi.get('/products', { params }) : api.get('/public/products', { params }),
    detail: (slug: string) => authenticated() ? userApi.get(`/products/${slug}`) : api.get(`/public/products/${slug}`),
}

export const postAPI = {
    list: (params?: any) => api.get('/public/posts', { params }),
    detail: (slug: string) => api.get(`/public/posts/${slug}`),
}

export const bannerAPI = {
    list: (params?: any) => api.get('/public/banners', { params }),
}

export const categoryAPI = {
    list: (params?: any) => api.get('/public/categories', { params }),
}

export const memberLevelAPI = {
    list: () => api.get('/public/member-levels'),
}
