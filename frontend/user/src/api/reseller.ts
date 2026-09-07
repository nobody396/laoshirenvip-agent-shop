import { userApi } from './client'
import type {
    ResellerApplyPayload,
    ResellerCustomDomainPayload,
    ResellerCustomerWalletTopUpPayload,
    ResellerOrderListParams,
    ResellerOrderStatsParams,
    ResellerProductSettingUpdatePayload,
    ResellerSiteConfigPayload,
    ResellerWithdrawApplyPayload,
} from './types'

export const resellerAPI = {
    managementProfile: () => userApi.get('/reseller/profile'),
    apply: (data: ResellerApplyPayload) => userApi.post('/reseller/apply', data),
    updatePayoutAlipayAccount: (account: string) => userApi.put('/reseller/payout-alipay-account', { account }),
    domains: () => userApi.get('/reseller/domains'),
    submitDomain: (data: ResellerCustomDomainPayload) => userApi.post('/reseller/domains', data),
    siteConfig: () => userApi.get('/reseller/site-config'),
    updateSiteConfig: (data: ResellerSiteConfigPayload) => userApi.put('/reseller/site-config', data),
    uploadImage: (file: File) => {
        const formData = new FormData()
        formData.append('file', file)
        return userApi.post('/reseller/upload', formData)
    },
    productSettings: (params?: any) => userApi.get('/reseller/product-settings', { params }),
    productSettingDetail: (productId: number) => userApi.get(`/reseller/product-settings/${productId}`),
    updateProductSettings: (productId: number, data: ResellerProductSettingUpdatePayload) =>
        userApi.put(`/reseller/product-settings/${productId}`, data),
    previewProductSettings: (productId: number, data: ResellerProductSettingUpdatePayload) =>
        userApi.post(`/reseller/product-settings/${productId}/preview`, data),
    resetProductSetting: (productId: number, skuId = 0) =>
        userApi.delete(`/reseller/product-settings/${productId}`, { params: { sku_id: skuId } }),
    dashboard: () => userApi.get('/reseller/dashboard'),
    orders: (params?: ResellerOrderListParams) => userApi.get('/reseller/orders', { params }),
    orderStats: (params?: ResellerOrderStatsParams) => userApi.get('/reseller/orders/stats', { params }),
    orderDetail: (orderNo: string) => userApi.get(`/reseller/orders/${encodeURIComponent(orderNo)}`),
    balanceAccounts: (params?: any) => userApi.get('/reseller/balance-accounts', { params }),
    ledgerEntries: (params?: any) => userApi.get('/reseller/ledger-entries', { params }),
    withdraws: (params?: any) => userApi.get('/reseller/withdraws', { params }),
    applyWithdraw: (data: ResellerWithdrawApplyPayload) =>
        userApi.post('/reseller/withdraws', data),
    customers: (params?: { page?: number; page_size?: number; keyword?: string }) =>
        userApi.get('/reseller/customers', { params }),
    topUpCustomerWallet: (customerId: number, data: ResellerCustomerWalletTopUpPayload) =>
        userApi.post(`/reseller/customers/${customerId}/wallet/topup`, data),
    customerWalletTransactions: (customerId: number, params?: { page?: number; page_size?: number }) =>
        userApi.get(`/reseller/customers/${customerId}/wallet/transactions`, { params }),
}
