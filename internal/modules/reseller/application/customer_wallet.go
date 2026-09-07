package application

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
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
