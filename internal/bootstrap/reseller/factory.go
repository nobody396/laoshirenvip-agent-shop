package resellerbootstrap

import (
	"github.com/dujiao-next/internal/app/container"
	adminhttp "github.com/dujiao-next/internal/modules/reseller/transport/http/admin"
	publictlshttp "github.com/dujiao-next/internal/modules/reseller/transport/http/publictls"
	userhttp "github.com/dujiao-next/internal/modules/reseller/transport/http/user"
)

type Handlers struct {
	PublicTLS           *publictlshttp.Handler
	User                *userhttp.UserHandler
	UserProductSetting  *userhttp.UserProductSettingHandler
	UserFinance         *userhttp.UserFinanceHandler
	UserOrder           *userhttp.UserOrderHandler
	AdminManagement     *adminhttp.AdminManagementHandler
	AdminProfileDetail  *adminhttp.AdminProfileDetailHandler
	AdminSiteConfig     *adminhttp.AdminSiteConfigHandler
	AdminProductSetting *adminhttp.AdminProductSettingHandler
	AdminOperations     *adminhttp.AdminOperationsHandler
	AdminFinance        *adminhttp.AdminFinanceHandler
}

func New(c *container.Container) Handlers {
	return Handlers{
		PublicTLS: publictlshttp.NewHandler(c.ResellerStore),
		User: userhttp.NewUserHandler(
			c.ResellerManagementService,
			c.ResellerSiteConfigService,
			c.UploadService,
		),
		UserProductSetting: userhttp.NewUserProductSettingHandler(c.ResellerProductSettingService),
		UserFinance: userhttp.NewUserFinanceHandler(
			c.ResellerAccountingQuery,
			c.ResellerAccountingWithdraw,
		),
		UserOrder: userhttp.NewUserOrderHandler(c.ResellerOrderService),
		AdminManagement: adminhttp.NewAdminManagementHandler(
			c.ResellerManagementService, c.ResellerStore, c.AuthzAuditService,
		),
		AdminProfileDetail: adminhttp.NewAdminProfileDetailHandler(
			c.ResellerStore, c.ResellerProductSettingService, c.ResellerAccountingQuery, c.ResellerOrderService,
		),
		AdminSiteConfig: adminhttp.NewAdminSiteConfigHandler(
			c.ResellerSiteConfigService, c.ResellerStore, c.AuthzAuditService,
		),
		AdminProductSetting: adminhttp.NewAdminProductSettingHandler(
			c.ResellerProductSettingService, c.AuthzAuditService,
		),
		AdminOperations: adminhttp.NewAdminOperationsHandler(c.ResellerOperationsService),
		AdminFinance: adminhttp.NewAdminFinanceHandler(
			c.ResellerAccountingQuery,
			c.ResellerAccountingWithdraw,
		),
	}
}
