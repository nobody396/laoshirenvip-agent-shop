package application

import (
	"strings"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type CustomerWalletProfileDirectory interface {
	GetProfileByUserID(userID uint) (*resellerdomain.Profile, error)
}

type CustomerWalletUserDirectory interface {
	GetByID(userID uint) (*userdomain.User, error)
	List(filter usercontract.ListFilter) ([]userdomain.User, int64, error)
}

type CustomerWalletFunds interface {
	GetAccount(userID uint) (*walletdomain.Account, error)
	GetResellerBalancesByUserIDs(resellerID uint, userIDs []uint) (map[uint]money.Amount, error)
	TransferToResellerAccount(input walletcontract.ResellerTransferInput) (*walletcontract.ResellerTransferResult, error)
	ListResellerTransactions(filter walletcontract.ResellerTransactionListFilter) ([]walletdomain.ResellerTransaction, int64, error)
}

type CustomerWalletService struct {
	profiles CustomerWalletProfileDirectory
	users    CustomerWalletUserDirectory
	wallets  CustomerWalletFunds
}

func NewCustomerWalletService(profiles CustomerWalletProfileDirectory, users CustomerWalletUserDirectory, wallets CustomerWalletFunds) *CustomerWalletService {
	return &CustomerWalletService{profiles: profiles, users: users, wallets: wallets}
}

type CustomerWalletTopUpInput struct {
	OwnerUserID    uint
	CustomerUserID uint
	Amount         money.Amount
	Reference      string
	Remark         string
}

type CustomerWalletTopUpResult struct {
	Profile             *resellerdomain.Profile
	Customer            *userdomain.User
	OwnerWallet         *walletdomain.Account
	CustomerWallet      *walletdomain.ResellerAccount
	OwnerTransaction    *walletdomain.Transaction
	CustomerTransaction *walletdomain.ResellerTransaction
	AlreadyApplied      bool
}

type CustomerWalletListRow struct {
	User          userdomain.User `json:"user"`
	WalletBalance money.Amount    `json:"wallet_balance"`
}

func (s *CustomerWalletService) activeProfile(ownerUserID uint) (*resellerdomain.Profile, error) {
	if s == nil || s.profiles == nil || s.users == nil || s.wallets == nil || ownerUserID == 0 {
		return nil, resellercontract.ErrNotOpened
	}
	profile, err := s.profiles.GetProfileByUserID(ownerUserID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, resellercontract.ErrNotOpened
	}
	if profile.Status != resellerdomain.ProfileStatusActive {
		return nil, resellercontract.ErrProfileInactive
	}
	if profile.SettlementStatus != resellerdomain.SettlementStatusNormal {
		return nil, resellercontract.ErrSettlementUnavailable
	}
	return profile, nil
}

func customerBelongsTo(profileID uint, user *userdomain.User) bool {
	return user != nil && user.Status == constants.UserStatusActive && user.RegistrationResellerID != nil && *user.RegistrationResellerID == profileID
}

func (s *CustomerWalletService) TopUp(input CustomerWalletTopUpInput) (*CustomerWalletTopUpResult, error) {
	profile, err := s.activeProfile(input.OwnerUserID)
	if err != nil {
		return nil, err
	}
	customer, err := s.users.GetByID(input.CustomerUserID)
	if err != nil {
		return nil, err
	}
	if !customerBelongsTo(profile.ID, customer) {
		return nil, resellercontract.ErrCustomerNotFound
	}
	transfer, err := s.wallets.TransferToResellerAccount(walletcontract.ResellerTransferInput{
		ResellerID:     profile.ID,
		OwnerUserID:    input.OwnerUserID,
		CustomerUserID: input.CustomerUserID,
		Amount:         input.Amount,
		Currency:       "CNY",
		Reference:      strings.TrimSpace(input.Reference),
		Remark:         strings.TrimSpace(input.Remark),
	})
	if err != nil {
		return nil, err
	}
	return &CustomerWalletTopUpResult{
		Profile: profile, Customer: customer,
		OwnerWallet: transfer.OwnerAccount, CustomerWallet: transfer.ResellerAccount,
		OwnerTransaction: transfer.OwnerTransaction, CustomerTransaction: transfer.ResellerTransaction,
		AlreadyApplied: transfer.AlreadyApplied,
	}, nil
}

func (s *CustomerWalletService) ListCustomers(ownerUserID uint, page, pageSize int, keyword string) ([]CustomerWalletListRow, int64, error) {
	profile, err := s.activeProfile(ownerUserID)
	if err != nil {
		return nil, 0, err
	}
	users, total, err := s.users.List(usercontract.ListFilter{
		Page: page, PageSize: pageSize, RegistrationResellerID: profile.ID,
		Keyword: strings.TrimSpace(keyword), SortBy: "created_at", SortOrder: "desc",
	})
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	balances, err := s.wallets.GetResellerBalancesByUserIDs(profile.ID, ids)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]CustomerWalletListRow, 0, len(users))
	for _, user := range users {
		rows = append(rows, CustomerWalletListRow{User: user, WalletBalance: balances[user.ID]})
	}
	return rows, total, nil
}

