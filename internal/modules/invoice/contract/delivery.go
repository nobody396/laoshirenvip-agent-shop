package contract

import "time"

type ReadyInvoice struct {
	RecordID, RequestNo, InvoiceNumber, FileToken, FileName string
	InvoiceDate                                             *time.Time
}
