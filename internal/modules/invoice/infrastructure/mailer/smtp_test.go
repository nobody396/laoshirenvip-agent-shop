package mailer

import "testing"

type senderStub struct {
	vipCalls     int
	partnerCalls int
}

func (s *senderStub) SendInvoiceEmail(_, _, _, _ string, _ []byte) error {
	s.vipCalls++
	return nil
}

func (s *senderStub) SendCustomEmailWithBinaryAttachment(_, _, _, _, _ string, _ []byte) error {
	s.partnerCalls++
	return nil
}

func TestSMTPSeparatesVIPAndPartnerSenders(t *testing.T) {
	vip, partner := &senderStub{}, &senderStub{}
	mailer := newSMTP(vip, partner)
	if err := mailer.SendInvoice("gmshop", "buyer@example.com", "subject", "body", "invoice.pdf", []byte("%PDF")); err != nil {
		t.Fatal(err)
	}
	if err := mailer.SendInvoice("dujiao", "buyer@example.com", "subject", "body", "invoice.pdf", []byte("%PDF")); err != nil {
		t.Fatal(err)
	}
	if err := mailer.SendInvoice("dujiao_recharge", "buyer@example.com", "subject", "body", "invoice.pdf", []byte("%PDF")); err != nil {
		t.Fatal(err)
	}
	if vip.vipCalls != 1 || vip.partnerCalls != 0 || partner.vipCalls != 0 || partner.partnerCalls != 2 {
		t.Fatalf("unexpected routing: vip=%+v partner=%+v", vip, partner)
	}
}