func (s *CustomerWalletService) ListCustomerTransactions(ownerUserID, customerUserID uint, page, pageSize int) ([]walletdomain.ResellerTransaction, int64, error) {
	profile, err := s.activeProfile(ownerUserID)
	if err != nil {
		return nil, 0, err
	}
	customer, err := s.users.GetByID(customerUserID)
	if err != nil {
		return nil, 0, err
	}
	if !customerBelongsTo(profile.ID, customer) {
		return nil, 0, resellercontract.ErrCustomerNotFound
	}
	return s.wallets.ListResellerTransactions(walletcontract.ResellerTransactionListFilter{
		Page: page, PageSize: pageSize, ResellerID: profile.ID, UserID: customerUserID,
	})
}

// CustomerPriceService manages fixed, authenticated per-customer SKU prices.
// Ownership checks, reseller retail resolution, and floor validation all live
// behind this interface so HTTP callers cannot bypass commercial invariants.
type CustomerPriceService struct {
	store resellercontract.CustomerPriceSettingStore
	users CustomerWalletUserDirectory
}

func NewCustomerPriceService(store resellercontract.CustomerPriceSettingStore, users CustomerWalletUserDirectory) *CustomerPriceService {
	return &CustomerPriceService{store: store, users: users}
}

type CustomerPriceQuote struct {
	Setting      *resellerdomain.CustomerPriceSetting
	BasePrice    decimal.Decimal
	RetailPrice  decimal.Decimal
	SpecialPrice decimal.Decimal
	GrossProfit  decimal.Decimal
}

type CustomerPriceListResult struct {
	Customer *userdomain.User
	Settings []resellerdomain.CustomerPriceSetting
}

func (s *CustomerPriceService) requireCustomer(ownerUserID, customerUserID uint) (*resellerdomain.Profile, *userdomain.User, error) {
	if s == nil || s.store == nil || s.users == nil || ownerUserID == 0 || customerUserID == 0 {
		return nil, nil, resellercontract.ErrCustomerNotFound
	}
	profile, err := s.store.GetProfileByUserID(ownerUserID)
	if err != nil {
		return nil, nil, err
	}
	if profile == nil {
		return nil, nil, resellercontract.ErrNotOpened
	}
	if profile.Status != resellerdomain.ProfileStatusActive {
		return nil, nil, resellercontract.ErrProfileInactive
	}
	customer, err := s.users.GetByID(customerUserID)
	if err != nil || !customerBelongsTo(profile.ID, customer) {
		return nil, nil, resellercontract.ErrCustomerNotFound
	}
	return profile, customer, nil
}

