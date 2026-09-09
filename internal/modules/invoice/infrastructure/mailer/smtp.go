package mailer

import notificationsmtp "github.com/dujiao-next/internal/modules/notification/infrastructure/smtp"

type SMTP struct{ sender *notificationsmtp.Service }

func NewSMTP(sender *notificationsmtp.Service) *SMTP { return &SMTP{sender: sender} }

func (s *SMTP) SendInvoice(to, subject, body, fileName string, content []byte) error {
	return s.sender.SendInvoiceEmail(to, subject, body, fileName, content)
}
