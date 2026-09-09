package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	invoicecontract "github.com/dujiao-next/internal/modules/invoice/contract"
	"github.com/dujiao-next/internal/modules/invoice/domain"
)

type InvoiceDocumentSource interface {
	ListReadyInvoices(context.Context) ([]invoicecontract.ReadyInvoice, error)
	DownloadInvoice(context.Context, string) ([]byte, error)
	UpdateDelivery(context.Context, string, string, string, *time.Time) error
}

type InvoiceMailer interface {
	SendInvoice(to, subject, body, fileName string, content []byte) error
}

type DeliveryStore interface {
	GetByRequestNo(string) (*domain.Request, error)
	ClaimEmail(string, string, *time.Time) (bool, *domain.Request, error)
	FinishEmail(string, bool, string, time.Time) error
}

func ProcessReadyInvoices(ctx context.Context, store DeliveryStore, source InvoiceDocumentSource, mailer InvoiceMailer) error {
	if store == nil || source == nil || mailer == nil {
		return nil
	}
	items, err := source.ListReadyInvoices(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		request, loadErr := store.GetByRequestNo(item.RequestNo)
		if loadErr != nil || request == nil {
			continue
		}
		claimed, request, claimErr := store.ClaimEmail(item.RequestNo, item.InvoiceNumber, item.InvoiceDate)
		if claimErr != nil || !claimed {
			continue
		}
		content, downloadErr := source.DownloadInvoice(ctx, item.FileToken)
		if downloadErr != nil {
			_ = store.FinishEmail(item.RequestNo, false, downloadErr.Error(), time.Now())
			_ = source.UpdateDelivery(ctx, item.RecordID, "邮件失败", "发票附件下载失败", nil)
			continue
		}
		fileName := strings.TrimSpace(item.FileName)
		if fileName == "" {
			fileName = "电子发票-" + item.InvoiceNumber + ".pdf"
		}
		subject := fmt.Sprintf("【老实人AI VIP】电子发票已开具｜%s元", request.InvoiceTotalAmount.String())
		body := fmt.Sprintf("%s，您好：\n\n您申请的电子发票已经开具，发票 PDF 请查看本邮件附件。\n\n发票号码：%s\n开票金额：%s 元\n原订单号：%s\n", request.BuyerTitle, item.InvoiceNumber, request.InvoiceTotalAmount.String(), request.OriginalOrderNo)
		if sendErr := mailer.SendInvoice(request.RecipientEmail, subject, body, fileName, content); sendErr != nil {
			_ = store.FinishEmail(item.RequestNo, false, sendErr.Error(), time.Now())
			_ = source.UpdateDelivery(ctx, item.RecordID, "邮件失败", "邮件发送失败", nil)
			continue
		}
		sentAt := time.Now()
		_ = store.FinishEmail(item.RequestNo, true, "", sentAt)
		_ = source.UpdateDelivery(ctx, item.RecordID, "已完成", "", &sentAt)
	}
	return nil
}
