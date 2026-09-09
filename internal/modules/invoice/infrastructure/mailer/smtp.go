package mailer

import notificationsmtp "github.com/dujiao-next/internal/modules/notification/infrastructure/smtp"

type emailSender interface {
	SendInvoiceEmail(to, subject, body, fileName string, content []byte) error
	SendCustomEmailWithBinaryAttachment(to, subject, body, fileName, contentType string, content []byte) error
}

type SMTP struct {
	vip     emailSender
	partner emailSender
}

func NewSMTP(vip *notificationsmtp.Service, partner *notificationsmtp.Service) *SMTP {
	return newSMTP(vip, partner)
}

func newSMTP(vip, partner emailSender) *SMTP {
	return &SMTP{vip: vip, partner: partner}
}

func (s *SMTP) SendInvoice(source, to, subject, body, fileName string, content []byte) error {
	if source == "gmshop" {
		return s.vip.SendInvoiceEmail(to, subject, body, fileName, content)
	}
	return s.partner.SendCustomEmailWithBinaryAttachment(to, subject, body, fileName, "application/pdf", content)
}
