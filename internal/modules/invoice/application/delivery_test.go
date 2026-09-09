package application

import (
	"context"
	"testing"
	"time"

	invoicecontract "github.com/dujiao-next/internal/modules/invoice/contract"
	"github.com/dujiao-next/internal/modules/invoice/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type deliveryStoreStub struct{ request *domain.Request }

func (s *deliveryStoreStub) GetByRequestNo(string) (*domain.Request, error) { return s.request, nil }
func (s *deliveryStoreStub) ClaimEmail(_ string, number string, date *time.Time) (bool, *domain.Request, error) {
	s.request.Status, s.request.InvoiceNumber, s.request.InvoiceDate = domain.StatusPendingEmail, number, date
	return true, s.request, nil
}
func (s *deliveryStoreStub) FinishEmail(_ string, success bool, lastError string, at time.Time) error {
	if success {
		s.request.Status, s.request.EmailSentAt = domain.StatusCompleted, &at
	} else {
		s.request.Status = domain.StatusEmailFailed
	}
	s.request.EmailLastError = lastError
	return nil
}

type documentSourceStub struct{ status string }

func (s *documentSourceStub) ListReadyInvoices(context.Context) ([]invoicecontract.ReadyInvoice, error) {
	return []invoicecontract.ReadyInvoice{{RecordID: "rec-1", RequestNo: "INV-1", InvoiceNumber: "2694", FileToken: "file-1", FileName: "invoice.pdf"}}, nil
}
func (s *documentSourceStub) DownloadInvoice(context.Context, string) ([]byte, error) {
	return []byte("%PDF-test"), nil
}
func (s *documentSourceStub) UpdateDelivery(_ context.Context, _ string, status, _ string, _ *time.Time) error {
	s.status = status
	return nil
}

type mailerStub struct {
	calls   int
	source  string
	to      string
	subject string
}

func (m *mailerStub) SendInvoice(source, to, subject, _, _ string, content []byte) error {
	m.calls++
	m.source = source
	m.to = to
	m.subject = subject
	if string(content) != "%PDF-test" {
		panic("wrong PDF")
	}
	return nil
}

func TestProcessReadyInvoicesSendsPDFAndCompletesRow(t *testing.T) {
	store := &deliveryStoreStub{request: &domain.Request{RequestNo: "INV-1", Source: "dujiao", BuyerTitle: "示例公司", OriginalOrderNo: "DJ-1", RecipientEmail: "finance@example.com", InvoiceTotalAmount: money.FromDecimal(decimal.RequireFromString("648.90")), Status: domain.StatusPendingIssue}}
	source, mailer := &documentSourceStub{}, &mailerStub{}
	if err := ProcessReadyInvoices(context.Background(), store, source, mailer); err != nil {
		t.Fatal(err)
	}
	if mailer.calls != 1 || mailer.source != "dujiao" || mailer.to != "finance@example.com" || mailer.subject != "电子发票已开具｜648.90元" || store.request.Status != domain.StatusCompleted || source.status != "已完成" {
		t.Fatal("unexpected delivery result")
	}
}

func TestProcessReadyInvoicesKeepsVIPSubjectForGMShop(t *testing.T) {
	store := &deliveryStoreStub{request: &domain.Request{RequestNo: "INV-1", Source: "gmshop", BuyerTitle: "示例公司", OriginalOrderNo: "GM-1", RecipientEmail: "finance@example.com", InvoiceTotalAmount: money.FromDecimal(decimal.RequireFromString("648.90")), Status: domain.StatusPendingIssue}}
	source, mailer := &documentSourceStub{}, &mailerStub{}
	if err := ProcessReadyInvoices(context.Background(), store, source, mailer); err != nil {
		t.Fatal(err)
	}
	if mailer.source != "gmshop" || mailer.subject != "【老实人AI VIP】电子发票已开具｜648.90元" {
		t.Fatalf("unexpected GMShop mail: source=%q subject=%q", mailer.source, mailer.subject)
	}
}