func (s *CustomerPriceService) List(ownerUserID, customerUserID uint) (*CustomerPriceListResult, error) {
	profile, customer, err := s.requireCustomer(ownerUserID, customerUserID)
	if err != nil {
		return nil, err
	}
	settings, err := s.store.ListCustomerPriceSettings(profile.ID, customerUserID)
	if err != nil {
		return nil, err
	}
	return &CustomerPriceListResult{Customer: customer, Settings: settings}, nil
}

func (s *CustomerPriceService) Set(ownerUserID, customerUserID, productID, skuID uint, fixedPrice decimal.Decimal) (*CustomerPriceQuote, error) {
	profile, _, err := s.requireCustomer(ownerUserID, customerUserID)
	if err != nil {
		return nil, err
	}
	if productID == 0 || skuID == 0 || fixedPrice.LessThanOrEqual(decimal.Zero) || !fixedPrice.Equal(fixedPrice.Round(2)) {
		return nil, resellercontract.ErrCustomerPriceInvalid
	}
	row, err := s.store.GetProductWithSettings(profile.ID, productID)
	if err != nil {
		return nil, err
	}
	if row == nil || !row.Product.IsActive {
		return nil, resellercontract.ErrCustomerPriceInvalid
	}
	productSettings, skuSettings := resellercontract.BuildSettingIndexes(row.Settings)
	productSetting := productSettings[productID]
	if productSetting != nil && !productSetting.IsListed {
		return nil, resellercontract.ErrCustomerPriceInvalid
	}
	sku := findCustomerPriceSKU(row.Product.SKUs, skuID)
	if sku == nil || !sku.IsActive {
		return nil, resellercontract.ErrCustomerPriceInvalid
	}
	skuSetting := skuSettings[resellercontract.SettingKey{ProductID: productID, SKUID: skuID}]
	if skuSetting != nil && !skuSetting.IsListed {
		return nil, resellercontract.ErrCustomerPriceInvalid
	}
	base := sku.PriceAmount.Decimal.Round(2)
	retail, _, err := ResolveUnitAmount(profile, productSetting, skuSetting, base)
	if err != nil {
		return nil, err
	}
	if err := ValidateUnitAmount(profile, sku, base, retail); err != nil {
		return nil, err
	}
	special := fixedPrice.Round(2)
	if err := ValidateUnitAmount(profile, sku, base, special); err != nil {
		return nil, err
	}
	if special.GreaterThan(retail) {
		return nil, resellercontract.ErrCustomerPriceAboveRetail
	}
	setting := resellerdomain.CustomerPriceSetting{
		ResellerID: profile.ID, CustomerUserID: customerUserID, ProductID: productID, SKUID: skuID,
		FixedPriceAmount: money.FromDecimal(special), CreatedByUserID: ownerUserID,
	}
	var saved *resellerdomain.CustomerPriceSetting
	err = s.store.WithinCustomerPriceSettingTransaction(func(store resellercontract.CustomerPriceSettingStore) error {
		var saveErr error
		saved, saveErr = store.UpsertCustomerPriceSetting(setting)
		return saveErr
	})
	if err != nil {
		return nil, err
	}
	return &CustomerPriceQuote{
		Setting: saved, BasePrice: base, RetailPrice: retail, SpecialPrice: special,
		GrossProfit: special.Sub(base).Round(2),
	}, nil
}

func (s *CustomerPriceService) Delete(ownerUserID, customerUserID, productID, skuID uint) error {
	profile, _, err := s.requireCustomer(ownerUserID, customerUserID)
	if err != nil {
		return err
	}
	if productID == 0 || skuID == 0 {
		return resellercontract.ErrCustomerPriceInvalid
	}
	return s.store.DeleteCustomerPriceSetting(profile.ID, customerUserID, productID, skuID)
}

func findCustomerPriceSKU(skus []productdomain.ProductSKU, skuID uint) *productdomain.ProductSKU {
	for i := range skus {
		if skus[i].ID == skuID {
			return &skus[i]
		}
	}
	return nil
}
