package consumer

import (
	"context"

	invoiceapp "github.com/dujiao-next/internal/modules/invoice/application"
	"github.com/hibiken/asynq"
)

func (c *Consumer) handleInvoiceDelivery(ctx context.Context, _ *asynq.Task) error {
	if err := c.InvoiceService.SyncPendingFeishu(ctx); err != nil {
		return err
	}
	return invoiceapp.ProcessReadyInvoices(ctx, c.InvoiceRepo, c.InvoiceDocumentSource, c.InvoiceMailer)
}
